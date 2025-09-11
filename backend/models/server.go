package models

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
