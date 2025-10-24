package models

import (
	"PoolManagerVM/backend/websockethandler"

	"gorm.io/gorm"
)

type ConfigPool struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Data   string `json:"data"`
}

func (c *ConfigPool) AfterCreate(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		websockethandler.SendMessageToUser(c.UserID, "created", c, "config")
	}
	return nil
}

func (c *ConfigPool) AfterUpdate(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		websockethandler.SendMessageToUser(c.UserID, "updated", c, "config")
	}
	return nil
}

func (c *ConfigPool) AfterDelete(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		websockethandler.SendMessageToUser(c.UserID, "deleted", c, "config")
	}
	return nil
}
