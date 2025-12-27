package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fanonwue/goutils"
	"github.com/fanonwue/goutils/dsext"
	"github.com/fanonwue/goutils/logging"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"gorm.io/gorm"
)

func createPrivacyPolicyCommand() *CommandHandler {
	return &CommandHandler{
		Pattern:     "/privacy",
		Description: "Privacy policy",
		HandlerType: bot.HandlerTypeMessageText,
		MatchType:   bot.MatchTypeExact,
		HandlerFunc: privacyPolicyHandler,
		ChatAction:  models.ChatActionTyping,
	}
}

func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	txErr := db.Db().Transaction(func(tx *gorm.DB) error {
		user, userFound := userFromChatId(chatId, tx)

		if userFound {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatId,
				Text:   "You are already registered. Welcome back!",
			})
			logSendMessageError(err)
			tx.Commit()
			return nil
		}

		user.TelegramChatId = chatId
		tx.Create(&user)
		tx.Commit()

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatId,
			ParseMode: models.ParseModeHTML,
			Text:      "You have been registered as a user. Please set up your cookies using the /cookies command.",
		})
		logSendMessageError(err)
		return nil
	})
	if txErr != nil {
		logging.Errorf("Error occured while running startHandler transaction: %v", txErr)
	}
}

func cookieHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	convHandler.SetActiveConversationStage(update.Message.Chat.ID, stageCookieInput)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      conversationMessage("Please input cookies 'a' and 'b' im the following form:\n\n<code>a=COOKIE, b=COOKIE</code>"),
		ReplyMarkup: &models.ForceReply{
			ForceReply:            true,
			InputFieldPlaceholder: "a=COOKIE_1, b=COOKIE_2",
		},
	})
	logSendMessageError(err)

}

func cookieInputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	tx := db.Db().Begin()
	defer tx.Rollback()
	user, _ := userFromChatId(update.Message.Chat.ID, tx)

	cookiesRaw := dsext.Map(strings.Split(update.Message.Text, ","), func(s string) string {
		return strings.TrimSpace(s)
	})

	cookies := make([]db.UserCookie, 0)

	for _, cookieKeyValue := range cookiesRaw {
		splitCookie := strings.Split(cookieKeyValue, "=")
		if len(splitCookie) != 2 {
			continue
		}

		cookies = append(cookies, db.UserCookie{
			UserID: user.ID,
			Name:   splitCookie[0],
			Value:  splitCookie[1],
		})
	}

	if len(cookies) != 2 {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   conversationMessage("You entered invalid cookies. Please try again."),
		})
		logSendMessageError(err)
		return
	}

	tx.Delete(&db.UserCookie{}, "user_id = ?", user.ID)
	tx.Create(&cookies)
	tx.Commit()

	convHandler.EndConversation(update.Message.Chat.ID)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Updated your FA cookies!",
	})
	logSendMessageError(err)
}

func timezoneHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	user, _ := userFromChatId(chatId, nil)
	if user == nil {
		logging.Errorf("Error getting user from chat ID %d", chatId)
		return
	}
	convHandler.SetActiveConversationStage(chatId, stageTimezoneInput)

	timezone := user.Timezone
	location, err := user.GetLocation()
	if err == nil {
		timezone = location.String()
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      conversationMessage(fmt.Sprintf("Please input your timezone.\nCurrent timezone is <code>%s</code>.", timezone)),
		ReplyMarkup: &models.ForceReply{
			ForceReply:            true,
			InputFieldPlaceholder: "Enter timezone, for example: Europe/Berlin",
		},
	})
	logSendMessageError(err)

}

func timezoneInputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	loc, err := time.LoadLocation(update.Message.Text)
	if err != nil || loc == nil {
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   conversationMessage(fmt.Sprintf("The timezone you specified is invalid. Please try again.\nError: %s", err)),
		})
		logSendMessageError(err)
		return
	}

	var user *db.User
	_ = db.Db().Transaction(func(tx *gorm.DB) error {
		var found bool
		user, found = userFromChatId(chatId, tx)
		if !found || user == nil {
			return fmt.Errorf("no user found for chat ID %d", chatId)
		}
		user.SetLocation(loc)
		tx.Save(user)
		return nil
	})

	convHandler.EndConversation(chatId)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatId,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf("Successfully update timezone to <code>%s</code>", user.Timezone),
	})
	logSendMessageError(err)
}

func cancelConversationHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Send a message to indicate the conversation has been cancelled
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "conversation cancelled",
	})
	logSendMessageError(err)
}

func unreadOnlyHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	user, userFound := userFromChatId(chatId, nil)
	if !userFound {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "No user found for your Chat ID. Have you registered using the /start command?",
		})
		logSendMessageError(err)
	}

	unreadOnlyStatus := func(unreadOnly bool) string {
		if unreadOnly {
			return "unread"
		}
		return "all"
	}

	messageParts := dsext.Filter(strings.Split(update.Message.Text, " "), func(s string) bool {
		return s != ""
	})

	// First message part is always the command
	if len(messageParts) < 2 {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatId,
			ParseMode: models.ParseModeHTML,
			Text: fmt.Sprintf("Please provide a parameter like 'on' or 'off'. Usage example:"+
				"\n\n/unread_only on"+
				"\n\nIt is currently set to <b>%s</b>", unreadOnlyStatus(user.UnreadNotesOnly)),
		})
		logSendMessageError(err)
		return
	}

	user.UnreadNotesOnly = goutils.IsTruthy(messageParts[1])
	db.Db().Save(user)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatId,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf("Notifying about <b>%s</b> messages", unreadOnlyStatus(user.UnreadNotesOnly)),
	})
	logSendMessageError(err)
}

func privacyPolicyHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf(privacyPolicyTemplate, update.Message.Chat.ID),
	})
	logSendMessageError(err)
}
