package telegram

import (
	"net/url"
	"os"
	"strconv"

	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

type linkPreviewOptionsHelper struct {
	models.LinkPreviewOptions
}

func defaultLinkPreviewOptionsHelper() *linkPreviewOptionsHelper {
	previewOptions := linkPreviewOptionsHelper{}
	previewOptions.SetDisabled(true)
	return &previewOptions
}

func defaultLinkPreviewOptions() *models.LinkPreviewOptions {
	return defaultLinkPreviewOptionsHelper().Get()
}

func (lpoh *linkPreviewOptionsHelper) SetUrlRaw(url string) *linkPreviewOptionsHelper {
	lpoh.URL = &url
	return lpoh
}

func (lpoh *linkPreviewOptionsHelper) SetUrl(url *url.URL) *linkPreviewOptionsHelper {
	if url == nil {
		return lpoh.ClearUrl()
	}
	return lpoh.SetUrlRaw(url.String())
}

func (lpoh *linkPreviewOptionsHelper) SetThumbnailUrl(url *tools.ThumbnailUrl) *linkPreviewOptionsHelper {
	if url == nil {
		return lpoh.ClearUrl()
	}
	return lpoh.SetUrl(&url.URL)
}

func (lpoh *linkPreviewOptionsHelper) ClearUrl() *linkPreviewOptionsHelper {
	lpoh.URL = nil
	return lpoh
}

func (lpoh *linkPreviewOptionsHelper) SetDisabled(disabled bool) *linkPreviewOptionsHelper {
	lpoh.IsDisabled = &disabled
	return lpoh
}

func (lpoh *linkPreviewOptionsHelper) ClearDisabled() *linkPreviewOptionsHelper {
	lpoh.IsDisabled = nil
	return lpoh
}

func (lpoh *linkPreviewOptionsHelper) SetShowAboveText(showAbove bool) *linkPreviewOptionsHelper {
	lpoh.ShowAboveText = &showAbove
	return lpoh
}

func (lpoh *linkPreviewOptionsHelper) ClearShowAboveText() *linkPreviewOptionsHelper {
	lpoh.ShowAboveText = nil
	return lpoh
}

func (lpoh *linkPreviewOptionsHelper) Get() *models.LinkPreviewOptions {
	return &lpoh.LinkPreviewOptions
}

const defaultMessageContentLength uint = maxMessageContentLength

var messageContentLength = readMessageContentLength()

func readMessageContentLength() uint {
	rawLength := os.Getenv(util.PrefixEnvVar("MAX_CONTENT_LENGTH"))
	if rawLength == "" {
		return defaultMessageContentLength
	}

	length, err := strconv.ParseUint(rawLength, 10, 32)
	if err != nil {
		logging.Warnf("Error parsing MAX_CONTENT_LENGTH, using default: %s", err)
	}

	if length > maxMessageContentLength {
		logging.Warnf("MAX_CONTENT_LENGTH set too large, using maximum value of %d", maxMessageContentLength)
		length = maxMessageContentLength
	}

	return uint(length)
}

func truncateMessage(message string) string {
	return util.TruncateStringWholeWords(message, messageContentLength)
}
