package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/fanonwue/goutils/dsext"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/conf"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const buttonDataPrefix = "settings-"

var settingsKeyboardLayout = [][]entries.EntryType{
	{entries.EntryTypeNote},
	{entries.EntryTypeSubmission, entries.EntryTypeSubmissionComment},
	{entries.EntryTypeJournal, entries.EntryTypeJournalComment},
}

var settingsMessageMap = map[int64]int{}
var settingsMessageMapMutex = &sync.RWMutex{}

func setMessageIdForChat(chatId int64, messageId int) {
	settingsMessageMapMutex.Lock()
	defer settingsMessageMapMutex.Unlock()
	settingsMessageMap[chatId] = messageId
}

func deleteMessageIdForChat(chatId int64) {
	settingsMessageMapMutex.Lock()
	defer settingsMessageMapMutex.Unlock()
	delete(settingsMessageMap, chatId)
}

func messageIdForChat(chatId int64) (int, bool) {
	settingsMessageMapMutex.RLock()
	defer settingsMessageMapMutex.RUnlock()
	messageId, ok := settingsMessageMap[chatId]
	return messageId, ok
}

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
	buttons := dsext.Map(settingsKeyboardLayout, func(row []entries.EntryType) []models.InlineKeyboardButton {
		return dsext.Map(row, func(entryType entries.EntryType) models.InlineKeyboardButton {
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
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		ParseMode:   models.ParseModeHTML,
		Text:        entryTypeStatusList(user),
		ReplyMarkup: settingsKeyboard(),
	})
	if err == nil {
		// Set the current settings message to reference it later when editing
		setMessageIdForChat(update.Message.Chat.ID, msg.ID)
	}
}

func onSettingsKeyboardSelect(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId, err := chatIdFromUpdate(update)
	if err != nil {
		return
	}

	editStatusMessage := func(messageId int, user *db.User) {
		if user == nil {
			user, _ = userFromChatId(chatId, nil)
		}
		b.EditMessageText(ctx, &bot.EditMessageTextParams{
			MessageID: messageId,
			ChatID:    chatId,
			ParseMode: models.ParseModeHTML,
			Text:      entryTypeStatusList(user) + "\n\n\nConversation Cancelled!",
		})
	}

	if update.CallbackQuery == nil {
		if update.Message != nil {
			convHandler.EndConversation(chatId)
			defer deleteMessageIdForChat(chatId)
			messageId, found := messageIdForChat(chatId)
			if found {
				editStatusMessage(messageId, nil)
			}
			_, err = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatId,
				Text:   "Cancelled settings conversation. Please enter your message again.",
			})
			logSendMessageError(err)
		}
		return
	}

	message := update.CallbackQuery.Message.Message

	tx := db.Db().Begin()
	defer tx.Rollback()

	user, _ := userFromChatId(chatId, tx)

	queryData := update.CallbackQuery.Data
	if queryData == "cancel" {
		convHandler.EndConversation(chatId)
		editStatusMessage(message.ID, user)
		return
	}

	entryType := dataToEntryType(queryData)
	if !entries.ValidEntryTypesSet().Contains(entryType) {
		return
	}

	enabledEntryTypes := dsext.NewSetSlice(user.EnabledEntryTypes())
	typeEnabled := enabledEntryTypes.Contains(entryType)
	// Toggle status
	typeEnabled = !typeEnabled

	user.EnableEntryType(entryType, typeEnabled, tx)
	tx.Commit()

	// Refresh user
	user, _ = userFromChatId(chatId, nil)

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

func entryTypeStatusList(user *db.User) string {
	statusMap := user.EntryTypeStatus()
	filters := conf.EntryUserFilters()
	entryStatusFunc := func(entryType entries.EntryType) string {
		_, found := statusMap[entryType]
		if found {
			return util.EmojiGreenCheck
		}
		return util.EmojiCross
	}
	entryFilterFunc := func(entryType entries.EntryType) string {
		entryFilters := filters[entryType]
		if len(entryFilters) == 0 {
			return ""
		}
		return strings.Join(entryFilters, ",")
	}
	entryTextFunc := func(entryType entries.EntryType) string {
		status := entryStatusFunc(entryType)
		filter := entryFilterFunc(entryType)
		if filter == "" {
			return status
		}
		return fmt.Sprintf("%s\nFilter: <code>%s</code>", status, filter)
	}

	args := dsext.Map(entries.ValidEntryTypes(), func(t entries.EntryType) any {
		return entryTextFunc(t)
	})

	return fmt.Sprintf(
		statusTemplate,
		args...,
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
