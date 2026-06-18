package models

import (
	"control_center/frontcontrolpb"
	"control_center/pb"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
)

type Serverpool struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	ServerpoolID   string `gorm:"uniqueIndex:idx_pool_user"`
	UserID         string `gorm:"uniqueIndex:idx_pool_user"`
	ImageRef       string
	FlavorRef      string
	Networks       JSONStringSlice `gorm:"type:text"`
	MinVM          int
	MaxVM          int
	PendingJobs    int
	ConfigID       string
	Timewindow     *time.Duration `gorm:"type:bigint"`
	TimeStart      *time.Time     `gorm:"type:timestamptz"`
	Keypublist     pq.StringArray `gorm:"type:text[]"`
	ListStudents   ListStudents   `gorm:"foreignKey:PoolId;constraint:OnDelete:CASCADE"`
	Keypubuser     string
	Status         string
	OffDays        string `gorm:"type:text"`
	AppPort        int    `gorm:"default:0"`
	Role           string `gorm:"default:'student'"` // "student" or "instructor"
	LinkedPoolID   string `gorm:"default:''"`        // pool étudiant associé (si role=instructor)
	MoodleCourseID int    `gorm:"default:0"`         // cours Moodle lié (0 = aucun), renseigné à l'import
	XCourseCode    string `gorm:"default:''"`        // cours de l'X lié (shortname, ex. CSC_41M03_EP-2025), renseigné à l'import
	Label          string `gorm:"default:''"`        // nom d'affichage facultatif (sinon ServerpoolID)
	Tags           string `gorm:"default:''"`        // étiquettes libres (CSV) pour organiser/filtrer
}

func (sp *Serverpool) FromPb(pbs *pb.StreamRessourceResponse) error {
	data := pbs.Data
	if data == nil {
		return fmt.Errorf("empty data map in StreamRessourceResponse")
	}

	if v, ok := data["id"]; ok && v != "" {
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			sp.ID = uint(id)
		} else {
			return fmt.Errorf("invalid id value: %v", err)
		}
	}

	sp.ServerpoolID = data["serverpool_id"]
	sp.UserID = data["user_id"]
	sp.ImageRef = data["image_ref"]
	sp.FlavorRef = data["flavor_ref"]

	if v, ok := data["networks"]; ok && v != "" {
		var networks []string
		if err := json.Unmarshal([]byte(v), &networks); err != nil {
			return fmt.Errorf("error unmarshaling networks: %v", err)
		}
		sp.Networks = networks
	}

	if v, ok := data["min_vm"]; ok && v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			sp.MinVM = val
		}
	}
	if v, ok := data["max_vm"]; ok && v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			sp.MaxVM = val
		}
	}
	if v, ok := data["pending_jobs"]; ok && v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			sp.PendingJobs = val
		}
	}
	sp.ConfigID = data["config_id"]

	return nil
}

func (sp *Serverpool) ToMap() map[string]string {
	result := map[string]string{
		"id":            fmt.Sprintf("%d", sp.ID),
		"serverpool_id": sp.ServerpoolID,
		"user_id":       sp.UserID,
		"image_ref":     sp.ImageRef,
		"flavor_ref":    sp.FlavorRef,
		"min_vm":        fmt.Sprintf("%d", sp.MinVM),
		"max_vm":        fmt.Sprintf("%d", sp.MaxVM),
		"pending_jobs":  fmt.Sprintf("%d", sp.PendingJobs),
		"config_id":     sp.ConfigID,
	}

	if sp.Networks != nil {
		if b, err := json.Marshal(sp.Networks); err == nil {
			result["networks"] = string(b)
		}
	}
	if sp.TimeStart != nil {
		result["timestart"] = sp.TimeStart.Format(time.RFC3339)
	}
	if sp.OffDays != "" {
		result["off_days"] = sp.OffDays
	}
	result["host"] = "OpenStack"
	return result
}

func (sp *Serverpool) ToFrontControlPb() *frontcontrolpb.ServerPool {
	var network string
	if len(sp.Networks) > 0 {
		network = sp.Networks[0]
	}

	return &frontcontrolpb.ServerPool{
		Id:       strconv.FormatUint(uint64(sp.ID), 10),
		Name:     sp.ServerpoolID,
		Image:    sp.ImageRef,
		Flavor:   sp.FlavorRef,
		Network:  network,
		Config:   sp.ConfigID,
		MinVm:    int32(sp.MinVM),
		MaxVm:    int32(sp.MaxVM),
		Metadata: map[string]string{},
		UserId:   sp.UserID,
		AppPort:  int32(sp.AppPort),
	}
}
