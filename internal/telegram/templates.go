package telegram

import "github.com/senexdrake/furaffinity-notifier/internal/util"

var newNoteMessageTemplate = util.TrimHtmlText(`
New note on FurAffinity from <a href="%s">%s</a>!
---------------------------------
<b>%s</b>

%s
---------------------------------
<a href="%s">Open</a>
(Note ID: <code>%d</code>)
`)

var newCommentMessageTemplate = util.TrimHtmlText(`
New comment on FurAffinity from <a href="%s">%s</a>!
---------------------------------
On: <b>%s</b>

%s
---------------------------------
<a href="%s">Open</a>
(Comment ID: <code>%d</code>)
`)

var privacyPolicyTemplate = util.TrimHtmlText(`
This bot saves the following user information:

1. Your Chat ID (to identify you and match your data to your Telegram account)
	- In your case, this would be <code>%d</code>

2. Your provided user information:
	- Unread notes setting

3. Your FurAffinity cookies 
	- these are very sensitive, this allows the bot to fully impersonate you, which is required due to how FurAffinity works

4. A list of Note IDs that belong to your FurAffinity account
	- this is needed to keep track of notes this bot has notified you about already. No content is stored.
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
