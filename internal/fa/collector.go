package fa

import (
	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type (
	Entry interface {
		EntryType() entries.EntryType
		Date() time.Time
		Link() *url.URL
		ID() uint
		From() FurAffinityUser
		Title() string
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
		LimitConcurrency      int
		OnlyUnreadNotes       bool
		OnlySinceRegistration bool
		UserID                uint
	}
	FurAffinityUser struct {
		DisplayName string
		UserName    string
		ProfileUrl  *url.URL
	}
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0"
const faBaseUrl = "https://www.furaffinity.net"
const faTimezone = "America/Los_Angeles"
const faNoteSeparator = "—————————"
const faDefaultUsername = "UNKNOWN"

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

	if withCookies {
		c.SetCookies(faBaseUrl, fc.cookies())
	}

	return c
}

func (fc *FurAffinityCollector) cookieMap() map[string]*http.Cookie {
	cookies := make([]db.UserCookie, 0)
	db.Db().Where(&db.UserCookie{UserID: fc.UserID}).Find(&cookies)

	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = &http.Cookie{Value: cookie.Value, Name: cookie.Name}
	}
	return cookieMap
}

func (fc *FurAffinityCollector) cookies() []*http.Cookie {
	return util.Values(fc.cookieMap())
}

func (fc *FurAffinityCollector) user() *db.User {
	user := &db.User{}
	user.ID = fc.UserID
	db.Db().Limit(1).Find(user)
	return user
}

func (fc *FurAffinityCollector) registrationDate() time.Time {
	return fc.user().CreatedAt
}

func (fc *FurAffinityCollector) isEntryNew(entryType entries.EntryType, note uint) bool {
	searchNote := db.KnownEntry{
		EntryType: entryType,
		ID:        note,
		UserID:    fc.UserID,
	}
	foundRows := make([]db.KnownEntry, 0)

	db.Db().Where(&searchNote).Find(&foundRows)

	return len(foundRows) == 0
}

func NewCollector(userId uint) *FurAffinityCollector {
	return &FurAffinityCollector{
		LimitConcurrency:      4,
		UserID:                userId,
		OnlyUnreadNotes:       true,
		OnlySinceRegistration: true,
	}
}

func (fu FurAffinityUser) Name() string {
	name := fu.DisplayName
	if len(name) == 0 {
		name = fu.UserName
	}
	return name
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

func userFromElement(e *colly.HTMLElement) FurAffinityUser {
	if e == nil {
		return FurAffinityUser{UserName: faDefaultUsername}
	}

	var profileUrl *url.URL
	usernameElement := e.DOM.Find(".js-userName-block")
	href, exists := usernameElement.Attr("href")
	if exists {
		profileUrl, _ = FurAffinityUrl().Parse(href)
	}
	username := strings.Trim(trimHtmlText(usernameElement.Text()), "~")
	if profileUrl == nil && len(username) > 0 {
		// Just guess the URL
		profileUrl, _ = FurAffinityUrl().Parse("/user/" + username + "/")
	}

	displayName := trimHtmlText(e.ChildText(".js-displayName-block"))

	return FurAffinityUser{
		DisplayName: displayName,
		UserName:    username,
		ProfileUrl:  profileUrl,
	}
}
