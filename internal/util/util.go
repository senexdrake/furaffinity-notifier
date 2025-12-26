package util

import (
	"html"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fanonwue/goutils"
)

const (
	EmojiGreenCheck  = "✅"
	EmojiCross       = "❌"
	EmojiSquareRed   = "🟥"
	EmojiSquareBlue  = "🟦"
	EmojiSquareWhite = "⬜️"
)

func TrimHtmlText(s string) string {
	return strings.Trim(s, "\n ")
}

var envVarHelper = goutils.EnvVarHelper("FN_")

func EnvHelper() goutils.EnvVarHelper {
	return envVarHelper
}
func PrefixEnvVar(s string) string {
	return envVarHelper.PrefixVar(s)
}

func ToUTC(time *time.Time) *time.Time {
	if time == nil {
		return nil
	}
	utc := time.UTC()
	return &utc
}

func FixAutoLinks(dom *goquery.Selection) {
	shortenedLinks := dom.Find("a.auto_link_shortened")
	shortenedLinks.Each(func(i int, sel *goquery.Selection) {
		href, found := sel.Attr("href")
		if !found || href == "" {
			return
		}
		sel.SetText(href)
	})
}

func UnescapeHtml(s string) string {
	return html.UnescapeString(s)
}

// BackwardsChannel iterates over a channel, putting all elements into an internal buffer. It then produces a new channel,
// filling it with values from the buffer in reversed order.
func BackwardsChannel[T any](channel <-chan T) <-chan T {
	return BackwardsChannelWithCapacity(channel, 10)
}

// BackwardsChannelWithCapacity iterates over a channel, putting all elements into an internal buffer. It then produces a new channel,
// filling it with values from the buffer in reversed order.
// The specified capacity will be used for the internal buffer.
func BackwardsChannelWithCapacity[T any](channel <-chan T, cap uint) <-chan T {
	// TODO Replace places where this is used with better alternatives that do not require reading everything into a buffer first
	// Like starting iterating from the bottom of the HTML page maybe
	buf := make([]T, 0, cap)
	for t := range channel {
		buf = append(buf, t)
	}

	reversedChannel := make(chan T)
	go func() {
		defer close(reversedChannel)
		for _, t := range slices.Backward(buf) {
			reversedChannel <- t
		}
	}()

	return reversedChannel
}
