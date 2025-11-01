package util

import (
	"errors"
	"html"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

type Set[T comparable] map[T]struct{}

const EnvPrefix = "FN_"

const (
	EmojiGreenCheck  = "✅"
	EmojiCross       = "❌"
	EmojiSquareRed   = "🟥"
	EmojiSquareBlue  = "🟦"
	EmojiSquareWhite = "⬜️"
)

var truthyValues = []string{"1", "true", "yes", "on", "enable"}

func NewSet[T comparable](elements []T) Set[T] {
	set := make(Set[T], len(elements))
	set.AddAll(elements)
	return set
}

func (s Set[T]) Add(t T) {
	s[t] = struct{}{}
}

func (s Set[T]) AddAll(t []T) {
	for i := range t {
		s.Add(t[i])
	}
}

func (s Set[T]) Contains(t T) bool {
	_, ok := (s)[t]
	return ok
}

func ReverseMap[M ~map[K]V, K comparable, V comparable](m M) map[V]K {
	reversed := make(map[V]K, len(m))
	for k, v := range m {
		reversed[v] = k
	}
	return reversed
}

func Map[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func Values[M ~map[K]V, K comparable, V any](m M) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

func Join[T any](values []T, sep string, transform func(T) string) string {
	var stringified []string
	for _, v := range values {
		stringified = append(stringified, transform(v))
	}
	return strings.Join(stringified, sep)
}

func TrimHtmlText(s string) string {
	return strings.Trim(s, "\n ")
}

func PrefixEnvVar(s string) string {
	return EnvPrefix + s
}

func TruthyValues() []string {
	return truthyValues
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

func EpochStringToTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty epoch string")
	}

	timeAttr, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(timeAttr, 0), nil
}

func TruncateStringWholeWords(s string, maxLength uint) string {
	lastSpaceIx := -1
	length := uint(0)
	for i, r := range s {
		if unicode.IsSpace(r) {
			lastSpaceIx = i
		}
		length++
		if length >= maxLength {
			if lastSpaceIx != -1 {
				return s[:lastSpaceIx] + "..."
			}
			// If here, s is longer than maxLength but has no spaces
		}
	}
	// If here, s is shorter than maxLength
	return s
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

	reversedChannel := make(chan T, 10)
	go func() {
		defer close(reversedChannel)
		for _, t := range slices.Backward(buf) {
			reversedChannel <- t
		}
	}()

	return reversedChannel
}
