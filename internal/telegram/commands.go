package telegram

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
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
	tx := db.Db().Begin()
	user, userFound := userFromChatId(chatId, tx)

	if userFound {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "You are already registered. Welcome back!",
		})
		tx.Commit()
		return
	}

	user.TelegramChatId = chatId
	tx.Create(&user)
	tx.Commit()

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatId,
		ParseMode: models.ParseModeHTML,
		Text:      "You have been registered as a user. Please set up your cookies using the /cookies command.",
	})
}

func cookieHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	convHandler.SetActiveConversationStage(update.Message.Chat.ID, stageCookieInput)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      "Please input cookies 'a' and 'b' im the following form:\n\n<code>a=COOKIE, b=COOKIE</code>",
	})

}

func cookieInputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	tx := db.Db().Begin()
	user, _ := userFromChatId(update.Message.Chat.ID, tx)

	cookiesRaw := util.Map(strings.Split(update.Message.Text, ","), func(s string) string {
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
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You entered invalid cookies. Please try again.",
		})
		return
	}

	tx.Delete(&db.UserCookie{}, "user_id = ?", user.ID)
	tx.Create(&cookies)
	tx.Commit()

	convHandler.EndConversation(update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Success",
	})
}

func timezoneHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	convHandler.SetActiveConversationStage(update.Message.Chat.ID, stageTimezoneInput)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      "Please input your timezone",
	})

}

func timezoneInputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	loc, err := time.LoadLocation(update.Message.Text)
	if err != nil || loc == nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("The timezone you specified is invalid. Please try again.\nError: %s", err),
		})
		return
	}

	tx := db.Db().Begin()
	user, _ := userFromChatId(update.Message.Chat.ID, tx)

	user.SetLocation(loc)

	tx.Save(user)
	tx.Commit()

	convHandler.EndConversation(update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf("Successfully update timezone to <b>%s</b>", user.Timezone),
	})
}

func cancelConversationHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Send a message to indicate the conversation has been cancelled
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "conversation cancelled",
	})
}

func unreadOnlyHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	user, userFound := userFromChatId(chatId, nil)
	if !userFound {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatId,
			Text:   "No user found for your Chat ID. Have you registered using the /start command?",
		})
	}

	unreadOnlyStatus := func(unreadOnly bool) string {
		if unreadOnly {
			return "unread"
		}
		return "all"
	}

	messageParts := util.Filter(strings.Split(update.Message.Text, " "), func(s string) bool {
		return s != ""
	})

	// First message part is always the command
	if len(messageParts) < 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatId,
			ParseMode: models.ParseModeHTML,
			Text: fmt.Sprintf("Please provide a parameter like 'on' or 'off'. Usage example:"+
				"\n\n/unread_only on"+
				"\n\nIt is currently set to <b>%s</b>", unreadOnlyStatus(user.UnreadNotesOnly)),
		})
		return
	}

	user.UnreadNotesOnly = slices.Contains(util.TruthyValues(), strings.ToLower(messageParts[1]))
	db.Db().Save(user)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatId,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf("Notifying about <b>%s</b> messages", unreadOnlyStatus(user.UnreadNotesOnly)),
	})
}

func privacyPolicyHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      fmt.Sprintf(privacyPolicyTemplate, update.Message.Chat.ID),
	})
}
