package database

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
		TelegramChatId  int64        `gorm:"uniqueIndex"`
		UnreadNotesOnly bool         `gorm:"default:true;not null"`
		KnownEntries    []KnownEntry `gorm:"constraint:OnDelete:CASCADE;"`
		Cookies         []UserCookie `gorm:"constraint:OnDelete:CASCADE;"`
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

const latestSchemaVersion = 2

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

func migrate() {
	migrator := Db().Migrator()

	if !migrator.HasTable(&SchemaInfo{}) {
		migrator.CreateTable(&SchemaInfo{})
		Db().Create(&SchemaInfo{Version: 1})

		if !migrator.HasTable(&User{}) {
			// Assume this is a completely new DB
			migrator.AutoMigrate(&User{}, &UserCookie{}, &KnownEntry{})
			err := updateSchemaVersion(latestSchemaVersion)
			if err != nil {
				panic(err)
			}
		}
	}

	schemaInfo := SchemaInfo{}
	db.First(&schemaInfo)

	if schemaInfo.Version < 2 {
		// Migrate from old known_notes table to the more generalized known_entries structure
		if migrator.HasTable("known_notes") && !migrator.HasTable(&KnownEntry{}) {
			migrator.CreateTable(&KnownEntry{})
			tx := Db().Begin()
			tx.Exec("INSERT INTO known_entries (`id`, `user_id`, `notified_at`, `sent_date`)" +
				" SELECT `id`, `user_id`, `notified_at`, `sent_date` FROM known_notes")
			tx.Model(&KnownEntry{}).
				Where("entry_type IS NULL OR entry_type = ?", entries.EntryTypeInvalid).
				Update("entry_type", entries.EntryTypeNote)
			tx.Commit()
			if tx.Error != nil {
				panic(tx.Error)
			}
			migrator.DropTable("known_notes")
		}
		schemaInfo.Version = 2
		err := updateSchemaVersion(schemaInfo.Version)
		if err != nil {
			panic(err)
		}
	}
}

func updateSchemaVersion(toVersion uint) error {
	tx := Db().Session(&gorm.Session{AllowGlobalUpdate: true}).Begin()
	tx.Model(&SchemaInfo{}).Update("version", toVersion)
	tx.Commit()
	return tx.Error
}

func CreateDatabase() {
	migrate()
	Db().AutoMigrate(&User{}, &UserCookie{}, &KnownEntry{})
}
