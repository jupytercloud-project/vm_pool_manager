package models

import (
	"PoolManagerVM/backend/events"
	"PoolManagerVM/backend/notifier"
	"PoolManagerVM/backend/pb"
	"PoolManagerVM/backend/websockethandler"
	"fmt"

	"gorm.io/gorm"
)

type ConfigPool struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Data   string `json:"data"`
}

func (c *ConfigPool) ToMap() map[string]string {
	result := map[string]string{
		"id":      fmt.Sprintf("%d", c.ID),
		"user_id": c.UserID,
		"name":    c.Name,
		"data":    c.Data,
	}
	return result
}

func (c *ConfigPool) AfterCreate(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		websockethandler.SendMessageToUser(c.UserID, "created", c, "config")
		notifier.GlobalChan <- events.RessourceEvent{Action: "created", Type: pb.Type_CONFIG, Ressource: *c}
	}
	return nil
}

func (c *ConfigPool) AfterUpdate(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		websockethandler.SendMessageToUser(c.UserID, "updated", c, "config")
		notifier.GlobalChan <- events.RessourceEvent{Action: "updated", Type: pb.Type_CONFIG, Ressource: *c}
	}
	return nil
}

func (c *ConfigPool) AfterDelete(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		websockethandler.SendMessageToUser(c.UserID, "deleted", c, "config")
		notifier.GlobalChan <- events.RessourceEvent{Action: "deleted", Type: pb.Type_CONFIG, Ressource: *c}
	}
	return nil
}
