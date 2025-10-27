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

func (lpob *linkPreviewOptionsHelper) SetUrlRaw(url string) *linkPreviewOptionsHelper {
	lpob.URL = &url
	return lpob
}

func (lpob *linkPreviewOptionsHelper) SetUrl(url *url.URL) *linkPreviewOptionsHelper {
	if url == nil {
		return lpob.ClearUrl()
	}
	return lpob.SetUrlRaw(url.String())
}

func (lpob *linkPreviewOptionsHelper) SetThumbnailUrl(url *tools.ThumbnailUrl) *linkPreviewOptionsHelper {
	if url == nil {
		return lpob.ClearUrl()
	}
	return lpob.SetUrl(&url.URL)
}

func (lpob *linkPreviewOptionsHelper) ClearUrl() *linkPreviewOptionsHelper {
	lpob.URL = nil
	return lpob
}

func (lpob *linkPreviewOptionsHelper) SetDisabled(disabled bool) *linkPreviewOptionsHelper {
	lpob.IsDisabled = &disabled
	return lpob
}

func (lpob *linkPreviewOptionsHelper) ClearDisabled() *linkPreviewOptionsHelper {
	lpob.IsDisabled = nil
	return lpob
}

func (lpob *linkPreviewOptionsHelper) SetShowAboveText(showAbove bool) *linkPreviewOptionsHelper {
	lpob.ShowAboveText = &showAbove
	return lpob
}

func (lpob *linkPreviewOptionsHelper) ClearShowAboveText() *linkPreviewOptionsHelper {
	lpob.ShowAboveText = nil
	return lpob
}

func (lbop *linkPreviewOptionsHelper) Get() *models.LinkPreviewOptions {
	return &lbop.LinkPreviewOptions
}
