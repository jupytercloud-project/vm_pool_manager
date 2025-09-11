package models

// type ServerPool struct {
// 	ID           uint   `gorm:"primaryKey:autoIncrement"`
// 	ServerpoolID string `gorm:"index:idx_pool_user, unique"`
// 	UserID       string `gorm:"index:idx_pool_user, unique"`
// 	PendingJobs  int
// 	MinVM        int
// 	MaxVM        int
// 	ListServ     []Server `gorm:"foreignKey:PoolID"`
// }

// a tester
type Serverpool struct {
	ServerpoolID string   `gorm:"primaryKey"`
	UserID       string   `gorm:"primaryKey"`
	Params       []Param  `gorm:"foreignKey:ServerpoolID,UserID;references:ServerpoolID,UserID"` // relation has-many
	ListServ     []Server `gorm:"foreignKey:ServerpoolID,UserID;references:ServerpoolID,UserID"`
}

type Param struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	ServerpoolID string // clé étrangère
	UserID       string // clé étrangère
	ImageRef     string
	FlavorRef    string
	Networks     JSONStringSlice `gorm:"type:text"`
	MinVM        int
	MaxVM        int
	PendingJobs  int
}
