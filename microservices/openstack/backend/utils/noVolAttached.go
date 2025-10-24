package utils

import (
	"PoolManagerVM/backend/models"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"gorm.io/gorm"
)

func NoVolAttached(server servers.Server) bool {
	return len(server.AttachedVolumes) == 0
}

func NoVolAttachedDB(server models.Server, DB *gorm.DB) bool {
	var servers []models.Server

	if err := DB.Find(&servers).Error; err != nil {
		return true
	}

	for _, s := range servers {
		if s.ID == server.ID {
			if s.AttachVolumeID != "" {
				return false
			} else {
				return true
			}
		}
	}
	return true
}
