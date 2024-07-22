package database

import (
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"time"
)

type (
	User struct {
		gorm.Model
		TelegramChatId  int64 `gorm:"uniqueIndex"`
		UnreadNotesOnly bool
		KnownNotes      []KnownNote  `gorm:"constraint:OnDelete:CASCADE;"`
		Cookies         []UserCookie `gorm:"constraint:OnDelete:CASCADE;"`
	}

	UserCookie struct {
		UserID uint   `gorm:"uniqueIndex:name_per_user"`
		Name   string `gorm:"uniqueIndex:name_per_user"`
		Value  string
	}

	KnownNote struct {
		ID         uint `gorm:"primaryKey"`
		UserID     uint
		NotifiedAt *time.Time
		SentDate   time.Time
	}
)

var db *gorm.DB

func Db() *gorm.DB {
	if db == nil {
		databasePath := os.Getenv(util.PrefixEnvVar("DATABASE_PATH"))
		if databasePath == "" {
			databasePath = "./data/main.db"
		}

		os.MkdirAll(filepath.Dir(databasePath), os.ModePerm)

		openedDb, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
		if err != nil {
			return nil
		}
		db = openedDb
	}

	return db
}

func CreateDatabase() {
	Db().AutoMigrate(&User{}, &UserCookie{}, &KnownNote{})
}
