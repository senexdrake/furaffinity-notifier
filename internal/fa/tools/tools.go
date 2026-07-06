package tools

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fanonwue/goutils/dsext"
	"github.com/fanonwue/goutils/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/conf"
	faconf "github.com/senexdrake/furaffinity-notifier/internal/fa/conf"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

type ThumbnailUrl struct {
	url.URL
}

const ThumbnailSizeLarge = 600
const ThumbnailSizeSmall = 300

var thumbnailUrlSizeRegex = regexp.MustCompile("(.*@)(\\d*)(-.*)")

// profileUrlUsernameRegex matches the username portion of a profile URL.
// FA allows letters, numbers, dashes, dots, and tildes in usernames.
var profileUrlUsernameRegex = regexp.MustCompile(".*/user/([\\w-.~]*)/*")

func NewThumbnailUrl(url *url.URL) *ThumbnailUrl {
	if url == nil {
		return nil
	}
	return &ThumbnailUrl{*url}
}

func (tu *ThumbnailUrl) Size() uint {
	matches := thumbnailUrlSizeRegex.FindStringSubmatch(tu.Path)
	if len(matches) > 2 {
		id, err := strconv.ParseUint(matches[2], 10, 64)
		if err == nil {
			return uint(id)
		}
		logging.Errorf("Error parsing thumbnail size '%s': %s", matches[2], err)
	}
	return 0
}
func (tu *ThumbnailUrl) WithSize(height int) *ThumbnailUrl {
	newPath := thumbnailUrlSizeRegex.ReplaceAllString(tu.Path, "${1}"+strconv.Itoa(height)+"${3}")
	newUrl, _ := tu.Parse(newPath)
	return NewThumbnailUrl(newUrl)
}

func (tu *ThumbnailUrl) WithSizeLarge() *ThumbnailUrl {
	return tu.WithSize(ThumbnailSizeLarge)
}

func (tu *ThumbnailUrl) WithSizeSmall() *ThumbnailUrl {
	return tu.WithSize(ThumbnailSizeSmall)
}

func (tu *ThumbnailUrl) ToUrl() *url.URL {
	return new(tu.URL)
}

func UsernameFromProfileLink(link *url.URL) (string, error) {
	if link == nil {
		return "", errors.New("profile link is nil")
	}
	matches := profileUrlUsernameRegex.FindStringSubmatch(link.Path)
	if len(matches) > 1 {
		username := matches[1]
		if username == "" {
			return "", errors.New(fmt.Sprintf("empty username in profile link '%s'", link))
		}
		return username, nil
	}
	return "", errors.New(fmt.Sprintf("no username found in profile link '%s'", link))
}

func TagListToSet(rawTagList string) dsext.Set[string] {
	return dsext.NewSetSeq(dsext.MapSeq(strings.SplitSeq(rawTagList, " "), func(s string) string {
		return strings.TrimSpace(s)
	}))
}

func ParseDateFromString(entryType entries.EntryType, rawDate string, location *time.Location) (time.Time, error) {
	layoutsToTry := entryType.DateLayouts()
	date, err := util.ParseDateInLocation(rawDate, location, layoutsToTry...)
	if err != nil {
		layoutsFormatted := strings.Join(
			dsext.Map(layoutsToTry, func(s string) string { return fmt.Sprintf("'%s'", s) }),
			", ")
		msg := fmt.Sprintf("error parsing date: tried layouts [%s], got value '%s'", layoutsFormatted, rawDate)
		return time.Time{}, errors.New(msg)
	}
	return date, nil
}

func FixLinks(dom *goquery.Selection) {
	shortenedLinks := dom.Find("a.auto_link_shortened")
	shortenedLinks.Each(func(i int, sel *goquery.Selection) {
		href, found := sel.Attr("href")
		if !found || href == "" {
			return
		}
		parsed, err := url.Parse(href)
		if err != nil {
			logging.Warnf("Failed parsing message link: %s", err)
		}

		if conf.EnableExternalLinkRewrite() {
			parsed = removeFaExternalUrl(parsed)
		}

		sel.SetText(parsed.String())
	})
}

func removeFaExternalUrl(u *url.URL) *url.URL {
	if faconf.FaHost != u.Host || !strings.Contains(u.Path, "externalurl") {
		return u
	}
	queryValues := u.Query()
	target := queryValues.Get("q")
	if target == "" {
		return u
	}
	parsed, err := url.Parse(target)
	if err != nil {
		logging.Warnf("Failed parsing external link target: %s", err)
		return u
	}
	return parsed
}
