package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const minPasswordLen = 8

// ErrWeakPassword is returned when a password does not meet the minimum length.
var ErrWeakPassword = errors.New("password must be at least 8 characters")

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	if len(password) < minPasswordLen {
		return "", ErrWeakPassword
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
