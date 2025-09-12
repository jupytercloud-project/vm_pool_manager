package models

import (
	"fmt"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

// type Server struct {
// 	IDServer     string   `json:"IDServer" gorm:"primary_key;column:id_server"` // clé primaire de Server
// 	NomServer    string   `json:"NomServer"`                                    // nom du Server
// 	ImageServer  string   `json:"ImageServer"`                                  // image du Server
// 	FlavorServer string   `json:"FlavorServer"`                                 // flavor du Server
// 	IDServerPool uint     `json:"IDServerPool"`                                 // lien clé étrangère avec ServerPool
// 	IDMetadata   uint     `json:"IDMetadata"`                                   // lien clé étrangère avec Metadata
// 	IDUser       string   `json:"IDUser"`                                       // lien clé étrangère avec User
// 	IPAddress    []string `json:"IPAddress" gorm:"foreignKey:IDServer"`
// 	Networks     []string `json:"Networks" gorm:"many2many:ServerNetwork;"` // relation many2many avec Network
// 	// création d'une table de jointure ServerNetwork
// 	Keynames []string `json:"Keynames" gorm:"many2many:ServerKeyName;"` // relation many2many avec KeyName
// 	// création d'une table de jointure ServerKeyName
// 	SecurityGroups []string `json:"SecurityGroups" gorm:"many2many:ServerSecurityGroup;"` // relation many2many
// 	// avec SecurityGroup, création d'une table de jointure ServerSecurityGroup
// }

// type Server struct {
// 	ID         string `gorm:"primaryKey"`
// 	Name       string
// 	Status     string
// 	FlavorRef  string
// 	ImageRef   string
// 	Networks   []string
// 	PoolID     *uint
// 	Metadata   map[string]string
// 	ServerPool *ServerPool `gorm:"foreignKey:PoolID"`
// }

type Server struct {
	ID           string `gorm:"primaryKey"`
	Name         string
	Status       string
	FlavorRef    string
	ImageRef     string
	Networks     JSONStringSlice `gorm:"type:text"` // Slice stocké en JSON
	Metadata     JSONStringMap   `gorm:"type:text"` // Map stockée en JSON
	ServerpoolID string          // clé étrangère vers Serverpool
	UserID       string          // clé étrangère vers Serverpool
	ServerPool   *Serverpool     `gorm:"foreignKey:ServerpoolID,UserID;references:ServerpoolID,UserID"`
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
