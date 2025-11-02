package fa

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
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

	JournalEntry struct {
		id      uint
		title   string
		from    FurAffinityUser
		date    time.Time
		link    *url.URL
		content *JournalContent
	}

	JournalContent struct {
		id   uint
		text string
	}

	message struct {
		title string
		from  FurAffinityUser
		date  time.Time
		link  *url.URL
	}
)

const otherMessagesPath = "/msg/others/"
const entryDateLayout = "January 2, 2006 03:04:05 PM"

func (ce *CommentEntry) EntryType() entries.EntryType { return ce.entryType }
func (ce *CommentEntry) Date() time.Time              { return ce.date }
func (ce *CommentEntry) Link() *url.URL               { return ce.link }
func (ce *CommentEntry) ID() uint                     { return ce.id }
func (ce *CommentEntry) Title() string                { return ce.title }
func (ce *CommentEntry) From() *FurAffinityUser       { return &ce.from }
func (ce *CommentEntry) Content() EntryContent        { return ce.content }
func (ce *CommentEntry) SetContent(ec EntryContent) {
	switch ec.(type) {
	case *CommentContent:
		ce.content = ec.(*CommentContent)
	default:
		panic("unknown content type")
	}
}
func (ce *CommentEntry) HasContent() bool { return ce.content != nil }

func (cc *CommentContent) ID() uint     { return cc.id }
func (cc *CommentContent) Text() string { return cc.text }

func (je *JournalEntry) ID() uint                     { return je.id }
func (je *JournalEntry) Title() string                { return je.title }
func (je *JournalEntry) From() *FurAffinityUser       { return &je.from }
func (je *JournalEntry) EntryType() entries.EntryType { return entries.EntryTypeJournal }
func (je *JournalEntry) Link() *url.URL               { return je.link }
func (je *JournalEntry) Content() EntryContent        { return je.content }
func (je *JournalEntry) SetContent(ec EntryContent) {
	switch ec.(type) {
	case *JournalContent:
		je.content = ec.(*JournalContent)
	default:
		panic("unknown content type")
	}
}
func (je *JournalEntry) Date() time.Time { return je.date }

func (jc *JournalContent) ID() uint     { return jc.id }
func (jc *JournalContent) Text() string { return jc.text }

func (je *JournalEntry) HasContent() bool { return je.Content() != nil }

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
	// Add one to the WaitGroup to make sure it can't pass the Wait() call before these functions are being evaluated
	wg.Add(1)
	baseElement.ForEach("li", func(i int, el *colly.HTMLElement) {
		wg.Add(1)
		defer wg.Done()
		entry := perEntryFunc(channel, &wg, el)
		if entry == nil || entry.ID() == 0 {
			return
		}
		if !fc.DateIsValid(entry.EntryType(), entry.Date()) {
			return
		}
		channel <- entry
	})
	wg.Done()
	wg.Wait()
}

func (fc *FurAffinityCollector) getOtherEntriesUnfiltered(entryTypes ...entries.EntryType) <-chan Entry {
	c := fc.otherCollector()

	channel := make(chan Entry)

	c.OnHTML("#messages-comments-submission", func(e *colly.HTMLElement) {
		entryType := entries.EntryTypeSubmissionComment
		if !slices.Contains(entryTypes, entryType) {
			return
		}
		handlerFunc := func(channel chan<- Entry, wg *sync.WaitGroup, element *colly.HTMLElement) Entry {
			parsed, err := fc.parseCommentEntry(entryType, element)
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
			parsed, err := fc.parseCommentEntry(entryType, element)
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

	c.OnHTML("#messages-journals", func(e *colly.HTMLElement) {
		if !slices.Contains(entryTypes, entries.EntryTypeJournal) {
			return
		}
		handlerFunc := func(channel chan<- Entry, wg *sync.WaitGroup, element *colly.HTMLElement) Entry {
			parsed, err := fc.parseJournalEntry(element)
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

	link, _ := FurAffinityUrl().Parse(otherMessagesPath)

	go func() {
		defer close(channel)
		c.Visit(link.String())
		c.Wait()
	}()

	return channel
}

func (fc *FurAffinityCollector) GetOtherEntries(entryTypes ...entries.EntryType) <-chan Entry {
	allEntries := fc.getOtherEntriesUnfiltered(entryTypes...)
	filteredEntries := make(chan Entry)
	go func() {
		defer close(filteredEntries)
		for entry := range allEntries {
			if fc.IsWhitelisted(entry.EntryType(), entry.From().UserName) {
				filteredEntries <- entry
			}
		}
	}()
	return filteredEntries
}

func (fc *FurAffinityCollector) GetNewOtherEntries(entryTypes ...entries.EntryType) <-chan Entry {
	newEntries := make(chan Entry)

	allEntries := fc.GetOtherEntries(entryTypes...)

	go func() {
		defer close(newEntries)
		for entry := range allEntries {
			if fc.isEntryNew(entry.EntryType(), entry.ID()) {
				newEntries <- entry
			}
		}
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
	case *JournalEntry:
		return fc.getJournalContent(entry.(*JournalEntry))
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

func (fc *FurAffinityCollector) getJournalContent(entry *JournalEntry) *JournalContent {
	c := fc.otherCollector()

	content := JournalContent{id: entry.ID()}

	valid := false

	c.OnHTML("#site-content .journal-content", func(e *colly.HTMLElement) {
		fc.removeHeadersAndFooters(e.DOM, entry.EntryType())
		util.FixAutoLinks(e.DOM)
		content.text = trimHtmlText(e.DOM.Text())
		valid = len(content.text) > 0
	})

	c.Visit(entry.Link().String())
	c.Wait()

	if !valid {
		return nil
	}

	return &content
}

func (fc *FurAffinityCollector) parseCommentEntry(entryType entries.EntryType, entryElement *colly.HTMLElement) (*CommentEntry, error) {
	msg, msgParseError := fc.parseMessage(entryType, entryElement)
	if msgParseError != nil {
		return nil, msgParseError
	}

	comment := CommentEntry{
		entryType: entryType,
		date:      msg.date,
		from:      msg.from,
		link:      msg.link,
		title:     msg.title,
	}

	parseError := false

	if comment.link != nil {
		id, err := commentIdFromFragment(comment.link.Fragment)
		if err != nil {
			parseError = true
		} else {
			comment.id = id
		}
	}

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

func (fc *FurAffinityCollector) parseJournalEntry(entryElement *colly.HTMLElement) (*JournalEntry, error) {
	msg, msgParseError := fc.parseMessage(entries.EntryTypeJournal, entryElement)
	if msgParseError != nil {
		return nil, msgParseError
	}

	journal := JournalEntry{
		date:  msg.date,
		from:  msg.from,
		link:  msg.link,
		title: msg.title,
	}

	parseError := false

	if journal.link != nil {
		id, err := journalIdFromLink(journal.link)
		if err != nil {
			parseError = true
		} else {
			journal.id = id
		}
	}

	if parseError {
		return nil, errors.New("error parsing journal")
	}

	return &journal, nil
}

func journalIdFromLink(link *url.URL) (uint, error) {
	if link == nil {
		return 0, errors.New("journal link is nil")
	}
	matches := journalIdRegex.FindStringSubmatch(link.Path)
	if len(matches) > 1 {
		id, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			return uint(id), nil
		}
		return 0, errors.New(fmt.Sprintf("error parsing journal ID '%s': %s", matches[1], err))
	}
	return 0, errors.New(fmt.Sprintf("no journal ID found in link '%s'", link.String()))

}

func (fc *FurAffinityCollector) parseMessage(entryType entries.EntryType, entryElement *colly.HTMLElement) (*message, error) {
	msg := message{}
	parseError := false

	indexAuthor := 0
	indexTitle := 1

	switch entryType {
	case entries.EntryTypeJournal:
		indexAuthor = 1
		indexTitle = 0
	default:
	}

	entryElement.ForEachWithBreak("a", func(i int, e *colly.HTMLElement) bool {
		if i == indexAuthor {
			// This link is the user
			link, err := FurAffinityUrl().Parse(e.Attr("href"))
			if err != nil {
				parseError = true
				return true
			}

			username, err := tools.UsernameFromProfileLink(link)
			if err != nil {
				logging.Errorf("error parsing username from profile link: %s", err)
				parseError = true
				return true
			}

			msg.from = FurAffinityUser{
				ProfileUrl:  link,
				UserName:    tools.NormalizeUsername(username),
				DisplayName: trimHtmlText(e.Text),
			}
		} else if i == indexTitle {
			// This is the title
			link, err := FurAffinityUrl().Parse(e.Attr("href"))
			if err != nil {
				parseError = true
				return true
			}
			msg.link = link
			msg.title = trimHtmlText(e.Text)
		} else {
			return false
		}

		return true
	})

	entryElement.ForEach("span.popup_date", func(i int, e *colly.HTMLElement) {
		// Try using the data-time attribute first
		timeFromAttr, err := util.EpochStringToTime(e.Attr("data-time"))
		if err == nil {
			msg.date = timeFromAttr
			return
		}

		dateString := trimHtmlText(e.Text)
		date, err := time.ParseInLocation(entryDateLayout, dateString, fc.location())
		if err != nil {
			parseError = true
			return
		}
		msg.date = date
	})

	if parseError {
		return nil, errors.New("error parsing message")
	}
	return &msg, nil
}

func (fc *FurAffinityCollector) location() *time.Location {
	return fc.notesDateLocation()
}
