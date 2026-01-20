// Package api provides invite code generation and verification utilities.
package api

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Code alphabet - excludes 0, O, 1, I, L to avoid confusion
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// CodeExpiration is the default expiration time for invite codes (48 hours)
const CodeExpiration = 48 * time.Hour

// GenerateInviteCode generates a 10-character code formatted as XXXX-XXXX-XX.
func GenerateInviteCode() (string, error) {
	code := make([]byte, 10)
	alphabetLen := byte(len(codeAlphabet))

	randomBytes := make([]byte, 10)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	for i := 0; i < 10; i++ {
		code[i] = codeAlphabet[randomBytes[i]%alphabetLen]
	}

	// Format as XXXX-XXXX-XX
	formatted := string(code[:4]) + "-" + string(code[4:8]) + "-" + string(code[8:])
	return formatted, nil
}

// HashCode creates a bcrypt hash of the code for storage.
func HashCode(code string) (string, error) {
	normalized := NormalizeCode(code)
	hash, err := bcrypt.GenerateFromPassword([]byte(normalized), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash code: %w", err)
	}
	return string(hash), nil
}

// VerifyCode checks if the provided code matches the hash.
func VerifyCode(code, hash string) bool {
	normalized := NormalizeCode(code)
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized))
	return err == nil
}

// NormalizeCode removes dashes and converts to uppercase.
func NormalizeCode(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

// GetCodePrefix returns the first 4 characters for display (XXXX).
func GetCodePrefix(code string) string {
	normalized := NormalizeCode(code)
	if len(normalized) < 4 {
		return normalized
	}
	return normalized[:4]
}

// FormatExpiresIn formats the time remaining until expiration.
func FormatExpiresIn(expiresAt time.Time) string {
	remaining := time.Until(expiresAt)
	if remaining <= 0 {
		return "expired"
	}

	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// IsValidCodeFormat checks if the code has the correct format.
func IsValidCodeFormat(code string) bool {
	normalized := NormalizeCode(code)
	if len(normalized) != 10 {
		return false
	}

	// Check all characters are in the alphabet
	for _, c := range normalized {
		if !strings.ContainsRune(codeAlphabet, c) {
			return false
		}
	}
	return true
}
