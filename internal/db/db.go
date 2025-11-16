package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type (
	SchemaInfo struct {
		Version uint
	}
	User struct {
		gorm.Model
		TelegramChatId           int64           `gorm:"uniqueIndex"`
		UnreadNotesOnly          bool            `gorm:"default:true;not null"`
		KnownEntries             []KnownEntry    `gorm:"constraint:OnDelete:CASCADE;"`
		Cookies                  []UserCookie    `gorm:"constraint:OnDelete:CASCADE;"`
		EntryTypes               []UserEntryType `gorm:"constraint:OnDelete:CASCADE;"`
		Timezone                 string          `gorm:"default:'UTC';not null"`
		InvalidCredentialsSentAt *time.Time
	}

	UserCookie struct {
		UserID uint   `gorm:"primaryKey;autoIncrement:false;not null"`
		Name   string `gorm:"primaryKey;not null"`
		Value  string
	}

	UserEntryType struct {
		UserID    uint              `gorm:"primaryKey:type_per_user;autoIncrement:false;not null"`
		EntryType entries.EntryType `gorm:"primaryKey:type_per_user;autoIncrement:false;not null"`
		EnabledAt time.Time         `gorm:"default:current_timestamp;not null"`
	}

	KnownEntry struct {
		EntryType  entries.EntryType `gorm:"primaryKey;autoIncrement:false;default:0;not null"`
		ID         uint              `gorm:"primaryKey;autoIncrement:false;not null"`
		UserID     uint              `gorm:"index;not null"`
		NotifiedAt *time.Time
		SentDate   time.Time
	}
)

func (u *User) EntryTypeStatus() map[entries.EntryType]UserEntryType {
	entryTypes := u.EntryTypes
	if entryTypes == nil {
		search := UserEntryType{UserID: u.ID}
		Db().Where(&search).Find(&entryTypes)
	}
	typeMap := make(map[entries.EntryType]UserEntryType, len(entryTypes))
	for _, entryType := range entryTypes {
		typeMap[entryType.EntryType] = entryType
	}
	return typeMap
}

func (u *User) EnabledEntryTypes() []entries.EntryType {
	status := u.EntryTypeStatus()
	entryTypes := make([]entries.EntryType, 0, len(status))
	for entryType := range status {
		entryTypes = append(entryTypes, entryType)
	}
	return entryTypes
}

func (u *User) EnabledEntryTypesSet() util.Set[entries.EntryType] {
	return util.NewSet(u.EnabledEntryTypes())
}

func (u *User) GetLocation() (*time.Location, error) {
	return time.LoadLocation(u.Timezone)
}

func (u *User) SetLocation(loc *time.Location) {
	u.Timezone = loc.String()
}

func (u *User) EnableEntryType(entryType entries.EntryType, enabled bool, tx *gorm.DB) {
	if tx == nil {
		tx = Db()
	}

	userEntryType := NewUserEntryType(u.ID, entryType)

	if enabled {
		count := int64(0)
		tx.Where(userEntryType).Count(&count)
		if count > 0 {
			return
		}
		tx.Save(userEntryType)
	} else {
		tx.Delete(userEntryType)
	}

}

func (u *User) SetCredentialsValid(valid bool) {
	if valid {
		u.InvalidCredentialsSentAt = nil
	} else {
		now := time.Now()
		u.InvalidCredentialsSentAt = &now
	}
}

func (u *User) ResetCredentialsValid(tx *gorm.DB) {
	if u.InvalidCredentialsSentAt == nil {
		return
	}
	u.SetCredentialsValidAndSave(true, tx)
}

func (u *User) SetCredentialsValidAndSave(valid bool, tx *gorm.DB) {
	u.SetCredentialsValid(valid)
	if tx == nil {
		tx = Db()
	}
	tx.Save(u)
}

func (u *User) InvalidCredentialsNotified() bool {
	return u.InvalidCredentialsSentAt != nil
}

func (e *KnownEntry) BeforeSave(tx *gorm.DB) error {
	e.NotifiedAt = util.ToUTC(e.NotifiedAt)
	e.SentDate = e.SentDate.UTC()
	return nil
}

func NewUserEntryType(userId uint, entryType entries.EntryType) *UserEntryType {
	uet := UserEntryType{
		UserID:    userId,
		EntryType: entryType,
		EnabledAt: time.Now().UTC(),
	}
	return &uet
}

func (uet *UserEntryType) BeforeSave(tx *gorm.DB) error {
	if uet.EntryType == entries.EntryTypeInvalid {
		return errors.New("invalid entry type")
	}
	return nil
}

const latestSchemaVersion = 6

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
			panic(fmt.Sprintf("error opening database: %s", err))
		}

		sqlDB, err := openedDb.DB()
		if err != nil {
			logging.Errorf("Error retrieving sql DB interface: %s", err)
		} else {
			// Fix SQlite "database is locked"
			sqlDB.SetMaxOpenConns(1)
		}

		db = openedDb
	}

	return db
}

func CreateDatabase() {
	migrate()
	Db().AutoMigrate(&User{}, &UserCookie{}, &KnownEntry{}, &UserEntryType{})
}
