package conf

import (
	"os"
	"strconv"
	"sync"

	"github.com/fanonwue/goutils/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
)

const CreatorOnly = true

// MaxMessageContentLength is the maximum length of a message that can be sent to Telegram.
// The limit of a message is 4096 UTF-8 characters, but we have to take into account the whole template. Safer to
// use a smaller number.
const MaxMessageContentLength = 3072
const DefaultMessageContentLength uint = MaxMessageContentLength

var MessageContentLength = DefaultMessageContentLength
var TelegramCreatorId int64 = 0
var BotToken = ""

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

var setupDone = false
var setupMut = sync.Mutex{}

func Setup() {
	setupMut.Lock()
	defer setupMut.Unlock()
	if setupDone {
		return
	}
	MessageContentLength = readMessageContentLength()
	TelegramCreatorId = readTelegramCreatorId()
	BotToken = readBotToken()
	setupDone = true
}
