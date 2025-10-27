package fa

import (
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

type (
	CommentEntry struct {
		entryType entries.EntryType
		id        uint
		date      time.Time
		link      *url.URL
		from      FurAffinityUser
		title     string
		content   *CommentContent
	}

	CommentContent struct {
		id   uint
		text string
	}
)

const commentsPath = "/msg/others/"
const entryDateLayout = "January 2, 2006 03:04:05 PM"

func (ce *CommentEntry) EntryType() entries.EntryType { return ce.entryType }
func (ce *CommentEntry) Date() time.Time              { return ce.date }
func (ce *CommentEntry) Link() *url.URL               { return ce.link }
func (ce *CommentEntry) ID() uint                     { return ce.id }
func (ce *CommentEntry) Title() string                { return ce.title }
func (ce *CommentEntry) From() FurAffinityUser        { return ce.from }
func (ce *CommentEntry) Content() EntryContent        { return ce.content }
func (ce *CommentEntry) SetContent(ec EntryContent) {
	switch ec.(type) {
	case *CommentContent:
		ce.content = ec.(*CommentContent)
		return
	default:
		panic("unknown content type")
	}
}
func (ce *CommentEntry) HasContent() bool { return ce.content != nil }

func (cc *CommentContent) ID() uint     { return cc.id }
func (cc *CommentContent) Text() string { return cc.text }

func (fc *FurAffinityCollector) otherCollector() *colly.Collector {
	c := fc.configuredCollector(true)
	return c
}

func (fc *FurAffinityCollector) entryHandlerWrapper(
	channel chan<- Entry,
	baseElement *colly.HTMLElement,
	perEntryFunc func(chan<- Entry, *sync.WaitGroup, *colly.HTMLElement) Entry,
) {
	wg := sync.WaitGroup{}
	userRegistrationDate := fc.registrationDate()
	// Add one to the WaitGroup to make sure it can't pass the Wait() call before these functions are being evaluated
	wg.Add(1)
	baseElement.ForEach("li", func(i int, el *colly.HTMLElement) {
		wg.Add(1)
		defer wg.Done()
		entry := perEntryFunc(channel, &wg, el)
		if entry == nil || entry.ID() == 0 {
			return
		}
		if fc.OnlySinceRegistration && entry.Date().Before(userRegistrationDate) {
			return
		}
		channel <- entry
	})
	wg.Done()
	wg.Wait()
}

func (fc *FurAffinityCollector) GetOtherEntries(entryTypes ...entries.EntryType) <-chan Entry {
	c := fc.otherCollector()

	channel := make(chan Entry)

	c.OnHTML("#messages-comments-submission", func(e *colly.HTMLElement) {
		entryType := entries.EntryTypeSubmissionComment
		if !slices.Contains(entryTypes, entryType) {
			return
		}
		handlerFunc := func(channel chan<- Entry, wg *sync.WaitGroup, element *colly.HTMLElement) Entry {
			parsed, err := fc.parseComment(entryType, element)
			if err != nil {
				return nil
			}
			return parsed
		}

		fc.entryHandlerWrapper(
			channel,
			e,
			handlerFunc,
		)
	})

	c.OnHTML("#messages-comments-journal", func(e *colly.HTMLElement) {
		entryType := entries.EntryTypeJournalComment
		if !slices.Contains(entryTypes, entryType) {
			return
		}
		handlerFunc := func(channel chan<- Entry, wg *sync.WaitGroup, element *colly.HTMLElement) Entry {
			parsed, err := fc.parseComment(entryType, element)
			if err != nil {
				return nil
			}
			return parsed
		}

		fc.entryHandlerWrapper(
			channel,
			e,
			handlerFunc,
		)
	})

	link, _ := FurAffinityUrl().Parse(commentsPath)

	go func() {
		defer close(channel)
		c.Visit(link.String())
		c.Wait()
	}()

	return channel
}

func (fc *FurAffinityCollector) GetNewOtherEntries(entryTypes ...entries.EntryType) <-chan Entry {
	newEntries := make(chan Entry)

	allEntries := fc.GetOtherEntries(entryTypes...)

	go func() {
		for entry := range allEntries {
			if fc.isEntryNew(entry.EntryType(), entry.ID()) {
				newEntries <- entry
			}
		}
		close(newEntries)
	}()

	return newEntries
}

func (fc *FurAffinityCollector) GetNewOtherEntriesWithContent(entryTypes ...entries.EntryType) <-chan Entry {
	channel := make(chan Entry)
	go func() {
		concurrencyLimit := fc.LimitConcurrency
		if concurrencyLimit <= 0 {
			concurrencyLimit = 1
		}

		guardChannel := make(chan struct{}, concurrencyLimit)

		wg := sync.WaitGroup{}
		for entry := range fc.GetNewOtherEntries(entryTypes...) {
			guardChannel <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-guardChannel
					wg.Done()
				}()

				// Fetch note content without marking it as read, because we will do a batch operation alter
				entry.SetContent(fc.GetOtherEntryContent(entry))
				channel <- entry
			}()
		}

		wg.Wait()
		close(channel)
	}()

	return channel
}

func (fc *FurAffinityCollector) GetOtherEntryContent(entry Entry) EntryContent {
	switch entry.(type) {
	case *CommentEntry:
		return fc.getCommentContent(entry.(*CommentEntry))
	}
	return nil
}

func (fc *FurAffinityCollector) getCommentContent(entry *CommentEntry) *CommentContent {
	c := fc.otherCollector()

	content := CommentContent{id: entry.ID()}

	valid := false

	// Escape the ID
	escapedId := strings.ReplaceAll(entry.Link().Fragment, ":", "\\:")
	commentIdTag := "#" + escapedId

	c.OnHTML(commentIdTag, func(e *colly.HTMLElement) {
		parent := e.DOM.Parent()
		commentTextElement := parent.Find(".comment-content .comment_text").First()
		util.FixAutoLinks(commentTextElement)
		content.text = trimHtmlText(commentTextElement.Text())
		valid = len(content.text) > 0
	})

	c.Visit(entry.Link().String())
	c.Wait()

	if !valid {
		return nil
	}

	return &content
}

func (fc *FurAffinityCollector) parseComment(entryType entries.EntryType, entryElement *colly.HTMLElement) (*CommentEntry, error) {
	comment := CommentEntry{
		entryType: entryType,
	}

	parseError := false

	entryElement.ForEachWithBreak("a", func(i int, e *colly.HTMLElement) bool {
		switch i {
		case 0:
			// First link is the user
			link, err := FurAffinityUrl().Parse(e.Attr("href"))
			if err != nil {
				parseError = true
				return true
			}
			comment.from = FurAffinityUser{
				ProfileUrl:  link,
				DisplayName: trimHtmlText(e.Text),
			}
			break
		case 1:
			// Second link is the target
			link, err := FurAffinityUrl().Parse(e.Attr("href"))
			if err != nil {
				parseError = true
				return true
			}
			comment.link = link
			comment.title = trimHtmlText(e.Text)
			break
		default:
			return false
		}

		return true
	})

	if comment.link != nil {
		id, err := commentIdFromFragment(comment.link.Fragment)
		if err != nil {
			parseError = true
		} else {
			comment.id = id
		}
	}

	entryElement.ForEach("span.popup_date", func(i int, e *colly.HTMLElement) {
		// Try using the data-time attribute first
		timeFromAttr, err := util.EpochStringToTime(e.Attr("data-time"))
		if err == nil {
			comment.date = timeFromAttr
			return
		}

		dateString := trimHtmlText(e.Text)
		date, err := time.ParseInLocation(entryDateLayout, dateString, fc.location())
		if err != nil {
			parseError = true
			return
		}
		comment.date = date
	})

	if parseError {
		return nil, errors.New("error parsing submission comment")
	}

	return &comment, nil
}

func commentIdFromFragment(fragment string) (uint, error) {
	idStr := strings.TrimPrefix(fragment, "cid:")
	id, err := strconv.ParseUint(idStr, 10, 32)
	return uint(id), err
}

func (fc *FurAffinityCollector) location() *time.Location {
	return fc.notesDateLocation()
}
