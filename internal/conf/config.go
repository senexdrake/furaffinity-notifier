package conf

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fanonwue/goutils"
	"github.com/fanonwue/goutils/dsext"
	"github.com/fanonwue/goutils/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/tools"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const (
	EnableOtherEntries       = true
	EnableSubmissions        = true
	EnableUserFilters        = true
	EnableSubmissionsContent = true
	EnableBlockedTags        = true
	EnableMiscJobs           = true
)

const MinimumUpdateInterval = 30 * time.Second
const CreatorOnly = true

// MaxMessageContentLength is the maximum length of a message that can be sent to Telegram.
// The limit of a message is 4096 UTF-8 characters, but we have to take into account the whole template. Safer to
// use a smaller number.
const MaxMessageContentLength = 3072
const DefaultMessageContentLength uint = MaxMessageContentLength

var iterateSubmissionsBackwards = true
var enableLoginCheck = true
var enableKitoraRequestFormCheck = false

var MessageContentLength = DefaultMessageContentLength
var TelegramCreatorId int64 = 0
var BotToken = ""

var setupDone = false
var setupMut = sync.Mutex{}

func Setup() {
	setupMut.Lock()
	defer setupMut.Unlock()
	if setupDone {
		return
	}
	defer func() { setupDone = true }()
	MessageContentLength = readMessageContentLength()
	TelegramCreatorId = readTelegramCreatorId()
	BotToken = readBotToken()

	if EnableUserFilters {
		entryUserFilters = readEntryUserFilters()
	}

	if EnableSubmissions {
		iterateSubmissionsBackwards = envBoolLog("SUBMISSIONS_BACKWARDS", iterateSubmissionsBackwards)
	}

	enableLoginCheck = envBoolLog("ENABLE_LOGIN_CHECK", enableLoginCheck)

	if EnableMiscJobs {
		enableKitoraRequestFormCheck = envBoolLog("ENABLE_KITORA_FORM_CHECK", enableKitoraRequestFormCheck)
	}
}

func readMessageContentLength() uint {
	rawLength := os.Getenv(util.PrefixEnvVar("MAX_CONTENT_LENGTH"))
	if rawLength == "" {
		return DefaultMessageContentLength
	}

	length, err := strconv.ParseUint(rawLength, 10, 32)
	if err != nil {
		logging.Warnf("Error parsing MAX_CONTENT_LENGTH, using default: %s", err)
	}

	if length > MaxMessageContentLength {
		logging.Warnf("MAX_CONTENT_LENGTH set too large, using maximum value of %d", MaxMessageContentLength)
		length = MaxMessageContentLength
	}
	logging.Infof("Setting message content length to %d", length)
	return uint(length)
}

func readTelegramCreatorId() int64 {
	rawId := os.Getenv(util.PrefixEnvVar("TELEGRAM_CREATOR_ID"))
	id, err := strconv.ParseInt(rawId, 10, 64)
	if err != nil {
		logging.Panicf("Error parsing telegram creator id '%s': %v", rawId, err)
		return 0
	}
	return id
}

func readBotToken() string {
	botToken := os.Getenv(util.PrefixEnvVar("TELEGRAM_BOT_TOKEN"))
	if botToken == "" {
		panic("No Telegram bot token has been set")
	}
	return botToken
}

var entryUserFilters = make(map[entries.EntryType][]string)

func readEntryUserFilters() map[entries.EntryType][]string {
	filterMap := make(map[entries.EntryType][]string)
	for _, entryType := range entries.EntryTypes() {
		filter := userFiltersForEntryType(entryType)
		if len(filter) == 0 {
			continue
		}
		filterSlice := filter.Slice()
		filterMap[entryType] = filterSlice
		logging.Infof(
			"User filter for type '%s' enabled. Configured usernames: [%s]",
			entryType.Name(),
			strings.Join(filterSlice, ", "),
		)
	}
	return filterMap
}

func userFiltersForEntryType(entryType entries.EntryType) dsext.Set[string] {
	envVar := entryType.FilterEnvVar()
	if envVar == "" {
		return nil
	}

	filterRaw := os.Getenv(util.PrefixEnvVar(envVar))
	users := dsext.NewSet[string]()
	for _, userRaw := range goutils.SplitAny(filterRaw, ", ") {
		user := tools.NormalizeUsername(userRaw)
		if user != "" {
			users.Add(user)
		}
	}
	if len(users) == 0 {
		return nil
	}
	return users
}

func EntryUserFilters() map[entries.EntryType][]string {
	return entryUserFilters
}

func EnableLoginCheck() bool {
	return enableLoginCheck
}

func EnableKitoraRequestFormCheck() bool {
	return enableKitoraRequestFormCheck
}

func IterateSubmissionsBackwards() bool {
	return iterateSubmissionsBackwards
}

func envBoolLog(key string, defaultValue bool) bool {
	ret, err := util.EnvHelper().Bool(key, defaultValue)
	if err != nil {
		logging.Errorf("Error parsing bool for key '%s': %s", key, err)
		return defaultValue
	}
	return ret
}
