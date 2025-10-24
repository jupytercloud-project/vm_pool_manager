package models

import (
	"time"
)

type Image struct {
	ID               string    `gorm:"primaryKey" json:"id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	Tags             string    `json:"tags"` // Stocké en CSV
	ContainerFormat  string    `json:"container_format"`
	DiskFormat       string    `json:"disk_format"`
	MinDiskGigabytes int       `json:"min_disk"`
	MinRAMMegabytes  int       `json:"min_ram"`
	Owner            string    `json:"owner"`
	Protected        bool      `json:"protected"`
	Visibility       string    `json:"visibility"`
	Hidden           bool      `json:"os_hidden"`
	Checksum         string    `json:"checksum"`
	SizeBytes        int64     `json:"size_bytes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	File             string    `json:"file"`
	Schema           string    `json:"schema"`
	VirtualSize      int64     `json:"virtual_size"`
	ImportMethods    string    `json:"import_methods"` // CSV
	StoreIDs         string    `json:"store_ids"`      // CSV
	// Metadata et Properties exclus car SQLite ne stocke pas les maps
}

type Flavor struct {
	ID          string  `gorm:"primaryKey" json:"id"`
	Name        string  `json:"name"`
	Disk        int     `json:"disk"`
	RAM         int     `json:"ram"`
	VCPUs       int     `json:"vcpus"`
	RxTxFactor  float64 `json:"rxtx_factor"`
	Swap        int     `json:"swap"`
	Ephemeral   int     `json:"ephemeral"`
	IsPublic    bool    `json:"is_public"`
	Description string  `json:"description"`
	ExtraSpecs  string  `json:"extra_specs"` // JSON sérialisé
}

type Network struct {
	ID                    string `gorm:"primaryKey" json:"id"`
	Name                  string `json:"name"`
	Description           string `json:"description"`
	AdminStateUp          bool   `json:"admin_state_up"`
	Status                string `json:"status"`
	TenantID              string `json:"tenant_id"`
	ProjectID             string `json:"project_id"`
	Shared                bool   `json:"shared"`
	RevisionNumber        int    `json:"revision_number"`
	Subnets               string `json:"subnets"`                 // CSV
	AvailabilityZoneHints string `json:"availability_zone_hints"` // CSV
	Tags                  string `json:"tags"`                    // CSV
	// CreatedAt et UpdatedAt ignorés car json:"-"
}

type VolumeDB struct {
	ID                  string `gorm:"primaryKey"` // Même ID que OpenStack
	Status              string
	Size                int
	AvailabilityZone    string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Name                string
	Description         string
	VolumeType          string
	SnapshotID          string
	SourceVolID         string
	BackupID            *string
	Metadata            JSONStringMap `gorm:"type:json"` // map[string]string
	UserID              string
	Bootable            string
	Encrypted           bool
	ReplicationStatus   string
	ConsistencyGroupID  string
	Multiattach         bool
	VolumeImageMetadata JSONStringMap `gorm:"type:json"`
	Host                string
	TenantID            string
	Attachments         JSONAttachments `gorm:"type:json"`
}
