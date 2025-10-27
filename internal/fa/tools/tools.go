package tools

import (
	"net/url"
	"regexp"
	"strconv"

	"github.com/senexdrake/furaffinity-notifier/internal/logging"
)

type ThumbnailUrl struct {
	url.URL
}

const ThumbnailSizeLarge = 600
const ThumbnailSizeSmall = 300

var thumbnailUrlSizeRegex = regexp.MustCompile("(.*@)(\\d*)(-.*)")

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
