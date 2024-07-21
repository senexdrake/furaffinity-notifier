package telegram

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"strings"
	"sync"
)

type ConversationStage map[int]bot.HandlerFunc

type ConversationEnd struct {
	Command  string
	Function bot.HandlerFunc
}

type ConversationHandler struct {
	stagesMutex         sync.RWMutex
	activeStagesPerChat map[int64]int
	stages              ConversationStage
	end                 *ConversationEnd
}

type ConversationBot struct {
	bot.Bot
	conversationHandler *ConversationHandler
}

func (c *ConversationHandler) CreateHandlerMiddleware() bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			hf := c.getStageFunction(update)
			if hf != nil {
				hf(ctx, bot, update)
			} else {
				next(ctx, bot, update)
			}
		}
	}
}

func NewConversationHandler(stages ConversationStage, end *ConversationEnd) *ConversationHandler {
	return &ConversationHandler{
		stagesMutex:         sync.RWMutex{},
		activeStagesPerChat: make(map[int64]int),
		stages:              stages,
		end:                 end,
	}
}

func (c *ConversationHandler) SetActiveConversationStage(chatId int64, stageId int) {
	c.stagesMutex.Lock()
	defer c.stagesMutex.Unlock()

	c.activeStagesPerChat[chatId] = stageId
}

func (c *ConversationHandler) EndConversation(chatId int64) {
	c.endConversationInternal(chatId, true)
}

func (c *ConversationHandler) endConversationInternal(chatId int64, lock bool) {
	if lock {
		c.stagesMutex.Lock()
		defer c.stagesMutex.Unlock()
	}

	delete(c.activeStagesPerChat, chatId)
}

func (c *ConversationHandler) getStageFunction(update *models.Update) bot.HandlerFunc {
	chatId := update.Message.Chat.ID

	stageId, active := c.stageIdForChat(chatId, true)

	if active {
		if strings.ToLower(update.Message.Text) == strings.ToLower(c.end.Command) {
			c.stagesMutex.Lock()
			defer c.stagesMutex.Unlock()
			// Retest condition after acquiring write lock
			_, active = c.stageIdForChat(chatId, false)
			if !active {
				// Return nil if it's not active anymore
				return nil
			}
			c.endConversationInternal(chatId, false)
			return c.end.Function
		}

		if hf, ok := c.stages[stageId]; ok {
			return hf
		}
	}

	return nil
}

func (c *ConversationHandler) stageIdForChat(chatId int64, lock bool) (int, bool) {
	if lock {
		c.stagesMutex.RLock()
		defer c.stagesMutex.RUnlock()
	}
	stageId, active := c.activeStagesPerChat[chatId]
	return stageId, active
}
