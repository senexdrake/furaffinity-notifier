package misc

import (
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/senexdrake/furaffinity-notifier/internal/conf"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const rawUrl = "https://dragon.vorwarts.com/commission"
const expectedText = "Request Form is currently closed"
const requestTimeout = 30 * time.Second
const targetElement = "#request_form"

var kitoraMessageContentTemplate = util.TrimHtmlText(`
KITORA REQUEST FORM OPEN!

Check <a href="%s">%s</a> now!
`)

var messageSent = false

func KitoraNotificationTarget() int64 {
	return conf.TelegramCreatorId
}

func KitoraHasNotified() bool {
	return messageSent
}
func KitoraSetNotified(b bool) {
	messageSent = b
}

func KitoraMessageContent() string {
	return fmt.Sprintf(kitoraMessageContentTemplate, rawUrl, rawUrl)
}

func KitoraCheckRequestFormOpen() (bool, error) {
	c := colly.NewCollector(colly.UserAgent(util.HttpDefaultUserAgent))
	c.SetRequestTimeout(requestTimeout)

	var callbackError error

	formOpen := false
	elementFound := false
	c.OnHTML(targetElement, func(element *colly.HTMLElement) {
		elementFound = true
		text := strings.ToLower(element.Text)
		formOpen = !strings.Contains(text, strings.ToLower(expectedText))
	})

	c.OnError(func(r *colly.Response, err error) {
		callbackError = err
	})

	err := c.Visit(rawUrl)
	if err != nil {
		return false, err
	}
	c.Wait()

	if callbackError != nil {
		return false, fmt.Errorf("error visiting Kitora commission page: %v", callbackError)
	}

	if !elementFound {
		return false, fmt.Errorf("could not find target element '%s' while visiting Kitora commission page", targetElement)
	}
	return formOpen, nil
}
