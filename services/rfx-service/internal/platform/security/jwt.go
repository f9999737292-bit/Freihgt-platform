package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AccessPrincipal is the verified user and tenant from a bearer JWT.
// Tenant and user id come only from these claims.
type AccessPrincipal struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Subject  string `json:"sub"`
	TenantID string `json:"tenant_id"`
	Exp      *int64 `json:"exp"`
}

// SignHS256 builds the same HS256 access token company-service accepts.
func SignHS256(secret string, userID, tenantID uuid.UUID, expiresAt time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("empty jwt secret")
	}
	headJSON, err := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	exp := expiresAt.Unix()
	claimsJSON, err := json.Marshal(jwtClaims{Subject: userID.String(), TenantID: tenantID.String(), Exp: &exp})
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + signature, nil
}

// ParseHS256 verifies the bearer token. An empty secret fails closed.
func ParseHS256(secret, token string, now time.Time) (AccessPrincipal, error) {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(token) == "" {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "HS256" {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	presented, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expected, presented) {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil || claims.Exp == nil {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	if now.Unix() >= *claims.Exp {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return AccessPrincipal{}, errors.New("authentication required")
	}
	return AccessPrincipal{UserID: userID, TenantID: tenantID}, nil
}
