package fa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
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
	SubmissionEntry struct {
		id             uint
		title          string
		from           FurAffinityUser
		rating         SubmissionRating
		submissionType SubmissionType
		date           time.Time
		thumbnail      *tools.ThumbnailUrl
		submissionData *SubmissionData
		content        *SubmissionContent
	}
	SubmissionContent struct {
		id              uint
		descriptionText string
		descriptionHtml string
		full            *url.URL
		thumbnail       *tools.ThumbnailUrl
		date            time.Time
	}

	SubmissionData struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Username    string `json:"username"`
		Lower       string `json:"lower"`
		AvatarMTime uint   `json:"avatar_mtime,string"`
	}
	SubmissionDataMap map[uint]*SubmissionData
)

type SubmissionRating uint8

func (sr SubmissionRating) String() string {
	switch sr {
	case SubmissionRatingGeneral:
		return "General"
	case SubmissionRatingMature:
		return "Mature"
	case SubmissionRatingAdult:
		return "Adult"
	}
	panic("unreachable")
}

func (sr SubmissionRating) Symbol() string {
	switch sr {
	case SubmissionRatingGeneral:
		return util.EmojiSquareWhite
	case SubmissionRatingMature:
		return util.EmojiSquareBlue
	case SubmissionRatingAdult:
		return util.EmojiSquareRed
	}
	panic("unreachable")
}

type SubmissionType uint8

func (st SubmissionType) String() string {
	switch st {
	case SubmissionTypeUnknown:
		return "Unknown"
	case SubmissionTypeImage:
		return "Image"
	case SubmissionTypeText:
		return "Text"
	}
	panic("unreachable")
}

const (
	SubmissionRatingGeneral SubmissionRating = iota
	SubmissionRatingMature
	SubmissionRatingAdult
)

const (
	SubmissionTypeUnknown SubmissionType = iota
	SubmissionTypeImage
	SubmissionTypeText
)

const submissionsPath = "/msg/submissions/new@72/"

var (
	submissionIdRegex = regexp.MustCompile(".*/view/(\\d*)/*")
)

func (se *SubmissionEntry) EntryType() entries.EntryType {
	return entries.EntryTypeSubmission
}

func (se *SubmissionEntry) Rating() SubmissionRating {
	return se.rating
}

func (se *SubmissionEntry) Type() SubmissionType {
	return se.submissionType
}

// Date returns the creation date of the submission. If no content is available, this is not the exact time but the time
// acquired from the whole block of the submission page.
func (se *SubmissionEntry) Date() time.Time {
	content := se.Content()
	if content != nil {
		date := content.date
		if !date.IsZero() {
			return date
		}
	}
	return se.date
}
func (se *SubmissionEntry) Link() *url.URL {
	submissionUrl, _ := FurAffinityUrl().Parse(fmt.Sprintf("/view/%d", se.ID()))
	return submissionUrl
}
func (se *SubmissionEntry) ID() uint {
	return se.id
}
func (se *SubmissionEntry) From() *FurAffinityUser {
	return &se.from
}
func (se *SubmissionEntry) Title() string {
	return se.title
}

func (se *SubmissionEntry) Description() string {
	content := se.Content()
	if content != nil {
		return content.descriptionText
	}
	data := se.SubmissionData()
	if data != nil {
		return data.Description
	}
	return ""
}

func (se *SubmissionEntry) Thumbnail() *tools.ThumbnailUrl {
	return se.thumbnail
}
func (se *SubmissionEntry) FullView() *url.URL {
	content := se.Content()
	if content == nil {
		return nil
	}
	return content.full
}

func (se *SubmissionEntry) SubmissionData() *SubmissionData {
	return se.submissionData
}

func (se *SubmissionEntry) Content() *SubmissionContent { return se.content }
func (se *SubmissionEntry) HasContent() bool {
	// TODO
	return se.content != nil
}
func (se *SubmissionEntry) SetContent(content *SubmissionContent) {
	se.content = content
}

func (fc *FurAffinityCollector) submissionCollector() *colly.Collector {
	c := fc.configuredCollector(true)
	return c
}

func (fc *FurAffinityCollector) GetSubmissionEntries() <-chan *SubmissionEntry {
	c := fc.submissionCollector()

	channel := make(chan *SubmissionEntry, fc.channelBufferSize())

	c.OnHTML("#site-content", func(siteElement *colly.HTMLElement) {

		rawSubmissionData := siteElement.DOM.Find("#js-submissionData").First().Text()
		submissionData := parseSubmissionData(rawSubmissionData)

		siteElement.ForEach("#messagecenter-submissions .notifications-by-date", func(i int, e *colly.HTMLElement) {
			date, err := submissionSectionDate(e)
			if err != nil {
				logging.Warnf("Error parsing submission section date: %s", err)
				date = time.Time{}
			}

			if !fc.DateIsValid(entries.EntryTypeSubmission, date) {
				return
			}

			fc.submissionHandlerWrapper(
				channel,
				e,
				date,
				submissionData,
			)
		})

	})

	link, _ := FurAffinityUrl().Parse(submissionsPath)

	go func() {
		defer close(channel)
		c.Visit(link.String())
		c.Wait()
	}()

	if fc.IterateSubmissionsBackwards {
		// The expected submission count is 72, so we can preallocate that amount
		return util.BackwardsChannelWithCapacity(channel, 72)
	}

	return channel
}

func (fc *FurAffinityCollector) submissionHandlerWrapper(
	channel chan<- *SubmissionEntry,
	baseElement *colly.HTMLElement,
	date time.Time,
	submissionData SubmissionDataMap,
) {

	wg := sync.WaitGroup{}
	// Add one to the WaitGroup to make sure it can't pass the Wait() call before these functions are being evaluated
	wg.Add(1)
	baseElement.ForEach("figure", func(i int, el *colly.HTMLElement) {
		wg.Add(1)
		defer wg.Done()

		entry, err := fc.parseSubmission(el, date)
		if err != nil || entry == nil {
			logging.Warnf("Error parsing submission: %s", err)
			return
		}

		if !fc.IsWhitelisted(entries.EntryTypeSubmission, entry.From().UserName) {
			return
		}

		data, found := submissionData[entry.ID()]
		if found {
			entry.submissionData = data
		}
		channel <- entry
	})

	wg.Done()
	wg.Wait()
}

func (fc *FurAffinityCollector) GetNewSubmissionEntries() <-chan *SubmissionEntry {
	filtered := make(chan *SubmissionEntry, fc.channelBufferSize())
	all := fc.GetSubmissionEntries()

	go func() {
		defer close(filtered)
		for submission := range all {
			if fc.isSubmissionNew(submission.ID()) {
				filtered <- submission
			}
		}
	}()

	return filtered
}

func (fc *FurAffinityCollector) GetNewSubmissionEntriesWithContent() <-chan *SubmissionEntry {
	return fc.submissionsWithContent(fc.GetNewSubmissionEntries())
}

func (fc *FurAffinityCollector) GetSubmissionEntriesWithContent() <-chan *SubmissionEntry {
	return fc.submissionsWithContent(fc.GetSubmissionEntries())
}

func (fc *FurAffinityCollector) submissionsWithContent(entryChannel <-chan *SubmissionEntry) <-chan *SubmissionEntry {
	channel := make(chan *SubmissionEntry, fc.channelBufferSize())
	go func() {
		defer close(channel)
		concurrencyLimit := fc.LimitConcurrency
		if concurrencyLimit <= 0 {
			concurrencyLimit = 1
		}

		guardChannel := make(chan struct{}, concurrencyLimit)

		wg := sync.WaitGroup{}
		for entry := range entryChannel {
			guardChannel <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() {
					<-guardChannel
					wg.Done()
				}()

				// Fetch note content without marking it as read, because we will do a batch operation alter
				entry.SetContent(fc.GetSubmissionContent(entry))
				channel <- entry
			}()
		}

		wg.Wait()
	}()

	return channel
}

func (fc *FurAffinityCollector) GetSubmissionContent(entry *SubmissionEntry) *SubmissionContent {
	if entry.Type() != SubmissionTypeImage {
		return nil
	}
	c := fc.otherCollector()

	content := SubmissionContent{id: entry.ID(), thumbnail: entry.Thumbnail()}

	valid := false

	c.OnHTML(".submission-content", func(e *colly.HTMLElement) {
		content.full = submissionFullView(entry.Type(), e)

		// Try using the data-time attribute first
		timeFromAttr, err := util.EpochStringToTime(
			e.ChildAttr(".submission-id-container span.popup_date", "data-time"))
		if err != nil {
			logging.Warnf("Error parsing date for submission content (%d): %s", entry.ID(), err)
			valid = false
			return
		}
		content.date = timeFromAttr

		descriptionElement := e.DOM.Find(".submission-description").First()
		content.descriptionText = trimHtmlText(descriptionElement.Text())
		html, _ := descriptionElement.Html()
		content.descriptionHtml = html

		valid = content.full != nil && content.descriptionText != ""
	})

	c.Visit(entry.Link().String())
	c.Wait()

	if !valid {
		return nil
	}

	return &content
}

func (fc *FurAffinityCollector) isSubmissionNew(id uint) bool {
	return fc.isEntryNew(entries.EntryTypeSubmission, id)
}

func (fc *FurAffinityCollector) parseSubmission(entryElement *colly.HTMLElement, date time.Time) (*SubmissionEntry, error) {
	entry := SubmissionEntry{
		date: date,
		from: userFromSubmissionPageElement(entryElement),
	}

	if entryElement.DOM.HasClass("t-image") {
		entry.submissionType = SubmissionTypeImage
	} else if entryElement.DOM.HasClass("t-text") {
		entry.submissionType = SubmissionTypeText
	}

	if entryElement.DOM.HasClass("r-general") {
		entry.rating = SubmissionRatingGeneral
	} else if entryElement.DOM.HasClass("r-mature") {
		entry.rating = SubmissionRatingMature
	} else if entryElement.DOM.HasClass("r-adult") {
		entry.rating = SubmissionRatingAdult
	}

	entryElement.ForEach("figcaption a", func(i int, captionElement *colly.HTMLElement) {
		if i != 0 {
			return
		}
		entry.title = util.TrimHtmlText(captionElement.Text)
		if entry.title == "" {
			entry.title = captionElement.Attr("title")
		}

		href := captionElement.Attr("href")
		if href == "" {
			return
		}
		link, err := FurAffinityUrl().Parse(href)
		if err != nil {
			logging.Errorf("Error parsing FA url: %s", err)
			return
		}

		id := submissionIdFromLink(link)
		if id == 0 {
			return
		}

		entry.id = id
	})

	if entry.ID() == 0 {
		return nil, errors.New("submission with empty ID is invalid")
	}

	entry.thumbnail = submissionThumbnail(entryElement)

	return &entry, nil
}

func submissionSectionDate(el *colly.HTMLElement) (time.Time, error) {
	timeFromAttr, err := util.EpochStringToTime(el.Attr("data-date"))
	if err != nil {
		return time.Time{}, err
	}
	return timeFromAttr.UTC(), nil
}

func submissionIdFromLink(link *url.URL) uint {
	if link == nil {
		return 0
	}
	matches := submissionIdRegex.FindStringSubmatch(link.Path)
	if len(matches) > 1 {
		id, err := strconv.ParseUint(matches[1], 10, 64)
		if err == nil {
			return uint(id)
		}
		logging.Errorf("Error parsing submission ID '%s': %s", matches[1], err)
	}
	return 0
}

func submissionThumbnail(el *colly.HTMLElement) *tools.ThumbnailUrl {
	imgElement := el.DOM.Find("img")
	if imgElement != nil {
		src, found := imgElement.Attr("src")
		if !found || src == "" {
			return nil
		}
		parsed, err := url.Parse(src)
		if err != nil {
			logging.Warnf("Failed parsing thumbnail URL for submission: %s", err)
			return nil
		}
		if parsed.Scheme == "" {
			parsed.Scheme = "https"
		}
		return tools.NewThumbnailUrl(parsed)
	}
	return nil
}

func submissionFullView(submissionType SubmissionType, el *colly.HTMLElement) *url.URL {
	if submissionType != SubmissionTypeImage {
		// Only images are supported for now
		return nil
	}
	imgElement := el.DOM.Find(".submission-image img").First()
	fullViewSrc, found := imgElement.Attr("data-fullview-src")
	if !found || fullViewSrc == "" {
		return nil
	}

	parsed, err := url.Parse(fullViewSrc)
	if err != nil {
		logging.Warnf("Failed parsing full view URL for submission: %s", err)
		return nil
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	return parsed
}

func parseSubmissionData(jsonData string) SubmissionDataMap {
	// First, unmarshal into map[string]SubmissionData to preserve the string keys
	var rawData map[string]*SubmissionData
	if err := json.Unmarshal([]byte(jsonData), &rawData); err != nil {
		logging.Errorf("Error unmarshaling submission data: %s", err)
		return make(SubmissionDataMap)
	}

	// Convert the string keys (submission IDs) to uint
	result := make(SubmissionDataMap, len(rawData))
	for idStr, data := range rawData {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			logging.Warnf("Error parsing submission ID '%s': %s", idStr, err)
			continue
		}

		data.Description = strings.TrimSpace(util.UnescapeHtml(data.Description))
		data.Title = strings.TrimSpace(util.UnescapeHtml(data.Title))

		result[uint(id)] = data
	}

	return result
}
