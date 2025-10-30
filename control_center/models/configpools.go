package models

import (
	"control_center/pb"
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

type ConfigPool struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Data   string `json:"data"`
}

func (c *ConfigPool) FromPb(pbs *pb.StreamRessourceResponse) error {
	data := pbs.Data
	if data == nil {
		return fmt.Errorf("empty data map in StreamRessourceResponse")
	}
	if v, ok := data["id"]; ok && v != "" {
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			c.ID = uint(id)
		}
	}
	c.UserID = data["user_id"]
	c.Name = data["name"]
	c.Data = data["data"]

	return nil
}

func (c *ConfigPool) AfterCreate(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		// websockethandler.SendMessageToUser(c.UserID, "created", c, "config")
	}
	return nil
}

func (c *ConfigPool) AfterUpdate(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		// websockethandler.SendMessageToUser(c.UserID, "updated", c, "config")
	}
	return nil
}

func (c *ConfigPool) AfterDelete(tx *gorm.DB) (err error) {
	if c.UserID != "admin" {
		// websockethandler.SendMessageToUser(c.UserID, "deleted", c, "config")
	}
	return nil
}
