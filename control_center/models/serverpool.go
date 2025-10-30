package models

import (
	"control_center/pb"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"gorm.io/gorm"
)

type Serverpool struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	ServerpoolID string `gorm:"index:idx_pool_user,unique"`
	UserID       string `gorm:"index:idx_pool_user,unique"`
	ImageRef     string
	FlavorRef    string
	Networks     JSONStringSlice `gorm:"type:text"`
	MinVM        int
	MaxVM        int
	PendingJobs  int
	ConfigID     int
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
	if v, ok := data["config_id"]; ok && v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			sp.ConfigID = val
		}
	}

	return nil
}

func PrintServerpool(sp Serverpool) error {
	fmt.Println("=== Serverpool Data ===")
	fmt.Println("ID: ", sp.ID)
	fmt.Println("ServerpoolID: ", sp.ServerpoolID)
	fmt.Println("UserID: ", sp.UserID)
	fmt.Println("ImageRef: ", sp.ImageRef)
	fmt.Println("FlavorRef: ", sp.FlavorRef)
	fmt.Println("Networks: ", sp.Networks)
	fmt.Println("MinVM: ", sp.MinVM)
	fmt.Println("MaxVm: ", sp.MaxVM)
	fmt.Println("PendingJobs: ", sp.PendingJobs)
	fmt.Println("ConfigID: ", sp.ConfigID)
	// for _, s := range sp.ListServ {
	// 	PrintServer(s)
	// }

	return nil
}

func PrintMapServerpool(m []Serverpool) error {
	fmt.Println("=== Print Map Serverpool ===")
	for _, p := range m {
		PrintServerpool(p)
		fmt.Println("=====================================")
	}
	return nil
}

func (s *Serverpool) AfterCreate(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		// websockethandler.SendMessageToUser(s.UserID, "created", s, "serverpool")
	}
	return nil
}

func (s *Serverpool) AfterUpdate(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		// websockethandler.SendMessageToUser(s.UserID, "updated", s, "serverpool")
	}
	return nil
}

func (s *Serverpool) AfterDelete(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		log.Println("Sending delete message to user:", s.UserID, "for serverpool:", s.ServerpoolID)
		// websockethandler.SendMessageToUser(s.UserID, "deleted", s, "serverpool")
	}
	return nil
}
