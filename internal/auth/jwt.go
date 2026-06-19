package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenDuration  = 15 * time.Minute
	refreshTokenDuration = 7 * 24 * time.Hour
)

type Claims struct {
	jwt.RegisteredClaims
}

func jwtSecret() ([]byte, error) {
	s := os.Getenv("JWT_SECRET")
	if len(s) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	return []byte(s), nil
}

// IssueAccessToken returns a signed HS256 JWT for the given user ID.
func IssueAccessToken(userID string) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenDuration)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// VerifyAccessToken parses and validates a signed JWT, returning the user ID.
func VerifyAccessToken(tokenStr string) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}
	return claims.Subject, nil
}

// RefreshTokenDuration exposes the refresh lifetime for token insertion.
func RefreshTokenDuration() time.Duration {
	return refreshTokenDuration
}
