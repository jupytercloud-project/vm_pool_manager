package jobs

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"

	"gorm.io/gorm"
)

func IncrementPending(poolID, userID string) error {
	return config.Database.Transaction(func(tx *gorm.DB) error {
		var pool models.ServerPool
		if err := tx.Where(&models.ServerPool{
			ServerpoolID: poolID,
			UserID:       userID,
		}).FirstOrCreate(&pool).Error; err != nil {
			return err
		}
		return tx.Model(&pool).Update("pending_jobs", pool.PendingJobs+1).Error
	})
}

func DecrementPending(poolID, userID string) error {
	return config.Database.Transaction(func(tx *gorm.DB) error {
		var pool models.ServerPool
		if err := tx.Where(&models.ServerPool{
			ServerpoolID: poolID,
			UserID:       userID,
		}).FirstOrCreate(&pool).Error; err != nil {
			return err
		}
		if pool.PendingJobs > 0 {
			return tx.Model(&pool).Update("pending_jobs", pool.PendingJobs-1).Error
		}
		return nil
	})
}
