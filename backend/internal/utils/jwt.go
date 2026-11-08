package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       int    `json:"user_id"`
	Role         string `json:"role"`
	SchoolID     *int   `json:"school_id"`
	NamaInstansi string `json:"nama_instansi"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, userID int, role string, schoolID *int, namaInstansi string, tokenVersion int) (string, error) {
	return GenerateTokenWithTTL(secret, userID, role, schoolID, namaInstansi, tokenVersion, 10*time.Minute)
}

func GenerateTokenWithTTL(secret string, userID int, role string, schoolID *int, namaInstansi string, tokenVersion int, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	claims := Claims{
		UserID:       userID,
		Role:         role,
		SchoolID:     schoolID,
		NamaInstansi: namaInstansi,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
