package config

import (
	"log"

	"github.com/joho/godotenv"
)

func LoadEnvConfig() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}
