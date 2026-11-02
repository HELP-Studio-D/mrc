package mrc

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestNewAuthManager(t *testing.T) {
	tests := []struct {
		name   string
		change func(*AuthConfig)
	}{
		{
			name: "short signing key",
			change: func(cfg *AuthConfig) {
				cfg.JWT.SigningKey = []byte("short")
			},
		},
		{
			name: "zero access ttl",
			change: func(cfg *AuthConfig) {
				cfg.JWT.AccessTTL = 0
			},
		},
		{
			name: "negative access ttl",
			change: func(cfg *AuthConfig) {
				cfg.JWT.AccessTTL = -time.Minute
			},
		},
		{
			name: "negative leeway",
			change: func(cfg *AuthConfig) {
				cfg.JWT.Leeway = -time.Second
			},
		},
		{
			name: "invalid bcrypt cost",
			change: func(cfg *AuthConfig) {
				cfg.Password.BcryptCost = bcrypt.MaxCost + 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testAuthConfig()
			tt.change(&cfg)

			manager, err := NewAuthManager(cfg)

			if manager != nil {
				t.Fatal("expected nil manager")
			}

			if !errors.Is(err, ErrInvalidAuthConfig) {
				t.Fatalf(
					"expected ErrInvalidAuthConfig, got %v",
					err,
				)
			}
		})
	}
}

func TestNewAuthManagerDefaultBcryptCost(t *testing.T) {
	cfg := testAuthConfig()
	cfg.Password.BcryptCost = 0

	manager, err := NewAuthManager(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manager.bcryptCost != bcrypt.DefaultCost {
		t.Fatalf(
			"expected bcrypt cost %d, got %d",
			bcrypt.DefaultCost,
			manager.bcryptCost,
		)
	}
}

func TestConfigureAuth(t *testing.T) {
	engine := &Engine{}

	err := engine.ConfigureAuth(testAuthConfig())
	if err != nil {
		t.Fatalf("ConfigureAuth() error = %v", err)
	}

	if engine.Auth == nil {
		t.Fatal("engine.Auth is nil")
	}
}

func TestConfigureAuthRejectsStartedEngine(t *testing.T) {
	engine := &Engine{
		started: true,
	}

	err := engine.ConfigureAuth(testAuthConfig())

	if !errors.Is(err, ErrServerStarted) {
		t.Fatalf(
			"expected ErrServerStarted, got %v",
			err,
		)
	}
}

func TestConfigureAuthRejectsClosedEngine(t *testing.T) {
	engine := &Engine{
		closed: true,
	}

	err := engine.ConfigureAuth(testAuthConfig())

	if !errors.Is(err, ErrEngineClosed) {
		t.Fatalf(
			"expected ErrEngineClosed, got %v",
			err,
		)
	}
}