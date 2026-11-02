package mrc

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPass(t *testing.T) {
	manager := newTestAuthManager(t)

	password := "my-super-secret-password"

	hash, err := manager.HashPass(password)
	if err != nil {
		t.Fatalf("HashPass() error = %v", err)
	}

	if hash == "" {
		t.Fatal("hash is empty")
	}

	if hash == password {
		t.Fatal("password was not hashed")
	}

	if hash[0] != '$' {
		t.Fatal("unexpected bcrypt hash format")
	}
}

func TestHashPassDifferentSalts(t *testing.T) {
	manager := newTestAuthManager(t)

	password := "same-password"

	hash1, err := manager.HashPass(password)
	if err != nil {
		t.Fatal(err)
	}

	hash2, err := manager.HashPass(password)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Fatal("bcrypt generated identical hashes")
	}
}

func TestCheckPass(t *testing.T) {
	manager := newTestAuthManager(t)

	password := "correct-password"

	hash, err := manager.HashPass(password)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := manager.CheckPass(password, hash)
	if err != nil {
		t.Fatalf("CheckPass() error = %v", err)
	}

	if !valid {
		t.Fatal("expected password to match")
	}
}

func TestCheckPassWrongPassword(t *testing.T) {
	manager := newTestAuthManager(t)

	hash, err := manager.HashPass("correct-password")
	if err != nil {
		t.Fatal(err)
	}

	valid, err := manager.CheckPass("wrong-password", hash)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if valid {
		t.Fatal("wrong password was accepted")
	}
}

func TestHashPassRejectsEmptyPassword(t *testing.T) {
	manager := newTestAuthManager(t)

	_, err := manager.HashPass("")

	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf(
			"expected ErrEmptyPassword, got %v",
			err,
		)
	}
}

func TestCheckPassRejectsEmptyPassword(t *testing.T) {
	manager := newTestAuthManager(t)

	_, err := manager.CheckPass("", "some-hash")

	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf(
			"expected ErrEmptyPassword, got %v",
			err,
		)
	}
}

func TestHashPassRejectsLongPassword(t *testing.T) {
	manager := newTestAuthManager(t)

	password := strings.Repeat("a", 73)

	_, err := manager.HashPass(password)

	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf(
			"expected ErrPasswordTooLong, got %v",
			err,
		)
	}
}

func TestCheckPassRejectsLongPassword(t *testing.T) {
	manager := newTestAuthManager(t)

	password := strings.Repeat("a", 73)

	_, err := manager.CheckPass(password, "some-hash")

	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf(
			"expected ErrPasswordTooLong, got %v",
			err,
		)
	}
}

func TestCheckPassRejectsInvalidHash(t *testing.T) {
	manager := newTestAuthManager(t)

	_, err := manager.CheckPass(
		"some-password",
		"invalid-hash",
	)

	if err == nil {
		t.Fatal("expected invalid hash error")
	}
}
