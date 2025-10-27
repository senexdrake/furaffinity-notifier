package telegram

import (
	"net/url"

	"github.com/go-telegram/bot/models"
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
