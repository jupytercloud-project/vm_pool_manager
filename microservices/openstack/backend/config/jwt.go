package config

import "os"

// stock the secret JWT
var JWTSecret = []byte(os.Getenv("SECRET_KEY_JWT"))
