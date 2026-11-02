package mrc

import "testing"

func TestNewClaims(t *testing.T) {
	claims := NewClaims("user-123")

	if claims.Subject != "user-123" {
		t.Fatalf(
			"expected subject user-123, got %s",
			claims.Subject,
		)
	}

	if claims.Data == nil {
		t.Fatal("claims.Data should be initialized")
	}
}

func TestClaimsRoles(t *testing.T) {
	claims := NewClaims("user-123")

	claims.AddRole("admin")
	claims.AddRole("user")
	claims.AddRole("admin")

	if !claims.HasRole("admin") {
		t.Fatal("expected admin role")
	}

	if claims.HasRole("moderator") {
		t.Fatal("unexpected moderator role")
	}

	if !claims.HasAnyRole("guest", "admin") {
		t.Fatal("expected HasAnyRole to return true")
	}

	if !claims.HasAllRoles("admin", "user") {
		t.Fatal("expected HasAllRoles to return true")
	}

	if claims.HasAllRoles("admin", "moderator") {
		t.Fatal("expected HasAllRoles to return false")
	}

	claims.RemoveRole("admin")

	if claims.HasRole("admin") {
		t.Fatal("admin role was not removed")
	}
}

func TestClaimsPermissions(t *testing.T) {
	claims := NewClaims("user-123")

	claims.AddPermission("users:read")
	claims.AddPermission("users:write")

	if !claims.HasPermission("users:read") {
		t.Fatal("expected users:read permission")
	}

	if !claims.HasAnyPermission(
		"users:delete",
		"users:read",
	) {
		t.Fatal("expected HasAnyPermission to return true")
	}

	if !claims.HasAllPermissions(
		"users:read",
		"users:write",
	) {
		t.Fatal("expected HasAllPermissions to return true")
	}

	if claims.HasAllPermissions(
		"users:read",
		"users:delete",
	) {
		t.Fatal("expected HasAllPermissions to return false")
	}

	claims.RemovePermission("users:write")

	if claims.HasPermission("users:write") {
		t.Fatal("permission was not removed")
	}
}

func TestClaimsCustomData(t *testing.T) {
	claims := NewClaims("user-123")

	claims.Set("tenant_id", "tenant-456")
	claims.Set("verified", true)

	tenantID, exists := claims.Value("tenant_id")
	if !exists {
		t.Fatal("expected tenant_id")
	}

	if tenantID != "tenant-456" {
		t.Fatalf("unexpected tenant_id: %v", tenantID)
	}

	verified, exists := claims.Value("verified")
	if !exists || verified != true {
		t.Fatal("expected verified to be true")
	}

	if _, exists := claims.Value("missing"); exists {
		t.Fatal("unexpected missing value")
	}
}

func TestContextClaims(t *testing.T) {
	ctx := NewContext(nil, nil, nil)

	claims := NewClaims("user-123")
	claims.AddRole("admin")

	ctx.SetClaims(claims)

	got, exists := ctx.Claims()

	if !exists {
		t.Fatal("expected claims to exist")
	}

	if got.Subject != claims.Subject {
		t.Fatalf(
			"expected subject %s, got %s",
			claims.Subject,
			got.Subject,
		)
	}

	if !got.HasRole("admin") {
		t.Fatal("expected admin role")
	}

	ctx.ClearClaims()

	if _, exists := ctx.Claims(); exists {
		t.Fatal("claims should have been cleared")
	}
}

func TestContextMustClaimsPanics(t *testing.T) {
	ctx := NewContext(nil, nil, nil)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	ctx.MustClaims()
}