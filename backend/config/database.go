package config

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"PoolManagerVM/backend/models"
)

var Database *gorm.DB

func Sync_DB() {
	var err error
	Database, err = gorm.Open(sqlite.Open("PoolManagerVM.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	Database.AutoMigrate(&models.User{})
	Database.AutoMigrate(&models.ServerPool{})
}
