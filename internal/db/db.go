package db

import (
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"time"
)

type (
	SchemaInfo struct {
		Version uint
	}
	User struct {
		gorm.Model
		TelegramChatId            int64        `gorm:"uniqueIndex"`
		UnreadNotesOnly           bool         `gorm:"default:true;not null"`
		NotesEnabled              bool         `gorm:"default:true;not null"`
		SubmissionsEnabled        bool         `gorm:"default:false;not null"`
		SubmissionCommentsEnabled bool         `gorm:"default:false;not null"`
		JournalCommentsEnabled    bool         `gorm:"default:false;not null"`
		JournalsEnabled           bool         `gorm:"default:false;not null"`
		KnownEntries              []KnownEntry `gorm:"constraint:OnDelete:CASCADE;"`
		Cookies                   []UserCookie `gorm:"constraint:OnDelete:CASCADE;"`
	}

	UserCookie struct {
		UserID uint   `gorm:"uniqueIndex:name_per_user"`
		Name   string `gorm:"uniqueIndex:name_per_user"`
		Value  string
	}

	KnownEntry struct {
		EntryType  entries.EntryType `gorm:"primaryKey;autoIncrement:false;default:0;not null"`
		ID         uint              `gorm:"primaryKey;autoIncrement:false;not null"`
		UserID     uint              `gorm:"index;not null"`
		NotifiedAt *time.Time
		SentDate   time.Time
	}
)

func (u *User) EntryTypeStatus() map[entries.EntryType]bool {
	return map[entries.EntryType]bool{
		entries.EntryTypeNote:              u.NotesEnabled,
		entries.EntryTypeSubmission:        u.SubmissionsEnabled,
		entries.EntryTypeSubmissionComment: u.SubmissionCommentsEnabled,
		entries.EntryTypeJournal:           u.JournalsEnabled,
		entries.EntryTypeJournalComment:    u.JournalCommentsEnabled,
	}
}

func (u *User) EnabledEntryTypes() []entries.EntryType {
	entryTypes := make([]entries.EntryType, 0)

	for entryType, enabled := range u.EntryTypeStatus() {
		if enabled {
			entryTypes = append(entryTypes, entryType)
		}
	}

	return entryTypes
}

func (e *KnownEntry) BeforeSave(tx *gorm.DB) error {
	e.NotifiedAt = util.ToUTC(e.NotifiedAt)
	e.SentDate = e.SentDate.UTC()
	return nil
}

const latestSchemaVersion = 3

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
	migrate()
	Db().AutoMigrate(&User{}, &UserCookie{}, &KnownEntry{})
}
