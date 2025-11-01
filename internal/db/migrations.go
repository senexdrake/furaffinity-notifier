package db

import (
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
}

func updateSchemaVersion(toVersion uint) error {
	tx := Db().Session(&gorm.Session{AllowGlobalUpdate: true}).Begin()
	tx.Model(&SchemaInfo{}).Update("version", toVersion)
	tx.Commit()
	return tx.Error
}
