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
	Host   string `json:"host"`
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
	c.Host = data["host"]
	return nil
}

func (c *ConfigPool) ToMap() map[string]string {
	result := map[string]string{
		"id":      fmt.Sprintf("%d", c.ID),
		"user_id": c.UserID,
		"name":    c.Name,
		"data":    c.Data,
	}
	result["host"] = "OpenStack"
	return result
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
