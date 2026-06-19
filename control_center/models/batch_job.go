package models

import "time"

// BatchJob : tâche de calcul soumise par un utilisateur (script exécuté sur une VM
// d'un pool « calcul » via SSH, sortie collectée). Phase 4 — B1 (jobs batch) + B4
// (auto-arrêt : la VM est suspendue en fin de job).
//
// Statuts : queued → running → succeeded | failed | canceled.
type BatchJob struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	OwnerEmail string     `json:"owner_email" gorm:"index"`
	Name       string     `json:"name"`
	PoolID     string     `json:"pool_id"` // pool « calcul » cible
	Script     string     `json:"script"`  // script bash exécuté sur la VM
	Status     string     `json:"status" gorm:"index"`
	ExitCode   int        `json:"exit_code"`
	Log        string     `json:"log"`     // stdout+stderr (tronqué)
	VMName     string     `json:"vm_name"` // VM sur laquelle le job a tourné
	AutoStop   bool       `json:"auto_stop" gorm:"default:true"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}
