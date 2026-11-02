package mrc

import (
	"encoding/json"
	"errors"
	"testing"
)

type authTestWriter struct {
	status  Status
	payload []byte
	options map[string]string
}

func (w *authTestWriter) SetStatus(status Status) error {
	w.status = status
	return nil
}

func (w *authTestWriter) SetPayload(payload []byte) error {
	w.payload = append([]byte(nil), payload...)
	return nil
}

func (w *authTestWriter) AddOption(key, value string) error {
	if w.options == nil {
		w.options = make(map[string]string)
	}

	if _, exists := w.options[key]; exists {
		return errors.New("duplicate response option")
	}

	w.options[key] = value

	return nil
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid bearer",
			input: "Bearer abc123",
			want:  "abc123",
		},
		{
			name:  "lowercase bearer",
			input: "bearer abc123",
			want:  "abc123",
		},
		{
			name:    "empty header",
			input:   "",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			input:   "Basic abc123",
			wantErr: true,
		},
		{
			name:    "missing token",
			input:   "Bearer",
			wantErr: true,
		},
		{
			name:    "too many fields",
			input:   "Bearer abc123 extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractBearerToken(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf(
					"expected %q, got %q",
					tt.want,
					got,
				)
			}
		})
	}
}

func TestAuthMiddlewareAcceptsValidToken(t *testing.T) {
	manager := newTestAuthManager(t)

	expected := NewClaims("user-123")
	expected.AddRole("admin")

	token, err := manager.GenerateToken(expected)
	if err != nil {
		t.Fatal(err)
	}

	middleware, err := manager.Middleware(
		func(c *Context) (string, error) {
			return token, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	writer := &authTestWriter{}
	ctx := NewContext(nil, writer, nil)

	middleware(ctx)

	if ctx.IsAborted() {
		t.Fatal("valid token was rejected")
	}

	actual, exists := ctx.Claims()
	if !exists {
		t.Fatal("claims were not saved in Context")
	}

	if actual.Subject != expected.Subject {
		t.Fatalf(
			"expected subject %s, got %s",
			expected.Subject,
			actual.Subject,
		)
	}

	if !actual.HasRole("admin") {
		t.Fatal("admin role was not preserved")
	}
}

func TestAuthMiddlewareRejectsInvalidToken(t *testing.T) {
	manager := newTestAuthManager(t)

	middleware, err := manager.Middleware(
		func(c *Context) (string, error) {
			return "invalid-token", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	writer := &authTestWriter{}
	ctx := NewContext(nil, writer, nil)

	middleware(ctx)

	if !ctx.IsAborted() {
		t.Fatal("expected request to be aborted")
	}

	if writer.status != StatusUnauthorized {
		t.Fatalf(
			"expected StatusUnauthorized, got %v",
			writer.status,
		)
	}

	if _, exists := ctx.Claims(); exists {
		t.Fatal("invalid token must not set claims")
	}

	var body map[string]string

	if err := json.Unmarshal(writer.payload, &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if body["error"] != "unauthorized" {
		t.Fatalf("unexpected response: %s", writer.payload)
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	manager := newTestAuthManager(t)

	middleware, err := manager.Middleware(
		func(c *Context) (string, error) {
			return "", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	writer := &authTestWriter{}
	ctx := NewContext(nil, writer, nil)

	middleware(ctx)

	if !ctx.IsAborted() {
		t.Fatal("expected request to be aborted")
	}

	if writer.status != StatusUnauthorized {
		t.Fatalf(
			"expected StatusUnauthorized, got %v",
			writer.status,
		)
	}
}

func TestAuthMiddlewareRejectsExtractorError(t *testing.T) {
	manager := newTestAuthManager(t)

	middleware, err := manager.Middleware(
		func(c *Context) (string, error) {
			return "", errors.New("header not found")
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	writer := &authTestWriter{}
	ctx := NewContext(nil, writer, nil)

	middleware(ctx)

	if !ctx.IsAborted() {
		t.Fatal("expected request to be aborted")
	}

	if writer.status != StatusUnauthorized {
		t.Fatalf(
			"expected StatusUnauthorized, got %v",
			writer.status,
		)
	}
}

func TestAuthMiddlewareRequiresExtractor(t *testing.T) {
	manager := newTestAuthManager(t)

	_, err := manager.Middleware(nil)

	if err == nil {
		t.Fatal("expected error for nil extractor")
	}
}
