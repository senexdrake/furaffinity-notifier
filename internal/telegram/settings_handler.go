package telegram

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"strconv"
	"strings"
)

const buttonDataPrefix = "settings-"

var settingsKeyboardLayout = [][]entries.EntryType{
	{entries.EntryTypeNote},
	{entries.EntryTypeSubmission, entries.EntryTypeSubmissionComment},
	{entries.EntryTypeJournal, entries.EntryTypeJournalComment},
}

var settingsKeyboardTexts = []string{"Notes", "\n", "Submissions", "Submission Comments", "\n", "Journals", "Journal Comments"}

func entryTypeToText(entryType entries.EntryType) string {
	return entryType.Name()
}
func entryTypeToData(entryType entries.EntryType) string {
	return buttonDataPrefix + strconv.Itoa(int(entryType))
}

func dataToEntryType(data string) entries.EntryType {
	withoutPrefix := strings.TrimPrefix(data, buttonDataPrefix)
	value, err := strconv.Atoi(withoutPrefix)
	if err != nil {
		return entries.EntryTypeInvalid
	}
	return entries.EntryType(value)
}

func settingsKeyboard() *models.InlineKeyboardMarkup {
	buttons := util.Map(settingsKeyboardLayout, func(row []entries.EntryType) []models.InlineKeyboardButton {
		return util.Map(row, func(entryType entries.EntryType) models.InlineKeyboardButton {
			return models.InlineKeyboardButton{
				Text:         entryTypeToText(entryType),
				CallbackData: entryTypeToData(entryType),
			}
		})
	})

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "Cancel", CallbackData: "cancel"},
	})

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func settingsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	convHandler.SetActiveConversationStage(update.Message.Chat.ID, stageSettings)
	user, _ := userFromChatId(update.Message.Chat.ID, nil)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		ParseMode:   models.ParseModeHTML,
		Text:        entryTypeStatusList(user),
		ReplyMarkup: settingsKeyboard(),
	})
}

func onSettingsKeyboardSelect(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	message := update.CallbackQuery.Message.Message
	chatId := message.Chat.ID

	queryData := update.CallbackQuery.Data

	if queryData == "cancel" {
		convHandler.EndConversation(chatId)
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			MessageID: message.ID,
			ChatID:    chatId,
			ParseMode: models.ParseModeHTML,
			Text:      "Cancelled",
		})
	}

	tx := database.Db().Begin()
	user, _ := userFromChatId(chatId, tx)
	entryType := dataToEntryType(queryData)
	typeEnabled := false
	switch entryType {
	case entries.EntryTypeNote:
		user.NotesEnabled = !user.NotesEnabled
		typeEnabled = user.NotesEnabled
		break
	case entries.EntryTypeSubmission:
		user.SubmissionsEnabled = !user.SubmissionsEnabled
		typeEnabled = user.SubmissionsEnabled
		break
	case entries.EntryTypeSubmissionComment:
		user.SubmissionCommentsEnabled = !user.SubmissionCommentsEnabled
		typeEnabled = user.SubmissionCommentsEnabled
		break
	case entries.EntryTypeJournal:
		user.JournalsEnabled = !user.JournalsEnabled
		typeEnabled = user.JournalsEnabled
		break
	case entries.EntryTypeJournalComment:
		user.JournalCommentsEnabled = !user.JournalCommentsEnabled
		typeEnabled = user.JournalCommentsEnabled
		break
	default:
		return
	}

	tx.Save(user)
	tx.Commit()

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
		Text:            responseTextEntryTypeToggle(entryType, typeEnabled, false),
	})
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		MessageID:   message.ID,
		ChatID:      chatId,
		ParseMode:   models.ParseModeHTML,
		Text:        entryTypeStatusList(user),
		ReplyMarkup: settingsKeyboard(),
	})
}

func entryTypeStatusList(user *database.User) string {
	statusMap := user.EntryTypeStatus()
	statusFunc := func(entryType entries.EntryType) string {
		status := statusMap[entryType]
		if status {
			return util.EmojiGreenCheck
		}
		return util.EmojiCross
	}

	return fmt.Sprintf(
		statusTemplate,
		statusFunc(entries.EntryTypeNote),
		statusFunc(entries.EntryTypeSubmission),
		statusFunc(entries.EntryTypeSubmissionComment),
		statusFunc(entries.EntryTypeJournal),
		statusFunc(entries.EntryTypeJournalComment),
	)
}

func responseTextEntryTypeToggle(entryType entries.EntryType, enabled bool, html bool) string {
	enabledText := "enabled"
	if !enabled {
		enabledText = "disabled"
	}

	format := "Type <b>%s</b> has been <b>%s</b>"
	if !html {
		format = "Type %s has been %s"
	}

	return fmt.Sprintf(format, entryType.Name(), enabledText)
}
