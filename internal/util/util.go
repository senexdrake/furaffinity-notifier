package util

import (
	"fmt"
	"html"
	"slices"
	"strings"
	"time"

	"github.com/fanonwue/goutils"
)

const (
	EmojiGreenCheck  = rune('✅')
	EmojiCross       = rune('❌')
	EmojiSquareRed   = rune('🟥')
	EmojiSquareBlue  = rune('🟦')
	EmojiSquareWhite = rune('⬜')
)

const (
	HttpDefaultUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:152.0) Gecko/20100101 Firefox/152.0"
	HttpDefaultRequestTimeout = 30 * time.Second
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
	return new(time.UTC())
}

func UnescapeHtml(s string) string {
	return html.UnescapeString(s)
}

func ParseDate(s string, layouts ...string) (time.Time, error) {
	return ParseDateInLocation(s, nil, layouts...)
}

func ParseDateInLocation(s string, location *time.Location, layouts ...string) (time.Time, error) {
	var t time.Time
	var err error
	for _, layout := range layouts {
		if location == nil {
			t, err = time.Parse(layout, s)
		} else {
			t, err = time.ParseInLocation(layout, s, location)
		}
		if err == nil {
			return t, nil
		}
	}
	return t, fmt.Errorf("failed to parse date: %s", s)
}

func NormalizeUsername(u string) string {
	return strings.ToLower(strings.TrimSpace(u))
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
