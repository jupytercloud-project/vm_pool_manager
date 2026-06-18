package models

import (
	"encoding/json"
	"time"
)

// VMInstance is populated by vm-registrar heartbeats (see sql/registrar_schema.sql).
type VMInstance struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name"`
	IP             string    `json:"ip"`
	PublicIP       string    `json:"public_ip" gorm:"column:public_ip"`
	AZ             string    `json:"az"`
	Role           string    `json:"role"`
	AppPort        int       `json:"app_port" gorm:"column:app_port"`
	Environment    string    `json:"environment"`
	Status         string    `json:"status"`
	Healthy        bool      `json:"healthy"`
	ActivityStatus string    `json:"activity_status" gorm:"column:activity_status"`
	RegisteredAt   time.Time `json:"registered_at" gorm:"column:registered_at"`
	LastSeen       time.Time `json:"last_seen" gorm:"column:last_seen"`
	// LastActive : dernier instant où un utilisateur était réellement connecté (SSH).
	// Contrairement à LastSeen (rafraîchi à chaque sonde), il ne bouge QUE sur activité,
	// ce qui permet de mesurer la durée d'inactivité pour l'auto-suspend.
	LastActive       time.Time       `json:"last_active" gorm:"column:last_active"`
	RawMeta          json.RawMessage `json:"raw_meta" gorm:"column:raw_meta;type:jsonb"`
	GuacConnectionID string          `json:"guac_connection_id" gorm:"column:guac_connection_id;default:''"`
}

func (VMInstance) TableName() string {
	return "vm_instances"
}
