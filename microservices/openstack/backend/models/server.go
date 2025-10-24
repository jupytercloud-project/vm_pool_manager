package models

import (
	"PoolManagerVM/backend/websockethandler"
	"fmt"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"gorm.io/gorm"
)

type Server struct {
	ID             string `gorm:"primaryKey"`
	Name           string
	Status         string
	FlavorRef      string
	ImageRef       string
	Networks       JSONStringSlice `gorm:"type:text"`
	Metadata       JSONStringMap   `gorm:"type:text"`
	ServerpoolID   string
	UserID         string
	ServerPool     *Serverpool `gorm:"foreignKey:ServerpoolID,UserID;references:ServerpoolID,UserID"`
	AttachVolumeID string
	VolPending     bool `gorm:"default:false; not null"`
	Reattrib       bool `gorm:"default:false; not null"`
	Progress       int  `gorm:"default:0; not null"`
	ConfigID       int
}

func FromGopherServer(s servers.Server) Server {
	var networks []string
	for netName, netAddrs := range s.Addresses {
		for _, addr := range netAddrs.([]any) {
			if addrMap, ok := addr.(map[string]any); ok {
				if ip, ok := addrMap["addr"].(string); ok {
					networks = append(networks, fmt.Sprintf("%s:%s", netName, ip))
				}
			}
		}
	}

	metadata := make(map[string]string)
	for k, v := range s.Metadata {
		metadata[k] = v
	}

	return Server{
		ID:           s.ID,
		Name:         s.Name,
		Status:       s.Status,
		FlavorRef:    s.Flavor["id"].(string),
		ImageRef:     s.Image["id"].(string),
		Networks:     networks,
		Metadata:     metadata,
		ServerpoolID: s.Metadata["serverpool_id"],
		UserID:       s.Metadata["user_id"],
	}
}

func PrintServer(server Server) error {

	// Afficher les infos du Server
	fmt.Println("=== Server Data ===")
	fmt.Printf("ID: %s\n", server.ID)
	fmt.Printf("Name: %s\n", server.Name)
	fmt.Printf("Status: %s\n", server.Status)
	fmt.Printf("FlavorRef: %s\n", server.FlavorRef)
	fmt.Printf("ImageRef: %s\n", server.ImageRef)
	fmt.Printf("Networks: %+v\n", server.Networks)
	fmt.Printf("Metadata: %+v\n", server.Metadata)
	fmt.Printf("ServerpoolID: %s\n", server.ServerpoolID)
	fmt.Printf("UserID: %s\n", server.UserID)

	// Si la relation ServerPool est chargée
	if server.ServerPool != nil {
		PrintServerpool(*server.ServerPool)
	}

	return nil
}

func (s *Server) AfterCreate(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		websockethandler.SendMessageToUser(s.UserID, "created", s, "server")
	}
	return nil
}

func (s *Server) AfterUpdate(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		websockethandler.SendMessageToUser(s.UserID, "updated", s, "server")
	}
	return nil
}

func (s *Server) AfterDelete(tx *gorm.DB) (err error) {
	if s.UserID != "admin" {
		websockethandler.SendMessageToUser(s.UserID, "deleted", s, "server")
	}
	return nil
}

func (s *Server) IsEqual(other Server) bool {
	if s.ID != other.ID ||
		s.Name != other.Name ||
		s.Status != other.Status ||
		s.FlavorRef != other.FlavorRef ||
		s.ImageRef != other.ImageRef ||
		s.ServerpoolID != other.ServerpoolID ||
		s.UserID != other.UserID {
		return false
	}

	if len(s.Networks) != len(other.Networks) {
		return false
	}
	for i, v := range s.Networks {
		if v != other.Networks[i] {
			return false
		}
	}

	if len(s.Metadata) != len(other.Metadata) {
		return false
	}
	for k, v := range s.Metadata {
		if otherVal, ok := other.Metadata[k]; !ok || v != otherVal {
			return false
		}
	}

	return true
}
