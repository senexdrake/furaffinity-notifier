package fa

import (
	"github.com/gocolly/colly"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

type (
	FurAffinityCollector struct {
		LimitConcurrency      int
		OnlyUnreadNotes       bool
		OnlySinceRegistration bool
		UserID                uint
	}
	FurAffinityUser struct {
		Name       string
		ProfileUrl *url.URL
	}
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0"
const faBaseUrl = "https://www.furaffinity.net"
const faDateLayout = "Jan 2, 2006 03:04PM"
const faTimezone = "America/Los_Angeles"
const faNoteSeparator = "—————————"

var (
	furaffinityBaseUrl, _         = url.Parse(faBaseUrl)
	furaffinityDefaultLocation, _ = time.LoadLocation(faTimezone)
)

func (nc *FurAffinityCollector) httpClient() *http.Client {
	cookieJar, _ := cookiejar.New(nil)
	cookieJar.SetCookies(furaffinityBaseUrl, nc.notesCookies())
	return &http.Client{
		Jar: cookieJar,
	}
}

func (nc *FurAffinityCollector) configuredCollector(withCookies bool) *colly.Collector {
	c := colly.NewCollector(
		colly.UserAgent(userAgent),
		colly.Async(true),
		colly.MaxDepth(2),
	)
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: nc.LimitConcurrency})

	if withCookies {
		c.SetCookies(faBaseUrl, nc.cookies())
	}

	return c
}

func (nc *FurAffinityCollector) cookieMap() map[string]*http.Cookie {
	cookies := make([]database.UserCookie, 0)
	database.Db().Where(&database.UserCookie{UserID: nc.UserID}).Find(&cookies)

	cookieMap := make(map[string]*http.Cookie)
	for _, cookie := range cookies {
		cookieMap[cookie.Name] = &http.Cookie{Value: cookie.Value, Name: cookie.Name}
	}
	return cookieMap
}

func (nc *FurAffinityCollector) cookies() []*http.Cookie {
	return util.Values(nc.cookieMap())
}

func (nc *FurAffinityCollector) user() *database.User {
	user := &database.User{}
	user.ID = nc.UserID
	database.Db().Limit(1).Find(user)
	return user
}

func (nc *FurAffinityCollector) registrationDate() time.Time {
	return nc.user().CreatedAt
}

func NewCollector(userId uint) *FurAffinityCollector {
	return &FurAffinityCollector{
		LimitConcurrency:      4,
		UserID:                userId,
		OnlyUnreadNotes:       true,
		OnlySinceRegistration: true,
	}
}

func FurAffinityUrl() *url.URL {
	return furaffinityBaseUrl
}

func trimHtmlText(s string) string {
	return util.TrimHtmlText(s)
}
