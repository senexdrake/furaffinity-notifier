package util

import (
	"errors"
	"html"
	"iter"
	"os"
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

func NewEmptySet[T comparable]() Set[T] {
	set := make(Set[T])
	return set
}

func NewSetSeq[T comparable](seq iter.Seq[T]) Set[T] {
	set := make(Set[T])
	set.AddAllSeq(seq)
	return set
}

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

func (s Set[T]) AddAllSeq(t iter.Seq[T]) {
	for v := range t {
		s.Add(v)
	}
}

func (s Set[T]) Contains(t T) bool {
	_, ok := (s)[t]
	return ok
}

func (s Set[T]) Intersect(other Set[T]) Set[T] {
	if len(s) == 0 || len(other) == 0 {
		return make(Set[T])
	}

	outer := s
	inner := other
	// The outer set should be the smaller one, as the Contains() method is O(1)
	if len(outer) > len(inner) {
		tmp := outer
		outer = inner
		inner = tmp
	}

	// The maximum size of the intersected Set is equal to the smaller Set, which will always be the outer one.
	// So it makes sense to set the capacity hint to the length of the outer Set.
	intersect := make(Set[T], len(outer))
	for v := range outer {
		if inner.Contains(v) {
			intersect.Add(v)
		}
	}
	return intersect
}

func (s Set[T]) Len() int {
	return len(s)
}

func (s Set[T]) IsEmpty() bool {
	return len(s) == 0
}

func (s Set[T]) Slice() []T {
	return Keys(s)
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

func MapSeq[T, U any](iterator iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range iterator {
			mapped := f(v)
			if !yield(mapped) {
				return
			}
		}
	}
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

	reversedChannel := make(chan T)
	go func() {
		defer close(reversedChannel)
		for _, t := range slices.Backward(buf) {
			reversedChannel <- t
		}
	}()

	return reversedChannel
}

func PanicHandler(handler func(err any)) {
	if err := recover(); err != nil {
		handler(err)
	}
}

func EnvBool(key string, defaultValue bool) (bool, error) {
	rawBool := os.Getenv(PrefixEnvVar(key))
	if rawBool != "" {
		return strconv.ParseBool(rawBool)
	}
	return defaultValue, nil
}
