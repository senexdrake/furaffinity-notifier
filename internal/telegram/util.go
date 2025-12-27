package telegram

import (
	"net/url"

	"github.com/fanonwue/goutils"
	"github.com/fanonwue/goutils/logging"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/conf"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
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

func truncateMessage(message string) string {
	return goutils.TruncateStringWholeWords(message, conf.MessageContentLength)
}

func logSendMessageError(err error) {
	if err == nil {
		return
	}
	logging.Errorf("Error sending message: %s", err)
}

func linkPreviewWithThumbnailOrFullView(fullView *url.URL, thumbnail *tools.ThumbnailUrl) *linkPreviewOptionsHelper {
	previewOptions := defaultLinkPreviewOptionsHelper()

	enablePreview := func(url *url.URL) *linkPreviewOptionsHelper {
		previewOptions.SetDisabled(false)
		previewOptions.SetShowAboveText(false)
		previewOptions.SetUrl(url)
		return previewOptions
	}

	if fullView != nil {
		return enablePreview(fullView)
	}

	if thumbnail != nil {
		return enablePreview(thumbnail.ToUrl())
	}

	return previewOptions
}

func conversationMessage(msg string) string {
	return msg + conversationMessageSuffix
}
