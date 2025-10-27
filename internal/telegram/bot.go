package telegram

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/tmpl"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"gorm.io/gorm"
)

var botInstance *bot.Bot
var botContext context.Context
var privacyPolicyCommand = createPrivacyPolicyCommand()

var convHandler *ConversationHandler

var telegramCreatorId = 0

const creatorOnly = true

const (
	stageCookieInput = iota + 1
	stageSettings
	stageTimezoneInput
)

func StartBot(ctx context.Context) *bot.Bot {
	telegramCreatorId, _ = strconv.Atoi(os.Getenv(util.PrefixEnvVar("TELEGRAM_CREATOR_ID")))

	var botContextCancel context.CancelFunc
	botContext, botContextCancel = context.WithCancel(ctx)

	convEnd := ConversationEnd{
		Command:  "/cancel",
		Function: cancelConversationHandler,
	}

	convHandler = NewConversationHandler(map[int]bot.HandlerFunc{
		stageCookieInput:   cookieInputHandler,
		stageSettings:      onSettingsKeyboardSelect,
		stageTimezoneInput: timezoneInputHandler,
	}, &convEnd)

	opts := []bot.Option{
		bot.WithErrorsHandler(errorHandler),
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

func errorHandler(err error) {
	logging.Logf(logging.LevelError, logging.DefaultCalldepth+1, "[TGBOT]: %v", err)
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

func registerHandlers(commands []*CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	for _, command := range commands {
		handler := command.HandlerFunc
		if command.ChatAction != "" {
			handler = command.ChatActionHandler()
		}

		tgBot.RegisterHandler(command.HandlerType, command.Pattern, command.MatchType, handler)
	}
}

func registerCommands(commands []*CommandHandler, tgBot *bot.Bot, ctx context.Context) {
	tgBot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: util.Map(commands, func(ch *CommandHandler) models.BotCommand {
			return models.BotCommand{Command: ch.Pattern, Description: ch.Description}
		}),
	})
}

func commandHandlers() []*CommandHandler {
	sortedCommands := []*CommandHandler{
		{
			Pattern:     "/cookies",
			Description: "Sets your FurAffinity cookies to access your private messages",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: cookieHandler,
			ChatAction:  models.ChatActionTyping,
		},
		{
			Pattern:     "/timezone",
			Description: "Sets your preferred timezone",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: timezoneHandler,
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

	slices.SortStableFunc(sortedCommands, func(a, b *CommandHandler) int {
		return strings.Compare(a.Pattern, b.Pattern)
	})

	// Add unsorted commands to the bottom
	unsortedCommands := []*CommandHandler{
		privacyPolicyCommand,
		{
			Pattern:     "/start",
			Description: "Starts bot interaction",
			HandlerType: bot.HandlerTypeMessageText,
			MatchType:   bot.MatchTypeExact,
			HandlerFunc: startHandler,
			ChatAction:  models.ChatActionTyping,
		},
	}

	commands := append(sortedCommands, unsortedCommands...)
	return commands
}

func HandleNewNote(summary *fa.NoteEntry, user *db.User) {
	noteContent := "-- NO CONTENT --"
	if summary.HasContent() {
		noteContent = summary.Content().Text()
	}

	buf := new(bytes.Buffer)
	err := newNoteMessageTemplate.Execute(buf, &tmpl.NewNotesContent{
		ID:       summary.ID(),
		Title:    summary.Title(),
		UserLink: summary.From().ProfileUrl.String(),
		UserName: summary.From().Name(),
		Content:  noteContent,
		Link:     summary.Link().String(),
	})

	if err != nil {
		logging.Errorf("error writing new notes template: %v", err)
		return
	}

	linkPreviewDisabled := true

	_, err = botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      buf.String(),
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &linkPreviewDisabled,
		},
	})

	if err != nil {
		logging.Errorf("error sending note notification: %v", err)
		return
	}

	notifiedAt := time.Now()
	db.Db().Create(&db.KnownEntry{
		EntryType:  entries.EntryTypeNote,
		ID:         summary.ID(),
		UserID:     user.ID,
		NotifiedAt: &notifiedAt,
		SentDate:   summary.Date(),
	})
}

func HandleNewSubmission(submission *fa.SubmissionEntry, user *db.User) {
	buf := new(bytes.Buffer)
	err := newSubmissionMessageTemplate.Execute(buf, &tmpl.NewSubmissionsContent{
		ID:       submission.ID(),
		Title:    submission.Title(),
		UserLink: submission.From().ProfileUrl.String(),
		Link:     submission.Link().String(),
		UserName: submission.From().UserName,
		Rating:   submission.Rating(),
		Type:     submission.Type(),
	})

	if err != nil {
		logging.Errorf("error writing new submissions template: %v", err)
		return
	}

	linkPreviewDisabled := true
	var linkPreviewUrl *string

	if submission.Thumbnail() != nil {
		linkPreviewDisabled = false
		tmpUrl := submission.Thumbnail().WithSizeLarge().String()
		linkPreviewUrl = &tmpUrl
	}

	_, err = botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      buf.String(),
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &linkPreviewDisabled,
			URL:        linkPreviewUrl,
		},
	})

	if err != nil {
		logging.Errorf("error sending submission notification: %v", err)
		return
	}

	notifiedAt := time.Now()
	db.Db().Create(&db.KnownEntry{
		EntryType:  entries.EntryTypeSubmission,
		ID:         submission.ID(),
		UserID:     user.ID,
		NotifiedAt: &notifiedAt,
		SentDate:   submission.Date(),
	})
}

func HandleNewEntry(entry fa.Entry, user *db.User) {
	// TODO Implement!!!
	entryContent := "-- NO CONTENT --"
	if entry.HasContent() {
		entryContent = entry.Content().Text()
	}

	buf := new(bytes.Buffer)
	err := newCommentMessageTemplate.Execute(buf, &tmpl.NewCommentsContent{
		ID:       entry.ID(),
		OnEntry:  entry.Title(),
		UserLink: entry.From().ProfileUrl.String(),
		UserName: entry.From().Name(),
		Content:  entryContent,
		Link:     entry.Link().String(),
	})

	if err != nil {
		logging.Errorf("error writing new comments template: %v", err)
		return
	}

	linkPreviewDisabled := true

	_, err = botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      buf.String(),
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &linkPreviewDisabled,
		},
	})

	if err != nil {
		logging.Errorf("error sending entry notification: %v", err)
		return
	}

	notifiedAt := time.Now()
	db.Db().Create(&db.KnownEntry{
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

func userFromChatId(chatId int64, tx *gorm.DB) (*db.User, bool) {
	if tx == nil {
		tx = db.Db()
	}
	user := &db.User{}
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
		// Always allow privacy policy command
		if update.Message != nil && strings.EqualFold(update.Message.Text, privacyPolicyCommand.Pattern) {
			next(ctx, b, update)
			return
		}

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
