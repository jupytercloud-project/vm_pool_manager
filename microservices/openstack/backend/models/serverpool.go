package models

import (
	"PoolManagerVM/backend/websockethandler"
	"fmt"
	"log"

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
	ListServ     []Server `gorm:"foreignKey:ServerpoolID,UserID;references:ServerpoolID,UserID"`
	ConfigID     int
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
	for _, s := range sp.ListServ {
		PrintServer(s)
	}

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
		websockethandler.SendMessageToUser(s.UserID, "created", s, "serverpool")
	}
	return nil
}

func (s *Serverpool) AfterUpdate(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		websockethandler.SendMessageToUser(s.UserID, "updated", s, "serverpool")
	}
	return nil
}

func (s *Serverpool) AfterDelete(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		log.Println("Sending delete message to user:", s.UserID, "for serverpool:", s.ServerpoolID)
		websockethandler.SendMessageToUser(s.UserID, "deleted", s, "serverpool")
	}
	return nil
}
