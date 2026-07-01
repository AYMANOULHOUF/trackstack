package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// GenerateDeviceToken creates a long-lived, high-entropy token for device
// auth (read_me.md Step 12). The plaintext is shown to the caller once;
// only its hash is persisted.
func GenerateDeviceToken() (plaintext string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return "dtk_" + hex.EncodeToString(buf), nil
}

// HashDeviceToken returns a deterministic SHA-256 hash suitable for an
// equality lookup in Postgres (devices.api_token_hash). Unlike bcrypt,
// this needs to support indexed lookup by exact value, which is fine here
// since the token itself already has 256 bits of entropy.
func HashDeviceToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// HashPassword and CheckPassword wrap bcrypt for dashboard user accounts.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
