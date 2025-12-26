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

	"github.com/PuerkitoBio/goquery"
	"github.com/fanonwue/goutils"
	"github.com/fanonwue/goutils/dsext"
	"github.com/fanonwue/goutils/logging"
	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

type (
	submissionFetchContext struct {
		date           time.Time
		submissionData SubmissionDataMap
		blockedTags    dsext.Set[string]
	}
	SubmissionEntry struct {
		id             uint
		title          string
		from           *FurAffinityUser
		rating         Rating
		submissionType SubmissionType
		date           time.Time
		thumbnail      *tools.ThumbnailUrl
		tags           dsext.Set[string]
		blockedReason  dsext.Set[string]
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

func (se *SubmissionEntry) Rating() Rating {
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
	return se.from
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

func (se *SubmissionEntry) Tags() dsext.Set[string]           { return se.tags }
func (se *SubmissionEntry) BlockedReasons() dsext.Set[string] { return se.blockedReason }
func (se *SubmissionEntry) IsBlocked() bool                   { return len(se.BlockedReasons()) > 0 }

func (fc *FurAffinityCollector) submissionCollector() *colly.Collector {
	c := fc.configuredCollector(true)
	return c
}

func (fc *FurAffinityCollector) GetSubmissionEntries() <-chan *SubmissionEntry {
	c := fc.submissionCollector()

	channel := make(chan *SubmissionEntry, fc.channelBufferSize())

	c.OnHTML("body", func(bodyElement *colly.HTMLElement) {

		blockedTagsRaw := bodyElement.DOM.AttrOr("data-tag-blocklist", "")
		blockedTags := tools.TagListToSet(blockedTagsRaw)

		rawSubmissionData := bodyElement.DOM.Find("#js-submissionData").First().Text()
		submissionData := parseSubmissionData(rawSubmissionData)

		bodyElement.ForEach("#messagecenter-submissions .notifications-by-date", func(i int, e *colly.HTMLElement) {
			date, err := submissionSectionDate(e)
			if err != nil {
				logging.Warnf("Error parsing submission section date: %s", err)
				date = time.Time{}
			}

			if !fc.DateIsValid(entries.EntryTypeSubmission, date) {
				return
			}

			context := submissionFetchContext{
				date:           date,
				submissionData: submissionData,
				blockedTags:    blockedTags,
			}

			fc.submissionHandlerWrapper(
				channel,
				e,
				&context,
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
	context *submissionFetchContext,
) {

	wg := sync.WaitGroup{}
	// Add one to the WaitGroup to make sure it can't pass the Wait() call before these functions are being evaluated
	wg.Add(1)
	baseElement.ForEach("figure", func(i int, el *colly.HTMLElement) {
		wg.Add(1)
		defer wg.Done()

		entry, err := fc.parseSubmission(el, context)
		if err != nil || entry == nil {
			logging.Warnf("Error parsing submission: %s", err)
			return
		}

		if !fc.IsWhitelisted(entries.EntryTypeSubmission, entry.From().UserName) {
			return
		}

		data, found := context.submissionData[entry.ID()]
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

				content := fc.GetSubmissionContent(entry)
				if content == nil {
					logging.Warnf("Failed to retrieve content for submission %d", entry.ID())
					return
				}
				// The content might have more detailed date information, so we should check the submission date again
				if !fc.DateIsValid(entries.EntryTypeSubmission, content.date) {
					return
				}

				entry.SetContent(content)
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
		if valid {
			// content has already been found
			return
		}

		content.full = submissionFullView(entry.Type(), e)
		if content.full == nil {
			logging.Warnf("No full view link found for submission %d", entry.ID())
			valid = false
			return
		}

		// Try using the data-time attribute first
		timeFromAttr, err := goutils.EpochStringToTime(
			e.ChildAttr(".submission-id-container span.popup_date", "data-time"))
		if err != nil {
			logging.Warnf("Error parsing date for submission content (%d): %s", entry.ID(), err)
			valid = false
			return
		}
		content.date = timeFromAttr

		descriptionElement := e.DOM.Find(".submission-description").First()
		if descriptionElement.Length() == 0 {
			logging.Warnf("No description element found for submission %d", entry.ID())
			valid = false
			return
		}
		fc.removeHeadersAndFooters(descriptionElement, entries.EntryTypeSubmission)
		content.descriptionText = trimHtmlText(descriptionElement.Text())
		if content.descriptionText == "" {
			logging.Warnf("Empty description for submission %d, ignoring", entry.ID())
		}
		html, _ := descriptionElement.Html()
		content.descriptionHtml = html
		valid = true
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

func (fc *FurAffinityCollector) parseSubmission(entryElement *colly.HTMLElement, context *submissionFetchContext) (*SubmissionEntry, error) {
	entry := SubmissionEntry{
		date: context.date,
		from: userFromSubmissionPageElement(entryElement),
	}

	if entryElement.DOM.HasClass("t-image") {
		entry.submissionType = SubmissionTypeImage
	} else if entryElement.DOM.HasClass("t-text") {
		entry.submissionType = SubmissionTypeText
	}

	if entryElement.DOM.HasClass("r-general") {
		entry.rating = RatingGeneral
	} else if entryElement.DOM.HasClass("r-mature") {
		entry.rating = RatingMature
	} else if entryElement.DOM.HasClass("r-adult") {
		entry.rating = RatingAdult
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

	imgElement := entryElement.DOM.Find("img").First()
	entry.thumbnail = submissionThumbnail(imgElement)

	rawTags := imgElement.AttrOr("data-tags", "")
	entry.tags = tools.TagListToSet(rawTags)

	if fc.RespectBlockedTags {
		blockedReason := entry.tags.Intersect(context.blockedTags)
		if len(blockedReason) > 0 {
			entry.blockedReason = blockedReason
		}
	}

	return &entry, nil
}

func submissionSectionDate(el *colly.HTMLElement) (time.Time, error) {
	timeFromAttr, err := goutils.EpochStringToTime(el.Attr("data-date"))
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

func submissionThumbnail(imgElement *goquery.Selection) *tools.ThumbnailUrl {
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
		logging.Warnf("No 'data-fullview-src' found on submission img element")
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
