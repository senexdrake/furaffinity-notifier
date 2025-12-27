package main

import (
	"context"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/fanonwue/goutils"
	"github.com/fanonwue/goutils/logging"
	"github.com/joho/godotenv"
	"github.com/senexdrake/furaffinity-notifier/internal/conf"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/misc"
	"github.com/senexdrake/furaffinity-notifier/internal/telegram"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

func init() {
	dotenvErr := godotenv.Load()
	logLevelErr := logging.SetLogLevelFromEnvironment(util.PrefixEnvVar("LOG_LEVEL"))
	if dotenvErr != nil {
		logging.Debugf("error loading .env file: %v", dotenvErr)
	}
	if logLevelErr != nil {
		logging.Errorf("error setting log level: %v", logLevelErr)
	}

	logging.Info("---- SETTING UP BOT ----")
	logging.Info("Welcome to FurAffinity Notifier!")

	conf.Setup()

	if conf.EnableKitoraRequestFormCheck() {
		logging.Infof("Kitora request form check enabled. User %d will be notified about future availability.", misc.KitoraNotificationTarget())
	}
}

func main() {
	appContext, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer cancel()
	db.CreateDatabase()

	logging.Infof("Starting Bot...")
	_ = telegram.StartBot(appContext)

	go StartBackgroundUpdates(appContext, updateInterval())

	<-appContext.Done()
	logging.Info("Bot exiting!")
}

func updateInterval() time.Duration {
	interval := 2 * time.Minute
	updateIntervalRaw, err := strconv.Atoi(os.Getenv(util.PrefixEnvVar("UPDATE_INTERVAL")))
	if err == nil {
		interval = time.Duration(updateIntervalRaw) * time.Second
	}
	if interval < conf.MinimumUpdateInterval {
		logging.Warnf("UPDATE_INTERVAL set too low, setting it to the minimum interval of %.0f seconds", conf.MinimumUpdateInterval.Seconds())
		interval = conf.MinimumUpdateInterval
	}
	return interval
}

func StartBackgroundUpdates(ctx context.Context, interval time.Duration) {
	logging.Infof("Starting background updates at an interval of %.0f seconds", interval.Seconds())
	defer logging.Info("BackgroundUpdates stopped")
	UpdateJob()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			UpdateJob()
		case <-ctx.Done():
			// The context is over, stop processing results
			return
		}
	}
}

func UpdateJob() {
	users := make([]db.User, 0)
	db.Db().
		Preload("EntryTypes").
		Preload("Cookies").
		Find(&users)

	wg := sync.WaitGroup{}

	if conf.EnableMiscJobs {
		runMisc(&wg)
	}

	// Do checks synchronously for now to prevent any massive rate limiting
	for _, user := range users {
		wg.Go(func() {
			updateForUser(&user)
		})
	}

	wg.Wait()
}

func applyUserFilters(c *fa.FurAffinityCollector) {
	for entryType, users := range conf.EntryUserFilters() {
		if users != nil && len(users) > 0 {
			c.SetUserFilter(entryType, users)
		}
	}
}

func updateForUser(user *db.User) {
	if user == nil {
		logging.Errorf("user is nil, skipping update")
		return
	}
	logging.Debugf("Running update for user %d", user.ID)
	c := fa.NewCollector(user)
	c.LimitConcurrency = 4
	c.IterateSubmissionsBackwards = conf.IterateSubmissionsBackwards()
	c.RespectBlockedTags = conf.EnableBlockedTags

	if conf.EnableLoginCheck() {
		// Check whether the user has valid credentials
		loggedIn, err := c.IsLoggedIn()
		if err != nil {
			logging.Errorf("Error checking login status for user %d: %s", c.UserID(), err)
			return
		}
		if !loggedIn {
			logging.Warnf("User %d does not have valid credentials, skipping", c.UserID())
			// Send notification if the user has not been notified yet
			if user.InvalidCredentialsSentAt == nil {
				telegram.HandleInvalidCredentials(user, true)
			}
			return
		}

		// User logged in, reset any invalid credentials notification data
		user.ResetCredentialsValid(nil)
	}

	// set filters
	if conf.EnableUserFilters {
		applyUserFilters(c)
	}

	entryTypes := user.EnabledEntryTypes()

	if slices.Contains(entryTypes, entries.EntryTypeNote) {
		channel := c.GetNewNotesWithContent()
		entryHandlerWrapper(user, channel, func(note *fa.NoteEntry) {
			telegram.HandleNewNote(note, user)
		})
	}

	if conf.EnableSubmissions && slices.Contains(entryTypes, entries.EntryTypeSubmission) {
		channel := submissionsChannel(c)
		entryHandlerWrapper(user, channel, func(submission *fa.SubmissionEntry) {
			telegram.HandleNewSubmission(submission, user)
		})
	}

	if conf.EnableOtherEntries {
		channel := c.GetNewOtherEntriesWithContent(entryTypes...)
		entryHandlerWrapper(user, channel, func(entry fa.Entry) {
			telegram.HandleNewEntry(entry, user)
		})
	}
	logging.Debugf("Finished update for user %d", user.ID)
}

func entryHandlerWrapper[T fa.BaseEntry](user *db.User, entryChannel <-chan T, entryHandler func(entry T)) {
	if user == nil {
		logging.Errorf("user is nil, skipping update")
		return
	}
	defer goutils.PanicHandler(func(err any) {
		logging.Errorf("Recovered from panic while running update for user %d: %s", user.ID, err)
	})

	for entry := range entryChannel {
		logging.Infof("Notifying user %d about '%s' %d", user.ID, entry.EntryType().Name(), entry.ID())
		entryHandler(entry)
	}

}

func submissionsChannel(c *fa.FurAffinityCollector) <-chan *fa.SubmissionEntry {
	if conf.EnableSubmissionsContent {
		return c.GetNewSubmissionEntriesWithContent()
	}
	return c.GetNewSubmissionEntries()
}

func runMisc(wg *sync.WaitGroup) {
	if conf.EnableKitoraRequestFormCheck() {
		wg.Go(func() {
			checkKitoraCommissionStatus(misc.KitoraNotificationTarget())
		})
	}
}

func checkKitoraCommissionStatus(notificationTarget int64) {
	if misc.KitoraHasNotified() {
		return
	}

	open, err := misc.KitoraCheckRequestFormOpen()
	if err != nil {
		logging.Errorf("error checking Kitora request form: %v", err)
		misc.KitoraSetNotified(true)
		return
	}

	if !open {
		return
	}

	logging.Infof("Notifiying user %d that the Kitora request form is open", notificationTarget)

	_, err = telegram.SendMessage(notificationTarget, misc.KitoraMessageContent())
	if err != nil {
		logging.Errorf("error sending Kitora request form notification: %v", err)
		return
	}

	misc.KitoraSetNotified(true)
}
