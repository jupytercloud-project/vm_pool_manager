package models

import "time"

type GitHubSession struct {
	ID        string    `gorm:"primaryKey"`
	Login     string    `gorm:"not null"`
	SSHKeys   string    `gorm:"type:text"` // JSON array of key strings
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
