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

	"github.com/joho/godotenv"
	"github.com/senexdrake/furaffinity-notifier/internal/db"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/telegram"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const minimumUpdateInterval = 30 * time.Second
const enableOtherEntries = true
const enableSubmissions = true
const enableUserFilters = true
const enableSubmissionsContent = true

var entryUserFilters = make(map[entries.EntryType][]string)
var iterateSubmissionsBackwards = true

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
		backwardsIterationRaw := os.Getenv(util.PrefixEnvVar("SUBMISSIONS_BACKWARDS"))
		if backwardsIterationRaw != "" {
			backwardsIteration, err := strconv.ParseBool(backwardsIterationRaw)
			if err != nil {
				logging.Errorf("Error parsing bool: %s", err)
			} else {
				iterateSubmissionsBackwards = backwardsIteration
			}
		}

	}
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
		wg.Add(1)
		go updateForUser(&user, func() {
			wg.Done()
		})
	}

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

func updateForUser(user *db.User, doneCallback func()) {
	defer doneCallback()
	if user == nil {
		logging.Errorf("user is nil, skipping update")
		return
	}
	logging.Debugf("Running update for user %d", user.ID)
	c := fa.NewCollector(user)
	c.LimitConcurrency = 4
	c.IterateSubmissionsBackwards = iterateSubmissionsBackwards

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
	defer util.PanicHandler(func(err any) {
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
