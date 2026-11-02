package mrc

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPass generates a bcrypt hash of a password.
func (a *AuthManager) HashPass(
	password string,
) (string, error) {
	if a == nil {
		return "", ErrAuthNotConfigured
	}

	if password == "" {
		return "", ErrEmptyPassword
	}

	if len([]byte(password)) > 72 {
		return "", ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		a.bcryptCost,
	)
	if err != nil {
		return "", fmt.Errorf(
			"mrc: password hashing: %w",
			err,
		)
	}

	return string(hash), nil
}

// CheckPass checks a password against its bcrypt hash.
// A password mismatch returns (false, nil).
func (a *AuthManager) CheckPass(
	password string,
	hash string,
) (bool, error) {
	if a == nil {
		return false, ErrAuthNotConfigured
	}

	if password == "" {
		return false, ErrEmptyPassword
	}

	if len([]byte(password)) > 72 {
		return false, ErrPasswordTooLong
	}

	if hash == "" {
		return false, errors.New(
			"mrc: password hash cannot be empty",
		)
	}

	err := bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	)

	if err == nil {
		return true, nil
	}

	if errors.Is(
		err,
		bcrypt.ErrMismatchedHashAndPassword,
	) {
		return false, nil
	}

	return false, fmt.Errorf(
		"mrc: password verification: %w",
		err,
	)
}
