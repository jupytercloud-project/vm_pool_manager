package jobs

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"

	"gorm.io/gorm"
)

func IncrementPending(poolID string) error {
	return config.Database.Transaction(func(tx *gorm.DB) error {
		var pool models.ServerPool
		if err := tx.FirstOrCreate(&pool, models.ServerPool{ID: poolID}).Error; err != nil {
			return err
		}
		return tx.Model(&pool).Update("pending_jobs", pool.PendingJobs+1).Error
	})
}

func DecrementPending(poolID string) error {
	return config.Database.Transaction(func(tx *gorm.DB) error {
		var pool models.ServerPool
		if err := tx.First(&pool, "id = ?", poolID).Error; err != nil {
			return err
		}
		if pool.PendingJobs > 0 {
			return tx.Model(&pool).Update("pending_jobs", pool.PendingJobs-1).Error
		}
		return nil
	})
}
