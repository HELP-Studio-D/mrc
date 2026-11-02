package mrc

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
	manager := newTestAuthManager(t)

	claims := NewClaims("user-123")
	claims.AddRole("admin")
	claims.AddPermission("users:read")
	claims.Set("tenant_id", "tenant-456")

	token, err := manager.GenerateToken(claims)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("generated token is empty")
	}

	parsed, err := manager.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}

	if parsed.Subject != "user-123" {
		t.Fatalf("unexpected subject: %s", parsed.Subject)
	}

	if !parsed.HasRole("admin") {
		t.Fatal("admin role was not preserved")
	}

	if !parsed.HasPermission("users:read") {
		t.Fatal("permission was not preserved")
	}

	tenantID, exists := parsed.Value("tenant_id")
	if !exists || tenantID != "tenant-456" {
		t.Fatal("custom claim was not preserved")
	}
}

func TestGenerateTokenRejectsInvalidClaims(t *testing.T) {
	manager := newTestAuthManager(t)

	tests := []struct {
		name   string
		claims *Claims
	}{
		{
			name:   "nil claims",
			claims: nil,
		},
		{
			name:   "empty subject",
			claims: NewClaims(""),
		},
		{
			name:   "whitespace subject",
			claims: NewClaims("   "),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := manager.GenerateToken(tt.claims)

			if err == nil {
				t.Fatal("expected error")
			}

			if token != "" {
				t.Fatal("expected empty token")
			}
		})
	}
}

func TestParseTokenRejectsInvalidSignature(t *testing.T) {
	manager := newTestAuthManager(t)

	token, err := manager.GenerateToken(
		NewClaims("user-123"),
	)
	if err != nil {
		t.Fatal(err)
	}

	cfg := testAuthConfig()
	cfg.JWT.SigningKey = []byte(
		"another-secret-key-0123456789abcdef",
	)

	anotherManager, err := NewAuthManager(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = anotherManager.ParseToken(token)

	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf(
			"expected ErrInvalidToken, got %v",
			err,
		)
	}
}

// signTestJWT creates JWTs for negative test cases.
func signTestJWT(
	t *testing.T,
	manager *AuthManager,
	method jwt.SigningMethod,
	claims jwt.Claims,
) string {
	t.Helper()

	token := jwt.NewWithClaims(method, claims)

	signed, err := token.SignedString(manager.signingKey)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	return signed
}

func TestParseTokenRejectsExpiredToken(t *testing.T) {
	manager := newTestAuthManager(t)

	now := time.Now().UTC()

	expiredClaims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   manager.issuer,
			Subject:  "user-123",
			Audience: jwt.ClaimStrings{"mrc-tests"},

			IssuedAt: jwt.NewNumericDate(
				now.Add(-2 * time.Hour),
			),

			ExpiresAt: jwt.NewNumericDate(
				now.Add(-time.Hour),
			),
		},
	}

	token := signTestJWT(
		t,
		manager,
		jwt.SigningMethodHS256,
		expiredClaims,
	)

	_, err := manager.ParseToken(token)

	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf(
			"expected ErrTokenExpired, got %v",
			err,
		)
	}
}

func TestParseTokenRequiresExpiration(t *testing.T) {
	manager := newTestAuthManager(t)

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   manager.issuer,
			Subject:  "user-123",
			Audience: jwt.ClaimStrings{"mrc-tests"},
			IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
		},
	}

	token := signTestJWT(
		t,
		manager,
		jwt.SigningMethodHS256,
		claims,
	)

	_, err := manager.ParseToken(token)

	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf(
			"expected ErrInvalidToken, got %v",
			err,
		)
	}
}

func TestParseTokenRejectsWrongAlgorithm(t *testing.T) {
	manager := newTestAuthManager(t)

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    manager.issuer,
			Subject:   "user-123",
			Audience:  jwt.ClaimStrings{"mrc-tests"},
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token := signTestJWT(
		t,
		manager,
		jwt.SigningMethodHS384,
		claims,
	)

	_, err := manager.ParseToken(token)

	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf(
			"expected ErrInvalidToken, got %v",
			err,
		)
	}
}

func TestParseTokenRejectsMissingSubject(t *testing.T) {
	manager := newTestAuthManager(t)

	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    manager.issuer,
			Audience:  jwt.ClaimStrings{"mrc-tests"},
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	token := signTestJWT(
		t,
		manager,
		jwt.SigningMethodHS256,
		claims,
	)

	_, err := manager.ParseToken(token)

	if !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf(
			"expected ErrInvalidClaims, got %v",
			err,
		)
	}
}

func TestParseTokenRejectsMalformedToken(t *testing.T) {
	manager := newTestAuthManager(t)

	_, err := manager.ParseToken("not-a-jwt")

	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf(
			"expected ErrInvalidToken, got %v",
			err,
		)
	}
}