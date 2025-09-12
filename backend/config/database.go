package config

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
)

var Database *gorm.DB

func Sync_DBv2() {
	var err error
	Database, err = gorm.Open(sqlite.Open("PoolManagerVM.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	Database.AutoMigrate(&models.User{}, &models.Serverpool{}, &models.Param{}, &models.Server{})

	allpool, err := utils.GetAllServerPool()
	if err != nil {
		panic("failed to connect to OpenStack")
	}

	models.PrintMapServerpool(allpool)

	for _, p := range allpool {
		for _, s := range p.ListServ {
			Database.FirstOrCreate(&s)
		}
		for _, param := range p.Params {
			Database.FirstOrCreate(&param)
		}
		Database.FirstOrCreate(&p)
	}
}
