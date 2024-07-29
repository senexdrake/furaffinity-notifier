package telegram

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"gorm.io/gorm"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"time"
)

var botInstance *bot.Bot
var botContext context.Context
var botContextCancel context.CancelFunc

var convHandler *ConversationHandler

var telegramCreatorId = 0

const creatorOnly = true

const (
	stageCookieInput = iota + 1
	stageSettings
)

func StartBot() *bot.Bot {
	telegramCreatorId, _ = strconv.Atoi(os.Getenv(util.PrefixEnvVar("TELEGRAM_CREATOR_ID")))

	botContext, botContextCancel = signal.NotifyContext(context.Background(), os.Interrupt)

	convEnd := ConversationEnd{
		Command:  "/cancel",
		Function: cancelConversationHandler,
	}

	convHandler = NewConversationHandler(map[int]bot.HandlerFunc{
		stageCookieInput: cookieInputHandler,
		stageSettings:    onSettingsKeyboardSelect,
	}, &convEnd)

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
		bot.WithMiddlewares(middlewares()...),
	}

	botToken := os.Getenv(util.PrefixEnvVar("TELEGRAM_BOT_TOKEN"))
	if botToken == "" {
		panic("No Telegram bot token has been set")
	}

	b, err := bot.New(botToken, opts...)
	if err != nil {
		panic(err)
	}

	commands := commandHandlers()

	registerHandlers(commands, b, botContext)
	registerCommands(commands, b, botContext)

	go func() {
		defer botContextCancel()
		b.Start(botContext)
	}()
	botInstance = b
	return b
}

func middlewares() []bot.Middleware {
	m := []bot.Middleware{
		convHandler.CreateHandlerMiddleware(),
	}

	if creatorOnly && telegramCreatorId > 0 {
		// Prepend creator only middleware to make sure it gets evaluated first
		m = append([]bot.Middleware{creatorOnlyMiddleware}, m...)
	}

	return m
}

func commandHandlers() []CommandHandler {
	sortedCommands := []CommandHandler{
		{
			Pattern:     "/cookies",
			Description: "Sets your FurAffinity cookies to access your private messages",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: cookieHandler,
			ChatAction:  models.ChatActionTyping,
		},
		{
			Pattern:     "/cancel",
			Description: "Cancels any active conversation",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: cancelConversationHandler,
			ChatAction:  models.ChatActionTyping,
		},
		{
			Pattern:     "/unread_only",
			Description: "Notify only about unread messages or about all new messages",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypePrefix,
			HandlerFunc: unreadOnlyHandler,
			ChatAction:  models.ChatActionTyping,
		},
		{
			Pattern:     "/settings",
			Description: "Change notification settings",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypePrefix,
			HandlerFunc: settingsHandler,
			ChatAction:  models.ChatActionTyping,
		},
	}

	slices.SortStableFunc(sortedCommands, func(a, b CommandHandler) int {
		return strings.Compare(a.Pattern, b.Pattern)
	})

	// Add unsorted commands to the bottom
	unsortedCommands := []CommandHandler{
		{
			Pattern:     "/start",
			Description: "Starts bot interaction",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: startHandler,
			ChatAction:  models.ChatActionTyping,
		},
		{
			Pattern:     "/privacy",
			Description: "Privacy policy",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: privacyPolicyHandler,
			ChatAction:  models.ChatActionTyping,
		},
	}

	commands := append(sortedCommands, unsortedCommands...)
	return commands
}

func registerHandlers(commands []CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	for _, command := range commands {
		handler := command.HandlerFunc
		if command.ChatAction != "" {
			handler = command.ChatActionHandler()
		}

		tgBot.RegisterHandler(command.HandlerType, command.Pattern, command.MatchType, handler)
	}
}

func registerCommands(commands []CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	tgBot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: util.Map(commands, func(ch CommandHandler) models.BotCommand {
			return models.BotCommand{Command: ch.Pattern, Description: ch.Description}
		}),
	})
}

func ShutdownBot() {
	botContextCancel()
}

func HandleNewNote(summary *fa.NoteEntry, user *database.User) {
	noteContent := "-- NO CONTENT --"
	if summary.HasContent() {
		noteContent = summary.Content().Text()
	}

	message := fmt.Sprintf(newNoteMessageTemplate,
		summary.From().ProfileUrl,
		summary.From().Name,
		summary.Title(),
		noteContent,
		summary.Link().String(),
		summary.ID(),
	)

	linkPreviewDisabled := true

	_, err := botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      message,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &linkPreviewDisabled,
		},
	})

	if err != nil {
		return
	}

	notifiedAt := time.Now()
	database.Db().Create(&database.KnownEntry{
		EntryType:  entries.EntryTypeNote,
		ID:         summary.ID(),
		UserID:     user.ID,
		NotifiedAt: &notifiedAt,
		SentDate:   summary.Date(),
	})
}

func HandleNewEntry(entry fa.Entry, user *database.User) {
	// TODO Implement!!!
	entryContent := "-- NO CONTENT --"
	if entry.HasContent() {
		entryContent = entry.Content().Text()
	}

	message := fmt.Sprintf(newCommentMessageTemplate,
		entry.From().ProfileUrl,
		entry.From().Name,
		entry.Title(),
		entryContent,
		entry.Link().String(),
		entry.ID(),
	)

	linkPreviewDisabled := true

	_, err := botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      message,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &linkPreviewDisabled,
		},
	})

	if err != nil {
		return
	}

	notifiedAt := time.Now()
	database.Db().Create(&database.KnownEntry{
		EntryType:  entry.EntryType(),
		ID:         entry.ID(),
		UserID:     user.ID,
		NotifiedAt: &notifiedAt,
		SentDate:   entry.Date(),
	})
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "default Handler",
	})
}

func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	tx := database.Db().Begin()
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
	tx := database.Db().Begin()
	user, _ := userFromChatId(update.Message.Chat.ID, tx)

	cookiesRaw := util.Map(strings.Split(update.Message.Text, ","), func(s string) string {
		return strings.TrimSpace(s)
	})

	cookies := make([]database.UserCookie, 0)

	for _, cookieKeyValue := range cookiesRaw {
		splitCookie := strings.Split(cookieKeyValue, "=")
		if len(splitCookie) != 2 {
			continue
		}

		cookies = append(cookies, database.UserCookie{
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

	tx.Delete(&database.UserCookie{}, "user_id = ?", user.ID)
	tx.Create(&cookies)
	tx.Commit()

	convHandler.EndConversation(update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Success",
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
	database.Db().Save(user)

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

func userFromChatId(chatId int64, tx *gorm.DB) (*database.User, bool) {
	if tx == nil {
		tx = database.Db()
	}
	user := &database.User{}
	tx.Limit(1).Find(user, "telegram_chat_id = ?", chatId)
	return user, user.ID > 0
}

func chatIdFromUpdate(update *models.Update) (int64, error) {
	chatId := int64(0)
	if update.Message != nil {
		chatId = update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		chatId = update.CallbackQuery.Message.Message.Chat.ID
	}

	if chatId == 0 {
		return 0, errors.New("could not determine chat ID")
	}
	return chatId, nil
}

func creatorOnlyMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		chatId, err := chatIdFromUpdate(update)
		if err != nil {
			return
		}

		if int64(telegramCreatorId) != chatId {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatId,
				Text:   "This bot is not yet available for the public. If you are interested, please contact this bot's creator (see bot description)",
			})
		} else {
			next(ctx, b, update)
		}
	}
}
