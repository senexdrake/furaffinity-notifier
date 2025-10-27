package main

import (
	"context"
	"fmt"
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
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/telegram"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const minimumUpdateInterval = 30 * time.Second
const enableOtherEntries = true

var submissionUsernameFilter = make([]string, 0)

func main() {
	appContext, _ := setup()
	_ = telegram.StartBot(appContext)

	go StartBackgroundUpdates(appContext, updateInterval())

	<-appContext.Done()
	logging.Info("Bot exiting!")
}

func setup() (context.Context, context.CancelFunc) {
	dotenvErr := godotenv.Load()
	logLevelErr := logging.SetLogLevelFromEnvironment(util.PrefixEnvVar("LOG_LEVEL"))
	if dotenvErr != nil {
		logging.Debugf("error loading .env file: %v", dotenvErr)
	}
	if logLevelErr != nil {
		logging.Errorf("error setting log level: %v", logLevelErr)
	}

	logging.Info("---- BOT STARTING ----")
	logging.Info("Welcome to FurAffinity Notifier!")

	rawSubmissionsUserFilter := os.Getenv(util.PrefixEnvVar("SUBMISSIONS_USER_FILTER"))
	for _, userRaw := range strings.Split(rawSubmissionsUserFilter, ",") {
		user := strings.TrimSpace(userRaw)
		if user != "" {
			submissionUsernameFilter = append(submissionUsernameFilter, user)
		}
	}

	if len(submissionUsernameFilter) > 0 {
		logging.Infof("Submissions user filter enabled. Configured usernames: %v", submissionUsernameFilter)
	}

	db.CreateDatabase()

	appContext, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	return appContext, cancel
}

func test() {
	user := db.User{}
	user.ID = 1
	db.Db().First(&user)
	c := fa.NewCollector(user.ID)
	c.LimitConcurrency = 4
	c.OnlyUnreadNotes = user.UnreadNotesOnly
	c.OnlySinceRegistration = false

	entryChannel := c.GetNewOtherEntries(entries.EntryTypeJournalComment, entries.EntryTypeSubmissionComment)

	for entry := range entryChannel {
		fmt.Println(entry)
	}
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
	db.Db().Find(&users)

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

func submissionsChannel(collector *fa.FurAffinityCollector) <-chan *fa.SubmissionEntry {
	if len(submissionUsernameFilter) > 0 {
		return collector.GetNewSubmissionEntriesFromUsers(slices.Clone(submissionUsernameFilter)...)
	}
	return collector.GetNewSubmissionEntries()
}

func updateForUser(user *db.User, doneCallback func()) {
	defer doneCallback()
	logging.Debugf("Running update for user %d", user.ID)
	c := fa.NewCollector(user.ID)
	c.LimitConcurrency = 4
	c.OnlyUnreadNotes = user.UnreadNotesOnly

	entryTypes := user.EnabledEntryTypes()

	if slices.Contains(entryTypes, entries.EntryTypeSubmission) {
		for submission := range submissionsChannel(c) {
			telegram.HandleNewSubmission(submission, user)
		}
	}

	if slices.Contains(entryTypes, entries.EntryTypeNote) {
		for note := range c.GetNewNotesWithContent() {
			telegram.HandleNewNote(note, user)
		}
	}

	if enableOtherEntries {
		entryChannel := c.GetNewOtherEntriesWithContent(entryTypes...)
		for entry := range entryChannel {
			telegram.HandleNewEntry(entry, user)
		}
	}
}
