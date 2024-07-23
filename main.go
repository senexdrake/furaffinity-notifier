package main

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/senexdrake/furaffinity-notifier/internal/database"
	"github.com/senexdrake/furaffinity-notifier/internal/fa"
	"github.com/senexdrake/furaffinity-notifier/internal/telegram"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const enableCommentNotifications = false

func main() {
	_ = godotenv.Load()

	database.CreateDatabase()
	telegram.StartBot()
	updateContext, updateContextCancel := signal.NotifyContext(context.Background(), os.Interrupt)
	go StartBackgroundUpdates(updateContext, updateInterval())

	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-quitChannel
	telegram.ShutdownBot()
	updateContextCancel()

	fmt.Println("Adios!")
}

func test() {
	user := database.User{}
	user.ID = 1
	database.Db().First(&user)
	c := fa.NewCollector(user.ID)
	c.LimitConcurrency = 4
	c.OnlyUnreadNotes = user.UnreadNotesOnly
	c.OnlySinceRegistration = false

	entryChannel := c.GetNewOtherEntriesWithContent()

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
	return interval
}

func StartBackgroundUpdates(ctx context.Context, interval time.Duration) {
	UpdateJob()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			UpdateJob()
		case <-ctx.Done():
			fmt.Println("Stopping BackgroundUpdates")
			// The context is over, stop processing results
			return
		}
	}
}

func UpdateJob() {
	users := make([]database.User, 0)
	database.Db().Find(&users)

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

func updateForUser(user *database.User, doneCallback func()) {
	defer doneCallback()
	c := fa.NewCollector(user.ID)
	c.LimitConcurrency = 4
	c.OnlyUnreadNotes = user.UnreadNotesOnly

	for note := range c.GetNewNotesWithContent() {
		telegram.HandleNewNote(note, user)
	}

	if enableCommentNotifications {
		for entry := range c.GetNewOtherEntriesWithContent() {
			telegram.HandleNewEntry(entry, user)
		}
	}
}
