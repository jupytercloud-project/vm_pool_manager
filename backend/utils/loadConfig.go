package utils

import (
	"PoolManagerVM/backend/models"

	"github.com/BurntSushi/toml"
)

func LoadConfig(path string) (*models.Config, error) {
	var cfg models.Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func CreateNewConfig() {

}
