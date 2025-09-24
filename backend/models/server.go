package models

import (
	"fmt"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
)

type Server struct {
	ID           string `gorm:"primaryKey"`
	Name         string
	Status       string
	FlavorRef    string
	ImageRef     string
	Networks     JSONStringSlice `gorm:"type:text"`
	Metadata     JSONStringMap   `gorm:"type:text"`
	ServerpoolID string
	UserID       string
	// Relation avec Serverpool (clé composite)
	ServerPool *Serverpool `gorm:"foreignKey:ServerpoolID,UserID;references:ServerpoolID,UserID"`
}

func FromGopherServer(s servers.Server) Server {
	var networks []string
	for netName, netAddrs := range s.Addresses {
		for _, addr := range netAddrs.([]interface{}) {
			if addrMap, ok := addr.(map[string]interface{}); ok {
				if ip, ok := addrMap["addr"].(string); ok {
					networks = append(networks, fmt.Sprintf("%s:%s", netName, ip))
				}
			}
		}
	}

	// Metadata est déjà une map[string]string
	metadata := make(map[string]string)
	for k, v := range s.Metadata {
		metadata[k] = v
	}

	return Server{
		ID:           s.ID,
		Name:         s.Name,
		Status:       s.Status,
		FlavorRef:    s.Flavor["id"].(string), // Flavor est une map
		ImageRef:     s.Image["id"].(string),  // Image aussi
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
