package fa

import (
	"fmt"
	"github.com/gocolly/colly"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
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
	FurAffinityCollector struct {
		LimitConcurrency  int
		WithContent       bool
		UserID            uint
		faDefaultLocation *time.Location
	}
	FurAffinityUser struct {
		Name       string
		ProfileUrl *url.URL
	}

	NoteContent struct {
		ID   uint
		Text string
	}

	NoteSummary struct {
		ID      uint
		Title   string
		Date    time.Time
		From    FurAffinityUser
		Link    *url.URL
		Content *NoteContent
	}
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0"
const faBaseUrl = "https://www.furaffinity.net"
const faDateLayout = "Jan 2, 2006 03:04PM"
const faTimezone = "America/Los_Angeles"
const faNoteSeparator = "—————————"

var (
	furaffinityBaseUrl, _ = url.Parse(faBaseUrl)
)

func (nc *FurAffinityCollector) notesDateLocation() *time.Location {
	if nc.faDefaultLocation == nil {
		nc.faDefaultLocation, _ = time.LoadLocation(faTimezone)
	}

	return nc.faDefaultLocation
}

func (nc *FurAffinityCollector) configuredCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent(userAgent),
		colly.Async(true),
		colly.MaxDepth(2),
	)
	c.SetCookies(faBaseUrl, nc.notesCookies())
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: nc.LimitConcurrency})
	return c
}

func (nc *FurAffinityCollector) GetNotes(page uint) <-chan *NoteSummary {
	var guardChannel chan struct{}
	if nc.LimitConcurrency > 0 {
		guardChannel = make(chan struct{}, nc.LimitConcurrency)
	}
	noteChannel := make(chan *NoteSummary)

	c := nc.configuredCollector()

	c.OnHTML("#notes-list", func(e *colly.HTMLElement) {
		e.ForEach(".note-list-container", func(i int, e *colly.HTMLElement) {
			if guardChannel != nil {
				guardChannel <- struct{}{}
			}
			parsed := nc.parseNoteSummary(e)
			if parsed != nil {
				noteChannel <- parsed
			}

			if guardChannel != nil {
				<-guardChannel
			}
		})
	})

	c.OnError(func(response *colly.Response, err error) {
		fmt.Println(err)
	})

	go func() {
		defer close(noteChannel)
		c.Visit(fmt.Sprintf(faBaseUrl+"/msg/pms/%d/", page))
		c.Wait()
	}()

	return noteChannel
}

func (nc *FurAffinityCollector) GetNewNotes() <-chan *NoteSummary {
	newNotes := make(chan *NoteSummary)

	allNotes := nc.GetNotes(1)

	go func() {
		for note := range allNotes {
			if isNoteNew(note.ID) {
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

		guardChannel := make(chan struct{}, concurrencyLimit)

		wg := sync.WaitGroup{}
		for note := range nc.GetNewNotes() {
			guardChannel <- struct{}{}
			wg.Add(1)
			go func() {
				note.Content = nc.GetNoteContent(note.ID)
				channel <- note
				<-guardChannel
				wg.Done()
			}()
		}

		wg.Wait()
		close(channel)
	}()

	return channel
}

func (nc *FurAffinityCollector) GetNoteContents(notes []uint) map[uint]*NoteContent {
	contentMap := make(map[uint]*NoteContent, len(notes))
	for _, note := range notes {
		content := nc.GetNoteContent(note)
		if content != nil {
			contentMap[note] = content
		}
	}
	return contentMap
}

func (nc *FurAffinityCollector) GetNoteContent(note uint) *NoteContent {
	c := nc.configuredCollector()

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

func (nc *FurAffinityCollector) cookies() map[string]*http.Cookie {
	cookies := make([]database.UserCookie, 0)
	database.Db().Where(&database.UserCookie{UserID: nc.UserID}).Find(&cookies)

	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = &http.Cookie{Value: cookie.Value, Name: cookie.Name}
	}
	return cookieMap
}

func (nc *FurAffinityCollector) notesCookies() []*http.Cookie {
	cookieMap := maps.Clone(nc.cookies())
	cookieMap["folder"] = &http.Cookie{
		Value: "inbox",
		Name:  "folder",
	}

	values := make([]*http.Cookie, 0, len(cookieMap))
	for _, val := range cookieMap {
		values = append(values, val)
	}
	return values
}

func NewCollector(userId uint) *FurAffinityCollector {
	return &FurAffinityCollector{
		LimitConcurrency: 4,
		WithContent:      true,
		UserID:           userId,
	}
}

func FurAffinityUrl() *url.URL {
	return furaffinityBaseUrl
}

func noteIdToLink(note uint) (*url.URL, error) {
	return FurAffinityUrl().Parse(fmt.Sprintf("/msg/pms/1/%d/", note))
}

func isNoteNew(note uint) bool {
	foundRows := make([]database.KnownNote, 0)

	database.Db().Find(&foundRows, note)

	return len(foundRows) == 0
}

func (nc *FurAffinityCollector) parseNoteSummary(noteElement *colly.HTMLElement) *NoteSummary {
	summary := NoteSummary{}
	parseError := false

	noteElement.ForEach(".note-list-subject", func(i int, e *colly.HTMLElement) {
		summary.Title = trimHtmlText(e.Text)
	})

	noteElement.ForEach("a.notelink", func(i int, e *colly.HTMLElement) {
		link, _ := furaffinityBaseUrl.Parse(e.Attr("href"))
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

func trimHtmlText(s string) string {
	return util.TrimHtmlText(s)
}
