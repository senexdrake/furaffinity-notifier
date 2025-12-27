package telegram

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/fanonwue/goutils/dsext"
	"github.com/fanonwue/goutils/logging"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/telegram/conf"
	"github.com/senexdrake/furaffinity-notifier/internal/tmpl"
	"gorm.io/gorm"
)

var botInstance *bot.Bot
var botContext context.Context
var privacyPolicyCommand = createPrivacyPolicyCommand()

var convHandler *ConversationHandler

const (
	stageCookieInput = iota + 1
	stageSettings
	stageTimezoneInput
)

func StartBot(ctx context.Context) *bot.Bot {
	conf.Setup()

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

	b, err := bot.New(conf.BotToken, opts...)
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

	if conf.CreatorOnly && conf.TelegramCreatorId > 0 {
		// Prepend creator-only middleware to make sure it gets evaluated first
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
		Commands: dsext.Map(commands, func(ch *CommandHandler) models.BotCommand {
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

func HandleInvalidCredentials(user *db.User, updateDatabase bool) {
	_, err := botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    user.TelegramChatId,
		ParseMode: models.ParseModeHTML,
		Text:      "Your cookies are invalid. Please set them again using the /cookies command.",
	})
	if err != nil {
		logging.Errorf("error sending invalid credentials notification: %s", err)
		return
	}
	if updateDatabase {
		user.SetCredentialsValidAndSave(false, nil)
	}
}

func HandleNewNote(summary *fa.NoteEntry, user *db.User) {
	noteContent := "-- NO CONTENT --"
	if summary.HasContent() {
		noteContent = summary.Content().Text()
	}

	buf := new(bytes.Buffer)
	err := newNoteMessageTemplate.Execute(buf, &tmpl.NewNotesContent{
		ID:      summary.ID(),
		Title:   summary.Title(),
		User:    summary.From(),
		Content: noteContent,
		Link:    summary.Link().String(),
	})

	if err != nil {
		logging.Errorf("error writing new notes template: %v", err)
		return
	}

	_, err = botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:             user.TelegramChatId,
		ParseMode:          models.ParseModeHTML,
		Text:               buf.String(),
		LinkPreviewOptions: defaultLinkPreviewOptions(),
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
	fullViewUrl := submission.FullView()
	fullViewUrlString := ""
	if fullViewUrl != nil {
		fullViewUrlString = fullViewUrl.String()
	}

	thumbnailUrl := submission.Thumbnail().WithSizeLarge()
	thumbnailUrlString := ""
	if thumbnailUrl != nil {
		thumbnailUrlString = thumbnailUrl.String()
	}

	buf := new(bytes.Buffer)
	err := newSubmissionMessageTemplate.Execute(buf, &tmpl.NewSubmissionsContent{
		ID:           submission.ID(),
		Title:        submission.Title(),
		Description:  submission.Description(),
		User:         submission.From(),
		Link:         submission.Link().String(),
		Rating:       submission.Rating(),
		Type:         submission.Type(),
		ThumbnailUrl: thumbnailUrlString,
		FullViewUrl:  fullViewUrlString,
		Blocked:      submission.IsBlocked(),
	})

	if err != nil {
		logging.Errorf("error writing new submissions template: %v", err)
		return
	}

	previewOptions := linkPreviewWithThumbnailOrFullView(fullViewUrl, thumbnailUrl)
	if submission.IsBlocked() {
		previewOptions.SetDisabled(true)
	}

	_, err = botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:             user.TelegramChatId,
		ParseMode:          models.ParseModeHTML,
		Text:               buf.String(),
		LinkPreviewOptions: previewOptions.Get(),
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
	entryContent := "-- NO CONTENT --"
	if entry.HasContent() {
		entryContent = entry.Content().Text()
	}

	buf := new(bytes.Buffer)
	linkPreviewOptions := defaultLinkPreviewOptionsHelper()

	switch entry.EntryType() {
	case entries.EntryTypeJournal:
		err := newJournalMessageTemplate.Execute(buf, &tmpl.NewJournalsContent{
			ID:      entry.ID(),
			Title:   entry.Title(),
			User:    entry.From(),
			Content: entryContent,
			Link:    entry.Link().String(),
			Rating:  entry.Rating(),
		})

		if err != nil {
			logging.Errorf("error writing new journals template: %v", err)
			return
		}

		linkPreviewOptions.SetDisabled(false)
		linkPreviewOptions.SetUrl(entry.Link())
	case entries.EntryTypeJournalComment, entries.EntryTypeSubmissionComment:
		err := newCommentMessageTemplate.Execute(buf, &tmpl.NewCommentsContent{
			ID:      entry.ID(),
			OnEntry: entry.Title(),
			User:    entry.From(),
			Content: entryContent,
			Link:    entry.Link().String(),
			Type:    entry.EntryType(),
			Rating:  entry.Rating(),
		})

		if err != nil {
			logging.Errorf("error writing new comments template: %v", err)
			return
		}
	default:
		logging.Errorf("unknown entry type in HandleNewEntry: %s", entry.EntryType())
		return
	}

	_, err := botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:             user.TelegramChatId,
		ParseMode:          models.ParseModeHTML,
		Text:               buf.String(),
		LinkPreviewOptions: linkPreviewOptions.Get(),
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

func SendMessage(chatId int64, message string) (*models.Message, error) {
	msg, err := botInstance.SendMessage(botContext, &bot.SendMessageParams{
		ChatID:    chatId,
		ParseMode: models.ParseModeHTML,
		Text:      message,
	})

	return msg, err
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
	tx.Preload("EntryTypes").Limit(1).Find(user, "telegram_chat_id = ?", chatId)
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

		if conf.TelegramCreatorId != chatId {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatId,
				Text:   "This bot is not yet available for the public. If you are interested, please contact this bot's creator (see bot description)",
			})
		} else {
			next(ctx, b, update)
		}
	}
}
