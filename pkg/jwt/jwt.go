// Package jwt centralizes token generation and parsing so the
// signing method, claim shape, and secret handling exist in exactly
// one place. Both the auth service (issues tokens) and the auth
// middleware (validates tokens) depend on this instead of duplicating
// jwt.Parse/jwt.NewWithClaims logic.
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

type Manager struct {
	secret     []byte
	expiration time.Duration
}

func NewManager(secret string, expiration time.Duration) *Manager {
	return &Manager{secret: []byte(secret), expiration: expiration}
}

// Generate issues a signed token whose subject is the user ID.
func (m *Manager) Generate(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(m.expiration).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseUserID validates the token's signature and expiry, and
// returns the subject (user ID) claim.
func (m *Manager) ParseUserID(tokenString string) (uint, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, ErrInvalidToken
	}

	exp, ok := claims["exp"].(float64)
	if !ok || float64(time.Now().Unix()) > exp {
		return 0, ErrExpiredToken
	}

	sub, ok := claims["sub"].(float64)
	if !ok {
		return 0, ErrInvalidToken
	}

	return uint(sub), nil
}
