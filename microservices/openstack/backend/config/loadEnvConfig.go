package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// load .env
func LoadEnvConfig() {
	err := godotenv.Load(os.Getenv("DOTENV_PATH"))
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}
