package integrationauth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTService struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTService(secret string, ttl time.Duration) *JWTService {
	if ttl <= 0 {
		ttl = TokenTTL
	}
	return &JWTService{secret: []byte(secret), ttl: ttl}
}

func (s *JWTService) CreateIntegrationToken(ctx AuthenticatedContext) (token string, expiresIn int64, err error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.ttl)
	scope := JoinScopes(ctx.Scopes)
	claims := IntegrationTokenClaims{
		TenantID:   ctx.TenantID.String(),
		CompanyID:  ctx.CompanyID.String(),
		Scope:      scope,
		AuthScheme: ctx.AuthScheme,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   ctx.PrincipalID.String(),
			Issuer:    TokenIssuer,
			Audience:  jwt.ClaimStrings{TokenAudience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	claims.Act.Kind = ActorKindIntegration

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign integration token: %w", err)
	}
	return signed, int64(s.ttl.Seconds()), nil
}

func (s *JWTService) ParseIntegrationToken(tokenString string) (*IntegrationTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &IntegrationTokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*IntegrationTokenClaims)
	if !ok || !token.Valid || !claims.IsIntegrationToken() {
		return nil, fmt.Errorf("invalid integration token")
	}
	if claims.Issuer != TokenIssuer {
		return nil, fmt.Errorf("invalid issuer")
	}
	if len(claims.Audience) == 0 || claims.Audience[0] != TokenAudience {
		return nil, fmt.Errorf("invalid audience")
	}
	return claims, nil
}

func ClaimsToContext(claims *IntegrationTokenClaims) (AuthenticatedContext, error) {
	principalID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return AuthenticatedContext{}, fmt.Errorf("invalid principal subject")
	}
	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return AuthenticatedContext{}, fmt.Errorf("invalid tenant claim")
	}
	companyID, err := uuid.Parse(claims.CompanyID)
	if err != nil {
		return AuthenticatedContext{}, fmt.Errorf("invalid company claim")
	}
	scopes := make([]string, 0)
	for _, part := range splitScopes(claims.Scope) {
		scopes = append(scopes, part)
	}
	normalized, err := NormalizeScopes(scopes)
	if err != nil {
		return AuthenticatedContext{}, err
	}
	scheme := claims.AuthScheme
	if scheme == "" {
		scheme = AuthSchemeOAuth
	}
	return AuthenticatedContext{
		PrincipalID: principalID,
		TenantID:    tenantID,
		CompanyID:   companyID,
		AuthScheme:  scheme,
		Scopes:      normalized,
	}, nil
}

func splitScopes(raw string) []string {
	return strings.Fields(strings.TrimSpace(raw))
}
