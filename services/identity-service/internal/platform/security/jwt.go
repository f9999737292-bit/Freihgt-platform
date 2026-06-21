package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenClaims struct {
	TenantID        string `json:"tenant_id"`
	Email           string `json:"email"`
	PreferredLocale string `json:"preferred_locale"`
	jwt.RegisteredClaims
}

type JWTService struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTService(secret string, ttlMinutes int) *JWTService {
	if ttlMinutes <= 0 {
		ttlMinutes = 60
	}
	return &JWTService{
		secret: []byte(secret),
		ttl:    time.Duration(ttlMinutes) * time.Minute,
	}
}

func (s *JWTService) CreateAccessToken(userID uuid.UUID, tenantID uuid.UUID, email, preferredLocale string) (string, int64, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.ttl)

	claims := TokenClaims{
		TenantID:        tenantID.String(),
		Email:           email,
		PreferredLocale: preferredLocale,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign token: %w", err)
	}

	return signed, int64(s.ttl.Seconds()), nil
}

func (s *JWTService) ParseAccessToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
