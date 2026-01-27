package jobs

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"log"

	"gorm.io/gorm"
)

func IncrementPending(ServerpoolID uint) {
	result := config.Database.Model(&models.Serverpool{}).
		Where("id = ?", ServerpoolID).
		UpdateColumn("pending_jobs", gorm.Expr("pending_jobs + ?", 1))

	if result.Error != nil {
		log.Println("Error: ", result.Error)
	}
}

func DecrementPending(ServerpoolID uint) {
	result := config.Database.Model(&models.Serverpool{}).
		Where("id = ? AND pending_jobs > 0", ServerpoolID).
		UpdateColumn("pending_jobs", gorm.Expr("pending_jobs - ?", 1))

	if result.Error != nil {
		log.Println("Error: ", result.Error)
	}
}

func ChangePendingVol(serverID string) {
	res := config.Database.Model(&models.Server{}).
		Where("id = ?", serverID).
		UpdateColumn("vol_pending", gorm.Expr("NOT vol_pending"))

	if res.Error != nil {
		log.Println("Error: ", res.Error)
	}

}

func ChangePendingNFS(ServerpoolID uint) {
	result := config.Database.Model(&models.Serverpool{}).
		Where("id = ?", ServerpoolID).
		UpdateColumn("pendingnfs", gorm.Expr("NOT pendingnfs"))

	if result.Error != nil {
		log.Println("Error: ", result.Error)
	}
}
