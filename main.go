package main

import (
	"context"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fanonwue/goutils"
	"github.com/fanonwue/goutils/logging"
	"github.com/joho/godotenv"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/misc"
	"github.com/senexdrake/furaffinity-notifier/internal/telegram"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const minimumUpdateInterval = 30 * time.Second
const enableOtherEntries = true
const enableSubmissions = true
const enableUserFilters = true
const enableSubmissionsContent = true
const enableBlockedTags = true

var entryUserFilters = make(map[entries.EntryType][]string)
var iterateSubmissionsBackwards = true
var enableLoginCheck = true
var enableKitoraRequestFormCheck = false

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

	if enableUserFilters {
		for _, entryType := range entries.EntryTypes() {
			filter := userFiltersForEntryType(entryType)
			if filter != nil {
				entryUserFilters[entryType] = filter
				logging.Infof(
					"User filter for type '%s' enabled. Configured usernames: [%s]",
					entryType.Name(),
					strings.Join(filter, ", "),
				)
			}
		}
	}

	if enableSubmissions {
		iterateSubmissionsBackwards = envBoolLog("SUBMISSIONS_BACKWARDS", iterateSubmissionsBackwards)
	}

	enableLoginCheck = envBoolLog("ENABLE_LOGIN_CHECK", enableLoginCheck)
	enableKitoraRequestFormCheck = envBoolLog("ENABLE_KITORA_FORM_CHECK", enableKitoraRequestFormCheck)
}

func main() {
	db.CreateDatabase()
	appContext, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer cancel()

	logging.Infof("Starting Bot...")
	_ = telegram.StartBot(appContext)

	if enableKitoraRequestFormCheck {
		logging.Infof("Kitora request form check enabled. User %d will be notified about future availability.", misc.KitoraNotificationTarget())
	}

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
	if interval < minimumUpdateInterval {
		logging.Warnf("UPDATE_INTERVAL set too low, setting it to the minimum interval of %.0f seconds", minimumUpdateInterval.Seconds())
		interval = minimumUpdateInterval
	}
	return interval
}

func StartBackgroundUpdates(ctx context.Context, interval time.Duration) {
	logging.Infof("Starting background updates at an interval of %.0f seconds", interval.Seconds())
	UpdateJob()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			UpdateJob()
		case <-ctx.Done():
			logging.Info("Stopping BackgroundUpdates")
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
	// Do checks synchronously for now to prevent any massive rate limiting
	for _, user := range users {
		wg.Go(func() {
			updateForUser(&user)
		})
	}

	runMisc(&wg)

	wg.Wait()
}

func userFiltersForEntryType(entryType entries.EntryType) []string {
	envVar := ""
	switch entryType {
	case entries.EntryTypeSubmission:
		envVar = "SUBMISSIONS_USER_FILTER"
	case entries.EntryTypeJournal:
		envVar = "JOURNALS_USER_FILTER"
	case entries.EntryTypeNote:
		envVar = "NOTES_USER_FILTER"
	case entries.EntryTypeJournalComment, entries.EntryTypeSubmissionComment:
		envVar = "COMMENTS_USER_FILTER"
	case entries.EntryTypeInvalid:
	default:
		panic("unreachable")
	}

	if envVar == "" {
		return nil
	}

	filterRaw := os.Getenv(util.PrefixEnvVar(envVar))
	users := make([]string, 0)
	for _, userRaw := range strings.Split(filterRaw, ",") {
		user := tools.NormalizeUsername(userRaw)
		if user != "" {
			users = append(users, user)
		}
	}
	if len(users) == 0 {
		return nil
	}
	return users
}

func applyUserFilters(c *fa.FurAffinityCollector) {
	for entryType, users := range entryUserFilters {
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
	c.IterateSubmissionsBackwards = iterateSubmissionsBackwards
	c.RespectBlockedTags = enableBlockedTags

	if enableLoginCheck {
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
	if enableUserFilters {
		applyUserFilters(c)
	}

	entryTypes := user.EnabledEntryTypes()

	if slices.Contains(entryTypes, entries.EntryTypeNote) {
		channel := c.GetNewNotesWithContent()
		entryHandlerWrapper(user, channel, func(note *fa.NoteEntry) {
			telegram.HandleNewNote(note, user)
		})
	}

	if enableSubmissions && slices.Contains(entryTypes, entries.EntryTypeSubmission) {
		channel := submissionsChannel(c)
		entryHandlerWrapper(user, channel, func(submission *fa.SubmissionEntry) {
			telegram.HandleNewSubmission(submission, user)
		})
	}

	if enableOtherEntries {
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
	if enableSubmissionsContent {
		return c.GetNewSubmissionEntriesWithContent()
	}
	return c.GetNewSubmissionEntries()
}

func envBoolLog(key string, defaultValue bool) bool {
	ret, err := util.EnvHelper().Bool(key, defaultValue)
	if err != nil {
		logging.Errorf("Error parsing bool for key '%s': %s", key, err)
		return defaultValue
	}
	return ret
}

func runMisc(wg *sync.WaitGroup) {
	if enableKitoraRequestFormCheck {
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
