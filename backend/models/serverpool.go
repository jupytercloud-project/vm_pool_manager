package models

type ServerPool struct {
	ID           uint   `gorm:"primaryKey:autoIncrement"`
	ServerpoolID string `gorm:"index:idx_pool_user, unique"`
	UserID       string `gorm:"index:idx_pool_user, unique"`
	PendingJobs  int
	MinVM        int
	MaxVM        int
	ListServ     []Server `gorm:"foreignKey:PoolID"`
}

// a tester
type Serverpoolv2 struct {
	ID           uint
	ServerpoolID string
	UserID       string
	Params       []Param
	ListServ     []Server
}

type Param struct {
	ImageRef    string
	FlavorRef   string
	Networks    []string
	MinVM       int
	MaxVM       int
	PendingJobs int
}
