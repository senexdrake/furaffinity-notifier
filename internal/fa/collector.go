package fa

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
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
		userFilters           map[entries.EntryType]util.Set[string]
	}
	FurAffinityUser struct {
		DisplayName string
		UserName    string
		ProfileUrl  *url.URL
	}
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:143.0) Gecko/20100101 Firefox/143.0"
const faBaseUrl = "https://www.furaffinity.net"
const faTimezone = "America/Los_Angeles"
const faNoteSeparator = "—————————"
const faDefaultUsername = "UNKNOWN"
const requestTimeout = 30 * time.Second

var (
	furaffinityBaseUrl, _         = url.Parse(faBaseUrl)
	furaffinityDefaultLocation, _ = time.LoadLocation(faTimezone)
	usernameRegex                 = regexp.MustCompile(".*/user/([\\w]*)/*")
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

func (fc *FurAffinityCollector) isEntryNew(entryType entries.EntryType, entryId uint) bool {
	searchEntry := db.KnownEntry{
		EntryType: entryType,
		ID:        entryId,
		UserID:    fc.UserID,
	}
	foundRows := make([]db.KnownEntry, 0)

	db.Db().Where(&searchEntry).Find(&foundRows)

	return len(foundRows) == 0
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
	set := make(util.Set[string], len(users))
	set.AddAll(util.Map(users, tools.NormalizeUsername))
	fc.userFilters[entryType] = set
}

func NewCollector(userId uint) *FurAffinityCollector {
	return &FurAffinityCollector{
		LimitConcurrency:      4,
		UserID:                userId,
		OnlyUnreadNotes:       true,
		OnlySinceRegistration: true,
		userFilters:           make(map[entries.EntryType]util.Set[string]),
	}
}

func NewCollectorForUser(user *db.User) *FurAffinityCollector {
	fc := NewCollector(user.ID)
	fc.OnlyUnreadNotes = user.UnreadNotesOnly
	return fc
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

func userFromNoteElement(e *colly.HTMLElement) FurAffinityUser {
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

func userFromSubmissionPageElement(e *colly.HTMLElement) FurAffinityUser {
	user := FurAffinityUser{UserName: faDefaultUsername}
	if e == nil {
		return user
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
		return user
	}

	user.UserName = usernameFromLink(user.ProfileUrl)
	return user
}

func usernameFromLink(link *url.URL) string {
	if link == nil {
		return ""
	}
	matches := usernameRegex.FindStringSubmatch(link.Path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
