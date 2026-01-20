// Package auth handles mvchat2 JWT token validation.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
	ErrMalformed    = errors.New("malformed token")
)

// Claims represents JWT claims from mvchat2.
type Claims struct {
	UserID string `json:"uid"` // UUID string
	jwt.RegisteredClaims
}

// UserInfo contains extracted user information from a validated token.
type UserInfo struct {
	UserID    string    // UUID string (e.g., "fa497802-ba40-4447-bc48-6da2bf726926")
	ExpiresAt time.Time
}

// Authenticator validates mvchat2 JWT tokens.
type Authenticator struct {
	tokenKey []byte
}

// New creates a new Authenticator with the given JWT signing key.
// The key should be the same as mvchat2's TOKEN_KEY.
func New(tokenKey []byte) *Authenticator {
	return &Authenticator{
		tokenKey: tokenKey,
	}
}

// ValidateToken validates a mvchat2 JWT token and returns user information.
func (a *Authenticator) ValidateToken(tokenString string) (*UserInfo, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.tokenKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.UserID == "" {
		return nil, ErrMalformed
	}

	var expiresAt time.Time
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	return &UserInfo{
		UserID:    claims.UserID,
		ExpiresAt: expiresAt,
	}, nil
}
