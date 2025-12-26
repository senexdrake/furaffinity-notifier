package fa

import (
	"bytes"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fanonwue/goutils/dsext"
	"github.com/fanonwue/goutils/logging"
	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

type (
	BaseEntry interface {
		EntryType() entries.EntryType
		Date() time.Time
		Link() *url.URL
		ID() uint
		From() *FurAffinityUser
		Title() string
		Rating() Rating
	}

	Entry interface {
		BaseEntry
		Content() EntryContent
		SetContent(EntryContent)
		HasContent() bool
	}

	EntryContent interface {
		ID() uint
		Text() string
	}

	EntrySubmissionContent interface {
		ID() uint
		Text() string
		ImageURL() *url.URL
	}

	FurAffinityCollector struct {
		LimitConcurrency            int
		OnlySinceRegistration       bool
		OnlySinceTypeEnabled        bool
		IterateSubmissionsBackwards bool
		RespectBlockedTags          bool
		User                        *db.User
		userFilters                 map[entries.EntryType]dsext.Set[string]
	}
	FurAffinityUser struct {
		DisplayName string
		UserName    string
		ProfileUrl  *url.URL
	}
)

type Rating uint8

const (
	RatingGeneral Rating = iota
	RatingMature
	RatingAdult
)

func (r Rating) String() string {
	switch r {
	case RatingGeneral:
		return "General"
	case RatingMature:
		return "Mature"
	case RatingAdult:
		return "Adult"
	}
	panic("unreachable")
}

func (r Rating) Symbol() string {
	switch r {
	case RatingGeneral:
		return util.EmojiSquareWhite
	case RatingMature:
		return util.EmojiSquareBlue
	case RatingAdult:
		return util.EmojiSquareRed
	}
	panic("unreachable")
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:143.0) Gecko/20100101 Firefox/143.0"
const faBaseUrl = "https://www.furaffinity.net"
const faTimezone = "America/Los_Angeles"
const faNoteSeparator = "—————————"
const faDefaultUsername = "UNKNOWN"
const requestTimeout = 30 * time.Second

var (
	furaffinityBaseUrl, _         = url.Parse(faBaseUrl)
	furaffinityDefaultLocation, _ = time.LoadLocation(faTimezone)
)

func (fc *FurAffinityCollector) httpClient() *http.Client {
	cookieJar, _ := cookiejar.New(nil)
	cookieJar.SetCookies(furaffinityBaseUrl, fc.notesCookies())
	return &http.Client{
		Jar: cookieJar,
	}
}

func (fc *FurAffinityCollector) configuredCollector(withCookies bool) *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent(userAgent),
		colly.Async(true),
		colly.MaxDepth(2),
	)

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: fc.LimitConcurrency})
	c.SetRequestTimeout(requestTimeout)

	if withCookies {
		c.SetCookies(faBaseUrl, fc.cookies())
	}

	return c
}

func (fc *FurAffinityCollector) userCookies() []db.UserCookie {
	if fc.User.Cookies != nil {
		return fc.User.Cookies
	}
	cookies := make([]db.UserCookie, 0)
	db.Db().Where(&db.UserCookie{UserID: fc.UserID()}).Find(&cookies)
	return cookies
}

func (fc *FurAffinityCollector) cookieMap() map[string]*http.Cookie {
	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range fc.userCookies() {
		cookieMap[cookie.Name] = &http.Cookie{Value: cookie.Value, Name: cookie.Name}
	}
	return cookieMap
}

func (fc *FurAffinityCollector) cookies() []*http.Cookie {
	return dsext.Values(fc.cookieMap())
}

func (fc *FurAffinityCollector) registrationDate() time.Time {
	return fc.User.CreatedAt
}

func (fc *FurAffinityCollector) isEntryNew(entryType entries.EntryType, entryId uint) bool {
	searchEntry := db.KnownEntry{
		EntryType: entryType,
		ID:        entryId,
		UserID:    fc.UserID(),
	}
	rowCount := int64(0)

	db.Db().Model(&searchEntry).Where(&searchEntry).Count(&rowCount)

	return rowCount == 0
}

// IsWhitelisted returns true if the user is whitelisted for the given entry type or if there is no filter specified
// for that entry type.
func (fc *FurAffinityCollector) IsWhitelisted(entryType entries.EntryType, user string) bool {
	filter, found := fc.userFilters[entryType]
	// SetUserFilter does not allow empty filters to be set, so we don't need to check for them.
	if !found {
		return true
	}
	return filter.Contains(tools.NormalizeUsername(user))
}

func (fc *FurAffinityCollector) SetUserFilter(entryType entries.EntryType, users []string) {
	// If the filter is empty, remove it. Nil slice has a length of 0 too.
	if len(users) == 0 {
		delete(fc.userFilters, entryType)
		return
	}
	fc.userFilters[entryType] = dsext.NewSetSlice(dsext.Map(users, tools.NormalizeUsername))
}

func (fc *FurAffinityCollector) UserID() uint {
	return fc.User.ID
}

func (fc *FurAffinityCollector) OnlyUnreadNotes() bool {
	return fc.User.UnreadNotesOnly
}

// TypeEnabledSince returns the date at which the given entry type was enabled for the user.
func (fc *FurAffinityCollector) TypeEnabledSince(entryType entries.EntryType) time.Time {
	typeMap := fc.User.EntryTypeStatus()
	t := typeMap[entryType]
	return t.EnabledAt
}

// DateIsValid returns true if the given date is valid for the user. A date is valid if it is after the user's
// registration date and if it is after the date at which the entry type was enabled for the user.
func (fc *FurAffinityCollector) DateIsValid(entryType entries.EntryType, date time.Time) bool {
	if fc.OnlySinceRegistration && date.Before(fc.registrationDate()) {
		return false
	}
	if fc.OnlySinceTypeEnabled && date.Before(fc.TypeEnabledSince(entryType)) {
		return false
	}
	return true
}

func (fc *FurAffinityCollector) channelBufferSize() int {
	return fc.LimitConcurrency
}

func (fc *FurAffinityCollector) IsLoggedIn() (bool, error) {
	c := fc.configuredCollector(true)
	c.Async = false

	loggedIn := false

	c.OnResponse(func(r *colly.Response) {
		loggedIn = isLoggedIn(r)
	})

	err := c.Visit(faBaseUrl + "/controls/settings")
	if err != nil {
		return false, err
	}
	c.Wait()
	return loggedIn, nil
}

func NewCollector(user *db.User) *FurAffinityCollector {
	return &FurAffinityCollector{
		LimitConcurrency:            4,
		User:                        user,
		OnlySinceRegistration:       true,
		OnlySinceTypeEnabled:        true,
		IterateSubmissionsBackwards: false,
		userFilters:                 make(map[entries.EntryType]dsext.Set[string]),
	}
}

func (fu FurAffinityUser) Name() string {
	if fu.DisplayName != "" {
		return fu.DisplayName
	}
	return fu.UserName
}

func (fu FurAffinityUser) IsValid() bool {
	return fu.Name() != faDefaultUsername
}

func FurAffinityUrl() *url.URL {
	return furaffinityBaseUrl
}

func trimHtmlText(s string) string {
	return util.TrimHtmlText(s)
}

func userFromNoteElement(e *colly.HTMLElement) *FurAffinityUser {
	user := FurAffinityUser{UserName: faDefaultUsername}
	if e == nil {
		return &user
	}

	userLinkElement := e.DOM.Find("a").First()
	href, exists := userLinkElement.Attr("href")
	if exists {
		profileUrl, err := FurAffinityUrl().Parse(href)
		if err != nil {
			logging.Warnf("Error parsing profile URL from note: %s", err)
		} else {
			user.ProfileUrl = profileUrl
		}
	}

	// Determine the username from the profile URL
	if user.ProfileUrl != nil {
		parsedUserName, err := tools.UsernameFromProfileLink(user.ProfileUrl)
		if err != nil {
			logging.Warnf("Error parsing username from profile link for note: %s", err)
		} else {
			user.UserName = parsedUserName
		}
	}

	// Fallback to the old mechanism of extracting the username from the element text.
	if user.UserName == "" {
		usernameElement := e.DOM.Find(".js-userName-block")
		user.UserName = strings.Trim(trimHtmlText(usernameElement.Text()), "~")
	}

	if user.ProfileUrl == nil && user.UserName != "" {
		// Just guess the URL
		user.ProfileUrl, _ = FurAffinityUrl().Parse("/user/" + user.UserName + "/")
	}

	user.DisplayName = trimHtmlText(e.ChildText(".js-displayName-block"))
	return &user
}

func userFromSubmissionPageElement(e *colly.HTMLElement) *FurAffinityUser {
	user := FurAffinityUser{UserName: faDefaultUsername}
	if e == nil {
		return &user
	}

	e.ForEach("figcaption a", func(i int, element *colly.HTMLElement) {
		// The first occurrence is NOT the username
		if i == 0 {
			return
		}

		user.DisplayName = util.TrimHtmlText(element.Text)

		href := element.Attr("href")
		parsedUrl, err := FurAffinityUrl().Parse(href)
		if href != "" && err == nil {
			user.ProfileUrl = parsedUrl
		}
	})

	if user.ProfileUrl == nil {
		return &user
	}

	parsedUserName, err := tools.UsernameFromProfileLink(user.ProfileUrl)
	if err != nil {
		logging.Errorf("Error parsing username from profile link for submission: %s", err)
		return &user
	}
	user.UserName = parsedUserName
	return &user
}

func isLoggedIn(resp *colly.Response) bool {
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return false
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body)) // A byte reader does not have to be closed
	if err != nil {
		logging.Errorf("Error parsing login check responnse: %s", err)
		return false
	}

	pageBody := doc.Find("body")

	loginMessageContainer := pageBody.Find("#site-content .notice-message")
	if loginMessageContainer.Length() == 0 {
		return true
	}
	loginMessageText := strings.ToLower(loginMessageContainer.Text())
	loginMessagePresent := strings.Contains(loginMessageText, "system message") && strings.Contains(loginMessageText, "please log in")
	return !loginMessagePresent
}
