package telegram

import (
	"github.com/senexdrake/furaffinity-notifier/internal/tmpl"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"html/template"
)

var baseTemplate = template.Must(
	template.New(tmpl.BaseTemplateName).Funcs(templateFuncMap()).ParseFS(tmpl.TemplateFS(), tmpl.TemplatePath(tmpl.BaseTemplateName)),
)

func createTemplate(targetTemplatePath string) (*template.Template, error) {
	cloned, err := baseTemplate.Clone()
	if err != nil {
		return nil, err
	}

	return cloned.ParseFS(tmpl.TemplateFS(), targetTemplatePath)
}

var newNoteMessageTemplate = template.Must(createTemplate(tmpl.TemplatePath("new-note.gohtml")))

var newCommentMessageTemplate = template.Must(createTemplate(tmpl.TemplatePath("new-comment.gohtml")))

var privacyPolicyTemplate = util.TrimHtmlText(`
This bot saves the following user information:

1. Your Chat ID (to identify you and match your data to your Telegram account)
	- In your case, this would be <code>%d</code>

2. Your provided user information:
	- Unread notes setting

3. Your FurAffinity cookies 
	- these are very sensitive, this allows the bot to fully impersonate you, which is required due to how FurAffinity works

4. A list of IDs that belong to your FurAffinity account: Note IDs, Comment IDs, Submission IDs and Journal IDs
	- this is needed to keep track of entries this bot has notified you about already. No content is stored, although it is fetched temporarily when notifying you.
`)

var statusTemplate = util.TrimHtmlText(`
Click one of the buttons to toggle it's respective settings.

Current settings:
<b>Notes</b>: %s
<b>Submissions</b>: %s (NOT IMPLEMENTED)
<b>Submission Comments</b>: %s
<b>Journals</b>: %s (NOT IMPLEMENTED)
<b>Journal Comments</b>: %s
`)

func templateFuncMap() template.FuncMap {
	return template.FuncMap{}
}
