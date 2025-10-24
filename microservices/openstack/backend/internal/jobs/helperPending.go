package jobs

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"log"

	"gorm.io/gorm"
)

// IncrementPending increases the pending_jobs counter for the given Serverpool.
//
// It performs an atomic update in the database by incrementing the pending_jobs column by 1.
// This function is typically called when a new job (e.g., VM creation or deletion) is scheduled.
//
// Parameters:
//   - ServerpoolID: The ID of the Serverpool whose pending_jobs counter should be incremented.
func IncrementPending(ServerpoolID uint) {
	result := config.Database.Model(&models.Serverpool{}).
		Where("id = ?", ServerpoolID).
		UpdateColumn("pending_jobs", gorm.Expr("pending_jobs + ?", 1))

	if result.Error != nil {
		log.Println("Error: ", result.Error)
	}
}

// DecrementPending decreases the pending_jobs counter for the given Serverpool.
//
// It performs an atomic update in the database by decrementing the pending_jobs column by 1,
// but only if the current value is greater than zero (to prevent negative counters).
// This function is typically called when a job has completed.
//
// Parameters:
//   - ServerpoolID: The ID of the Serverpool whose pending_jobs counter should be decremented.
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
