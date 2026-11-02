package mrc

import (
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func testAuthConfig() AuthConfig {
	return AuthConfig{
		JWT: JWTConfig{
			SigningKey: []byte(
				"0123456789abcdef0123456789abcdef",
			),
			Issuer:   "mrc-test",
			Audience: []string{"mrc-tests"},
			AccessTTL:  time.Hour,
			Leeway:     0,
		},
		Password: PasswordConfig{
			BcryptCost: bcrypt.MinCost,
		},
	}
}

func newTestAuthManager(t *testing.T) *AuthManager {
	t.Helper()

	manager, err := NewAuthManager(testAuthConfig())
	if err != nil {
		t.Fatalf("NewAuthManager() error = %v", err)
	}

	return manager
}