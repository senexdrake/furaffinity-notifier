package telegram

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type CommandHandler struct {
	Pattern     string
	Description string
	ChatAction  models.ChatAction
	HandlerType bot.HandlerType
	MatchType   bot.MatchType
	HandlerFunc bot.HandlerFunc
}

func (ch *CommandHandler) ChatActionHandler() bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		b.SendChatAction(ctx, &bot.SendChatActionParams{
			ChatID: update.Message.Chat.ID,
			Action: ch.ChatAction,
		})
		ch.HandlerFunc(ctx, b, update)
	}
}
