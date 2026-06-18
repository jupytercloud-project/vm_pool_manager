package models

import "time"

// AuditLog : trace d'une action sensible (qui, quoi, quand, depuis où).
// Alimenté automatiquement par le middleware HTTP sur les requêtes mutantes.
type AuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	Actor     string    `json:"actor" gorm:"index"` // email / login de l'auteur
	Role      string    `json:"role"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	IP        string    `json:"ip"`
}
