package models

import "fmt"

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
	ID           uint            `gorm:"primaryKey;autoIncrement"`
	ServerpoolID string          `gorm:"uniqueIndex:idx_param_unique"`
	UserID       string          `gorm:"uniqueIndex:idx_param_unique"`
	ImageRef     string          `gorm:"uniqueIndex:idx_param_unique"`
	FlavorRef    string          `gorm:"uniqueIndex:idx_param_unique"`
	Networks     JSONStringSlice `gorm:"type:text"`
	MinVM        int
	MaxVM        int
	PendingJobs  int
}

func PrintServerpool(sp Serverpool) error {
	fmt.Println("=== Serverpool Data ===")
	fmt.Println("ServerpoolID: ", sp.ServerpoolID)
	fmt.Println("UserID: ", sp.UserID)

	for _, p := range sp.Params {
		PrintParam(p)
	}
	for _, s := range sp.ListServ {
		PrintServer(s)
	}

	return nil
}

func PrintParam(param Param) error {

	// Afficher les infos du Server
	fmt.Println("=== Param Data ===")
	fmt.Println("ID: ", param.ID)
	fmt.Println("ServerpoolID: ", param.ServerpoolID)
	fmt.Println("UserID: ", param.UserID)
	fmt.Println("ImageRef: ", param.ImageRef)
	fmt.Println("FlavorRef: ", param.FlavorRef)
	fmt.Println("MinVM: ", param.MinVM)
	fmt.Println("MaxVm: ", param.MaxVM)
	fmt.Println("PendingJobs: ", param.PendingJobs)
	fmt.Println("Networks: ", param.Networks)

	return nil
}

func PrintMapServerpool(m []Serverpool) error {
	fmt.Println("=== Print Map Serverpool ===")
	for _, p := range m {
		PrintServerpool(p)
		fmt.Println("=====================================")
	}
	return nil
}
