package auth

import "time"

type Config struct {
	JWT      JWTConfig
	Password PasswordConfig
}

type JWTConfig struct {
	SigningKey []byte
	Issuer     string
	AccessTTL  time.Duration
}

type PasswordConfig struct {
	Algorithm  string
	BcryptCost int
}
