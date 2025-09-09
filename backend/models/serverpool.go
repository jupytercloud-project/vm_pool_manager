package models

type ServerPool struct {
	ID          string `gorm:"primaryKey"`
	PendingJobs int
	minVM       int
}
