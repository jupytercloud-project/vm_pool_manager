package config

import "os"

var JWTSecret = []byte(os.Getenv("SECRET_KEY_JWT"))
