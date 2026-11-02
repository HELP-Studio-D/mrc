package mrc

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAuthNotConfigured = errors.New(
		"mrc: authentication is not configured",
	)

	ErrInvalidAuthConfig = errors.New(
		"mrc: invalid authentication configuration",
	)

	ErrInvalidClaims = errors.New(
		"mrc: invalid token claims",
	)

	ErrInvalidToken = errors.New(
		"mrc: invalid token",
	)

	ErrTokenExpired = errors.New(
		"mrc: token expired",
	)

	ErrTokenMissing = errors.New(
		"mrc: token is missing",
	)

	ErrEmptyPassword = errors.New(
		"mrc: password cannot be empty",
	)

	ErrPasswordTooLong = errors.New(
		"mrc: password exceeds bcrypt 72-byte limit",
	)

	ErrEngineClosed = errors.New(
		"mrc: engine is closed",
	)
)

// AuthConfig configures authentication.
type AuthConfig struct {
	JWT      JWTConfig
	Password PasswordConfig
}

// JWTConfig configures JWT generation and verification.
type JWTConfig struct {
	// HS256 signing secret. Must contain at least 32 bytes.
	SigningKey []byte

	// Expected issuer.
	Issuer string

	// Expected audiences.
	Audience []string

	// Access token lifetime.
	AccessTTL time.Duration

	// Allowed clock difference.
	Leeway time.Duration
}

// PasswordConfig configures password hashing.
type PasswordConfig struct {
	BcryptCost int
}

// AuthManager handles JWT and password operations.
type AuthManager struct {
	signingKey []byte

	issuer   string
	audience []string

	accessTTL time.Duration
	leeway    time.Duration

	bcryptCost int
}

// NewAuthManager creates an authentication manager.
func NewAuthManager(cfg AuthConfig) (*AuthManager, error) {
	if len(cfg.JWT.SigningKey) < 32 {
		return nil, fmt.Errorf(
			"%w: JWT signing key must be at least 32 bytes",
			ErrInvalidAuthConfig,
		)
	}

	if cfg.JWT.AccessTTL <= 0 {
		return nil, fmt.Errorf(
			"%w: AccessTTL must be positive",
			ErrInvalidAuthConfig,
		)
	}

	if cfg.JWT.Leeway < 0 {
		return nil, fmt.Errorf(
			"%w: Leeway cannot be negative",
			ErrInvalidAuthConfig,
		)
	}

	cost := cfg.Password.BcryptCost

	if cost == 0 {
		cost = bcrypt.DefaultCost
	}

	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf(
			"%w: invalid bcrypt cost",
			ErrInvalidAuthConfig,
		)
	}

	return &AuthManager{
		signingKey: append([]byte(nil), cfg.JWT.SigningKey...),
		issuer:     cfg.JWT.Issuer,
		audience:   append([]string(nil), cfg.JWT.Audience...),
		accessTTL:  cfg.JWT.AccessTTL,
		leeway:     cfg.JWT.Leeway,
		bcryptCost: cost,
	}, nil
}

// ConfigureAuth configures authentication on Engine.
// Call this before Start or Run.
func (e *Engine) ConfigureAuth(cfg AuthConfig) error {
	if e == nil {
		return ErrAuthNotConfigured
	}

	manager, err := NewAuthManager(cfg)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	if e.started {
		return ErrServerStarted
	}

	e.Auth = manager

	return nil
}
