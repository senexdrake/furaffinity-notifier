package db

import (
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
	"github.com/senexdrake/furaffinity-notifier/internal/logging"
	"github.com/senexdrake/furaffinity-notifier/internal/util"
	"gorm.io/gorm"
)

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

	migrateToV2(migrator, &schemaInfo)
	migrateToV3(migrator, &schemaInfo)
	migrateToV4(migrator, &schemaInfo)
	migrateToV5(migrator, &schemaInfo)
}

func migrateToV2(migrator gorm.Migrator, info *SchemaInfo) {
	if info.Version >= 2 {
		return
	}
	logging.Info("Migrating database to version 2")

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
	info.Version = 2
	err := updateSchemaVersion(info.Version)
	if err != nil {
		panic(err)
	}
	logging.Info("Done")
}

func migrateToV3(migrator gorm.Migrator, info *SchemaInfo) {
	if info.Version >= 3 {
		return
	}
	logging.Info("Migrating database to version 3")

	columnsToAdd := []string{
		"notes_enabled",
		"journals_enabled",
		"journal_comments_enabled",
		"submissions_enabled",
		"submission_comments_enabled",
	}

	for _, column := range columnsToAdd {
		if !migrator.HasColumn(&User{}, column) {
			migrator.AddColumn(&User{}, column)
		}
	}
	info.Version = 3
	err := updateSchemaVersion(info.Version)
	if err != nil {
		panic(err)
	}
	logging.Info("Done")
}

func migrateToV4(migrator gorm.Migrator, info *SchemaInfo) {
	if info.Version >= 4 {
		return
	}
	logging.Info("Migrating database to version 4")

	tzCol := "timezone"
	if migrator.HasColumn(&User{}, tzCol) {
		return
	}
	err := migrator.AddColumn(&User{}, tzCol)
	if err != nil {
		panic(err)
	}

	info.Version = 4
	err = updateSchemaVersion(info.Version)
	if err != nil {
		panic(err)
	}
	logging.Info("Done")
}

func migrateToV5(migrator gorm.Migrator, info *SchemaInfo) {
	if info.Version >= 5 {
		return
	}

	logging.Info("Migrating database to version 5")

	type OldUserSettings struct {
		ID                        uint
		NotesEnabled              bool
		SubmissionsEnabled        bool
		SubmissionCommentsEnabled bool
		JournalCommentsEnabled    bool
		JournalsEnabled           bool
	}

	enabledTypes := func(settings OldUserSettings) []entries.EntryType {
		types := make([]entries.EntryType, 0)
		if settings.NotesEnabled {
			types = append(types, entries.EntryTypeNote)
		}
		if settings.SubmissionsEnabled {
			types = append(types, entries.EntryTypeSubmission)
		}
		if settings.SubmissionCommentsEnabled {
			types = append(types, entries.EntryTypeSubmissionComment)
		}
		if settings.JournalsEnabled {
			types = append(types, entries.EntryTypeJournal)
		}
		if settings.JournalCommentsEnabled {
			types = append(types, entries.EntryTypeJournalComment)
		}
		return types
	}

	oldUserSettings := make([]OldUserSettings, 0)

	db.Table("users").
		Find(&oldUserSettings)

	err := migrator.CreateTable(UserEntryType{})
	if err != nil {
		panic(err)
	}

	for _, settings := range oldUserSettings {
		enabledEntryTypes := enabledTypes(settings)
		userEntryTypes := util.Map(enabledEntryTypes, func(e entries.EntryType) *UserEntryType {
			return NewUserEntryType(settings.ID, e)
		})
		db.Save(&userEntryTypes)
	}

	columnsToDrop := []string{
		"notes_enabled",
		"journals_enabled",
		"journal_comments_enabled",
		"submissions_enabled",
		"submission_comments_enabled",
	}

	for _, col := range columnsToDrop {
		err = migrator.DropColumn(&User{}, col)
		if err != nil {
			panic(err)
		}
	}

	info.Version = 5
	err = updateSchemaVersion(info.Version)
	if err != nil {
		panic(err)
	}
	logging.Info("Done")
}

func updateSchemaVersion(toVersion uint) error {
	tx := Db().Session(&gorm.Session{AllowGlobalUpdate: true}).Begin()
	tx.Model(&SchemaInfo{}).Update("version", toVersion)
	tx.Commit()
	return tx.Error
}
