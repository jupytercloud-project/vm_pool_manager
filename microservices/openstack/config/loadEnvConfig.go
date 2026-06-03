package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// load .env
func LoadEnvConfig() {
	dotenvPath := os.Getenv("DOTENV_PATH")
	if dotenvPath == "" {
		dotenvPath = ".env"
	}
	err := godotenv.Load(dotenvPath)
	if err != nil {
		log.Fatalf("Error loading .env file from %s: %v", dotenvPath, err)
	}
}
