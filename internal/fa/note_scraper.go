package fa

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

type (
	NoteContent struct {
		id   uint
		text string
	}

	NoteEntry struct {
		id        uint
		title     string
		date      time.Time
		from      *FurAffinityUser
		link      *url.URL
		content   *NoteContent
		WasUnread bool
	}
)

const notesPath = "/msg/pms/"
const notesDateLayout = "January 2, 2006 03:04:05 PM"

func (ne *NoteEntry) EntryType() entries.EntryType { return entries.EntryTypeNote }
func (ne *NoteEntry) ID() uint                     { return ne.id }
func (ne *NoteEntry) Title() string                { return ne.title }
func (ne *NoteEntry) Date() time.Time              { return ne.date }
func (ne *NoteEntry) Link() *url.URL               { return ne.link }
func (ne *NoteEntry) From() *FurAffinityUser       { return ne.from }
func (ne *NoteEntry) Content() EntryContent        { return ne.content }
func (ne *NoteEntry) SetContent(ec EntryContent) {
	switch ec.(type) {
	case *NoteContent:
		ne.content = ec.(*NoteContent)
	default:
		panic("unknown content type")
	}
}
func (ne *NoteEntry) HasContent() bool { return ne.content != nil }

func (nc *NoteContent) ID() uint     { return nc.id }
func (nc *NoteContent) Text() string { return nc.text }

func (fc *FurAffinityCollector) notesDateLocation() *time.Location {
	return furaffinityDefaultLocation
}

func (fc *FurAffinityCollector) notesCookies() []*http.Cookie {
	folderCookie := http.Cookie{
		Value: "inbox",
		Name:  "folder",
	}

	if fc.OnlyUnreadNotes() {
		folderCookie.Value = "unread"
	}

	cookieMap := maps.Clone(fc.cookieMap())
	cookieMap["folder"] = &folderCookie

	return util.Values(cookieMap)
}

func (fc *FurAffinityCollector) noteCollector() *colly.Collector {
	c := fc.configuredCollector(false)
	c.SetCookies(faBaseUrl, fc.notesCookies())
	return c
}

func (fc *FurAffinityCollector) GetNotes(page uint) <-chan *NoteEntry {
	var guardChannel chan struct{}
	if fc.LimitConcurrency > 0 {
		guardChannel = make(chan struct{}, fc.LimitConcurrency)
	}
	noteChannel := make(chan *NoteEntry)

	c := fc.noteCollector()

	c.OnHTML("#notes-list", func(e *colly.HTMLElement) {
		e.ForEach(".note-list-container", func(i int, e *colly.HTMLElement) {
			if guardChannel != nil {
				guardChannel <- struct{}{}
				defer func() { <-guardChannel }()
			}
			parsed := fc.parseNoteSummary(e)
			if parsed == nil {
				return
			}

			if !fc.DateIsValid(entries.EntryTypeNote, parsed.Date()) {
				return
			}
			if !fc.IsWhitelisted(entries.EntryTypeNote, parsed.From().UserName) {
				return
			}

			noteChannel <- parsed
		})
	})

	c.OnError(func(response *colly.Response, err error) {
		logging.Errorf("Error while scraping note: %v", err)
	})

	go func() {
		defer close(noteChannel)
		err := c.Visit(fmt.Sprintf(faBaseUrl+notesPath+"%d/", page))
		if err != nil {
			logging.Errorf("Error while scraping notes: %v", err)
		}
		c.Wait()
	}()

	return noteChannel
}

func (fc *FurAffinityCollector) GetNewNotes() <-chan *NoteEntry {
	newNotes := make(chan *NoteEntry)

	allNotes := fc.GetNotes(1)

	go func() {
		defer close(newNotes)
		for note := range allNotes {
			if fc.isNoteNew(note.ID()) {
				newNotes <- note
			}
		}
	}()

	return newNotes
}

func (fc *FurAffinityCollector) GetNewNotesWithContent() <-chan *NoteEntry {
	channel := make(chan *NoteEntry)
	go func() {
		defer close(channel)
		concurrencyLimit := fc.LimitConcurrency
		if concurrencyLimit <= 0 {
			concurrencyLimit = 1
		}

		noteIds := make([]uint, 0)

		guardChannel := make(chan struct{}, concurrencyLimit)

		wg := sync.WaitGroup{}
		for note := range fc.GetNewNotes() {
			if note.WasUnread {
				noteIds = append(noteIds, note.ID())
			}
			guardChannel <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-guardChannel
					wg.Done()
				}()
				// Fetch note content without marking it as read, because we will do a batch operation alter
				note.content = fc.GetNoteContent(note.ID(), false)
				channel <- note
			}()
		}

		wg.Wait()
		err := fc.MarkUnread(noteIds...)
		if err != nil {
			logging.Errorf("Error while marking notes as unreads: %v", err)
		}
	}()

	return channel
}

func (fc *FurAffinityCollector) GetNoteContents(notes []uint, markUnread bool) map[uint]*NoteContent {
	contentMap := make(map[uint]*NoteContent, len(notes))
	for _, note := range notes {
		// Instruct to not mark as unread as this can be done via a batch request once
		content := fc.GetNoteContent(note, false)
		if content != nil {
			contentMap[note] = content
		}
	}

	if markUnread {
		fc.MarkUnread(util.Keys(contentMap)...)
	}

	return contentMap
}

func (fc *FurAffinityCollector) GetNoteContent(note uint, markUnread bool) *NoteContent {
	c := fc.noteCollector()

	channel := make(chan *NoteContent)

	c.OnHTML("#message .section-body", func(e *colly.HTMLElement) {
		// Remove FA scam warning
		dom := e.DOM
		dom.Find(".noteWarningMessage").Remove()
		dom.Find(".section-options").Remove()

		util.FixAutoLinks(e.DOM)

		textParts := strings.Split(trimHtmlText(dom.Text()), faNoteSeparator)
		text := ""
		if len(textParts) > 0 {
			text = trimHtmlText(textParts[0])
		}

		channel <- &NoteContent{
			id:   note,
			text: text,
		}

		if markUnread {
			fc.MarkUnread(note)
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

func (fc *FurAffinityCollector) MarkUnread(noteId ...uint) error {
	if len(noteId) == 0 {
		// No notes to mark as unread ;)
		return nil
	}
	client := fc.httpClient()
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

func (fc *FurAffinityCollector) isNoteNew(note uint) bool {
	return fc.isEntryNew(entries.EntryTypeNote, note)
}

func (fc *FurAffinityCollector) parseNoteSummary(noteElement *colly.HTMLElement) *NoteEntry {
	summary := NoteEntry{}
	parseError := false

	noteElement.ForEach(".note-list-subject-container", func(i int, e *colly.HTMLElement) {
		summary.title = trimHtmlText(e.Text)
	})

	noteElement.ForEach("a.notelink", func(i int, e *colly.HTMLElement) {
		dom := e.DOM
		if dom != nil {
			// If DOM is present, check whether the note is marked as unread by class assignment
			summary.WasUnread = e.DOM.HasClass("unread") || e.DOM.HasClass("note-unread")
		}

		link, _ := FurAffinityUrl().Parse(e.Attr("href"))
		summary.link = link
		pathParts := util.Filter(strings.Split(link.Path, "/"), func(s string) bool {
			return s != ""
		})
		id, err := strconv.ParseUint(pathParts[len(pathParts)-1], 10, 32)
		if err != nil {
			parseError = true
			return
		}
		summary.id = uint(id)
	})

	if !summary.WasUnread {
		// If the note is not marked as unread previously, double check by checking whether the unread icon is present
		noteElement.ForEach("img.unread", func(i int, imgElement *colly.HTMLElement) {
			// If this element is present, this note was unread previously, so we need to reset it later
			summary.WasUnread = true
		})
	}

	noteElement.ForEach(".note-list-senddate", func(i int, e *colly.HTMLElement) {
		// Try using the data-time attribute first
		timeFromAttr, err := util.EpochStringToTime(e.ChildAttr("span", "data-time"))
		if err == nil {
			summary.date = timeFromAttr
			return
		}

		dateString := trimHtmlText(e.Text)
		date, err := time.ParseInLocation(notesDateLayout, dateString, fc.notesDateLocation())
		if err != nil {
			logging.Errorf("Error parsing note send date (no time attribute was found). Expected layout '%s', got value '%s'", notesDateLayout, dateString)
			parseError = true
			return
		}
		summary.date = date
	})

	noteElement.ForEach(".note-list-sender", func(i int, e *colly.HTMLElement) {
		user := userFromNoteElement(e)
		summary.from = user
	})

	if parseError {
		return nil
	}
	return &summary
}
