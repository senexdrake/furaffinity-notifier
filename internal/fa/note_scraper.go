package fa

import (
	"fmt"
	"github.com/gocolly/colly"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	NoteContent struct {
		ID   uint
		Text string
	}

	NoteSummary struct {
		ID        uint
		Title     string
		Date      time.Time
		From      FurAffinityUser
		Link      *url.URL
		Content   *NoteContent
		WasUnread bool
	}
)

const notesPath = "/msg/pms/"

func (nc *FurAffinityCollector) notesDateLocation() *time.Location {
	return furaffinityDefaultLocation
}

func (nc *FurAffinityCollector) notesCookies() []*http.Cookie {
	folderCookie := http.Cookie{
		Value: "inbox",
		Name:  "folder",
	}

	if nc.OnlyUnreadNotes {
		folderCookie.Value = "unread"
	}

	cookieMap := maps.Clone(nc.cookieMap())
	cookieMap["folder"] = &folderCookie

	return util.Values(cookieMap)
}

func (nc *FurAffinityCollector) noteCollector() *colly.Collector {
	c := nc.configuredCollector(false)
	c.SetCookies(faBaseUrl, nc.notesCookies())
	return c
}

func (nc *FurAffinityCollector) GetNotes(page uint) <-chan *NoteSummary {
	var guardChannel chan struct{}
	if nc.LimitConcurrency > 0 {
		guardChannel = make(chan struct{}, nc.LimitConcurrency)
	}
	noteChannel := make(chan *NoteSummary)
	userRegistrationDate := nc.registrationDate()

	c := nc.noteCollector()

	c.OnHTML("#notes-list", func(e *colly.HTMLElement) {
		e.ForEach(".note-list-container", func(i int, e *colly.HTMLElement) {
			if guardChannel != nil {
				guardChannel <- struct{}{}
				defer func() { <-guardChannel }()
			}
			parsed := nc.parseNoteSummary(e)
			if parsed == nil {
				return
			}

			// Return when note has been sent before this user registered and the option
			// to only notify about newer notes has been set
			if nc.OnlySinceRegistration && parsed.Date.Before(userRegistrationDate) {
				return
			}

			noteChannel <- parsed
		})
	})

	c.OnError(func(response *colly.Response, err error) {
		fmt.Println(err)
	})

	go func() {
		defer close(noteChannel)
		c.Visit(fmt.Sprintf(faBaseUrl+notesPath+"%d/", page))
		c.Wait()
	}()

	return noteChannel
}

func (nc *FurAffinityCollector) GetNewNotes() <-chan *NoteSummary {
	newNotes := make(chan *NoteSummary)

	allNotes := nc.GetNotes(1)

	go func() {
		for note := range allNotes {
			if nc.isNoteNew(note.ID) {
				newNotes <- note
			}
		}
		close(newNotes)
	}()

	return newNotes
}

func (nc *FurAffinityCollector) GetNewNotesWithContent() <-chan *NoteSummary {
	channel := make(chan *NoteSummary)
	go func() {
		concurrencyLimit := nc.LimitConcurrency
		if concurrencyLimit <= 0 {
			concurrencyLimit = 1
		}

		noteIds := make([]uint, 0)

		guardChannel := make(chan struct{}, concurrencyLimit)

		wg := sync.WaitGroup{}
		for note := range nc.GetNewNotes() {
			if note.WasUnread {
				noteIds = append(noteIds, note.ID)
			}
			guardChannel <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-guardChannel
					wg.Done()
				}()
				// Fetch note content without marking it as read, because we will do a batch operation alter
				note.Content = nc.GetNoteContent(note.ID, false)
				channel <- note
			}()
		}

		wg.Wait()
		nc.MarkUnread(noteIds...)
		close(channel)
	}()

	return channel
}

func (nc *FurAffinityCollector) GetNoteContents(notes []uint, markUnread bool) map[uint]*NoteContent {
	contentMap := make(map[uint]*NoteContent, len(notes))
	for _, note := range notes {
		// Instruct to not mark as unread as this can be done via a batch request once
		content := nc.GetNoteContent(note, false)
		if content != nil {
			contentMap[note] = content
		}
	}

	if markUnread {
		nc.MarkUnread(util.Keys(contentMap)...)
	}

	return contentMap
}

func (nc *FurAffinityCollector) GetNoteContent(note uint, markUnread bool) *NoteContent {
	c := nc.noteCollector()

	channel := make(chan *NoteContent)

	c.OnHTML("#message .section-body", func(e *colly.HTMLElement) {
		// Remove FA scam warning
		dom := e.DOM
		dom.Find(".noteWarningMessage").Remove()
		dom.Find(".section-options").Remove()

		textParts := strings.Split(trimHtmlText(dom.Text()), faNoteSeparator)
		text := ""
		if len(textParts) > 0 {
			text = trimHtmlText(textParts[0])
		}

		channel <- &NoteContent{
			ID:   note,
			Text: text,
		}

		if markUnread {
			nc.MarkUnread(note)
		}
	})

	link, err := noteIdToLink(note)
	if err != nil {
		close(channel)
		return nil
	}

	go func() {
		defer close(channel)
		c.Visit(link.String())
		c.Wait()
	}()
	return <-channel
}

func (nc *FurAffinityCollector) MarkUnread(noteId ...uint) error {
	if len(noteId) == 0 {
		// No notes to mark as unread ;)
		return nil
	}
	client := nc.httpClient()
	formValues := url.Values{}
	formValues.Set("manage_notes", "1")
	formValues.Set("move_to", "unread")

	for _, id := range noteId {
		formValues.Add("items[]", strconv.Itoa(int(id)))
	}

	postUrl, _ := FurAffinityUrl().Parse(notesPath)
	_, err := client.PostForm(postUrl.String(), formValues)
	return err
}

func noteIdToLink(note uint) (*url.URL, error) {
	return FurAffinityUrl().Parse(fmt.Sprintf(notesPath+"1/%d/#message", note))
}

func (nc *FurAffinityCollector) isNoteNew(note uint) bool {
	searchNote := database.KnownEntry{
		EntryType: entries.EntryTypeNote,
		ID:        note,
		UserID:    nc.UserID,
	}
	foundRows := make([]database.KnownEntry, 0)

	database.Db().Where(&searchNote).Find(&foundRows)

	return len(foundRows) == 0
}

func (nc *FurAffinityCollector) parseNoteSummary(noteElement *colly.HTMLElement) *NoteSummary {
	summary := NoteSummary{}
	parseError := false

	noteElement.ForEach(".note-list-subject", func(i int, e *colly.HTMLElement) {
		e.ForEach("img.unread", func(i int, imgElement *colly.HTMLElement) {
			// If this element is present, this note was unread previously, so we need to reset it later
			summary.WasUnread = true
		})
		summary.Title = trimHtmlText(e.Text)
	})

	noteElement.ForEach("a.notelink", func(i int, e *colly.HTMLElement) {
		link, _ := FurAffinityUrl().Parse(e.Attr("href"))
		summary.Link = link
		pathParts := util.Filter(strings.Split(link.Path, "/"), func(s string) bool {
			return s != ""
		})
		id, err := strconv.ParseUint(pathParts[len(pathParts)-1], 10, 32)
		if err != nil {
			parseError = true
			return
		}
		summary.ID = uint(id)
	})

	noteElement.ForEach(".note-list-senddate", func(i int, e *colly.HTMLElement) {
		dateString := trimHtmlText(e.Text)
		date, err := time.ParseInLocation(faDateLayout, dateString, nc.notesDateLocation())
		if err != nil {
			parseError = true
			return
		}
		summary.Date = date
	})

	noteElement.ForEach(".note-list-sender a", func(i int, e *colly.HTMLElement) {
		user := FurAffinityUser{
			Name: trimHtmlText(e.Text),
		}

		profileUrl, err := FurAffinityUrl().Parse(e.Attr("href"))
		if err != nil {
			return
		}

		user.ProfileUrl = profileUrl

		summary.From = user
	})

	if parseError {
		return nil
	}
	return &summary
}