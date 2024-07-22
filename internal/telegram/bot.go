package telegram

import (
	"context"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"time"
)

type (
	CommandHandler struct {
		Pattern     string
		Description string
		HandlerType bot.HandlerType
		MatchType   bot.MatchType
		HandlerFunc bot.HandlerFunc
	}
)

var newNoteMessageTemplate = util.TrimHtmlText(`
New note on FurAffinity from <a href="%s">%s</a>!
---------------------------------
<b>%s</b>

%s
---------------------------------
<a href="%s">Open</a>
(Note ID: <code>%d</code>)
`)

var botInstance *bot.Bot
var botContext context.Context
var botContextCancel context.CancelFunc

var cookieConvHandler *ConversationHandler

var telegramCreatorId, _ = strconv.Atoi(os.Getenv(util.PrefixEnvVar("TELEGRAM_CREATOR_ID")))

const (
	creatorOnly      = true
	stageCookieInput = iota
)

func StartBot() *bot.Bot {
	botContext, botContextCancel = signal.NotifyContext(context.Background(), os.Interrupt)

	convEnd := ConversationEnd{
		Command:  "/cancel",
		Function: cancelConversation,
	}

	cookieConvHandler = NewConversationHandler(map[int]bot.HandlerFunc{
		stageCookieInput: cookieInputHandler,
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
		cookieConvHandler.CreateHandlerMiddleware(),
	}

	if creatorOnly && telegramCreatorId > 0 {
		// Prepend creator only middleware to make sure it gets evaluated first
		m = append([]bot.Middleware{creatorOnlyMiddleware}, m...)
	}

	return m
}

func commandHandlers() []CommandHandler {
	commands := []CommandHandler{
		{
			Pattern:     "/cookies",
			Description: "Sets your FurAffinity cookies to access your private messages",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: cookieHandler,
		},
		{
			Pattern:     "/start",
			Description: "Starts bot interaction",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: startHandler,
		},
		{
			Pattern:     "/cancel",
			Description: "Cancels any active conversation",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: cancelConversation,
		},
	}

	slices.SortStableFunc(commands, func(a, b CommandHandler) int {
		return strings.Compare(a.Pattern, b.Pattern)
	})

	return commands
}

func registerHandlers(commands []CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	for _, command := range commands {
		tgBot.RegisterHandler(command.HandlerType, command.Pattern, command.MatchType, command.HandlerFunc)
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

func SendMessage(s string, user *database.User) {
	botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID: user.TelegramChatId,
		Text:   s,
	})
}

func HandleNewNote(summary *fa.NoteSummary, user *database.User) {
	noteContent := "-- NO CONTENT --"
	if summary.Content != nil {
		noteContent = summary.Content.Text
	}

	message := fmt.Sprintf(newNoteMessageTemplate,
		summary.From.ProfileUrl,
		summary.From.Name,
		summary.Title,
		noteContent,
		summary.Link.String(),
		summary.ID,
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
	database.Db().Create(&database.KnownNote{
		ID:         summary.ID,
		UserID:     user.ID,
		NotifiedAt: &notifiedAt,
		SentDate:   summary.Date,
	})
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "default Handler",
	})
}

func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatId := update.Message.Chat.ID
	tx := database.Db().Begin()
	user := &database.User{}
	tx.Limit(1).Find(&user, "telegram_chat_id = ?", chatId)

	if user.ID > 0 {
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
	cookieConvHandler.SetActiveConversationStage(update.Message.Chat.ID, stageCookieInput)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		ParseMode: models.ParseModeHTML,
		Text:      "Please input cookies 'a' and 'b' im the following form:\n\n<code>a=COOKIE, b=COOKIE</code>",
	})

}

func cookieInputHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	tx := database.Db().Begin()
	user := database.User{TelegramChatId: update.Message.Chat.ID}
	tx.First(&user)

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

	cookieConvHandler.EndConversation(update.Message.Chat.ID)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Success",
	})
}

func cancelConversation(ctx context.Context, b *bot.Bot, update *models.Update) {

	// Send a message to indicate the conversation has been cancelled
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "conversation cancelled",
	})
}

func creatorOnlyMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if int64(telegramCreatorId) != update.Message.Chat.ID {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "This bot is not yet available for the public. If you are interested, please contact this bot's creator (see bot description)",
			})
		} else {
			next(ctx, b, update)
		}
	}
}
