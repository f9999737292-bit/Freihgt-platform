package integrationauth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type credentialCandidate struct {
	PrincipalID      uuid.UUID
	TenantID         uuid.UUID
	CompanyID        uuid.UUID
	CredentialID     uuid.UUID
	CredentialType   string
	PrincipalStatus  string
	AllowedCIDRs     []byte
	PrincipalRevoked *time.Time
	GraceEndsAt      *time.Time
	SecretHash       string
	HashAlgorithm    string
	CredExpiresAt    *time.Time
	CredRevokedAt    *time.Time
	RotationSuccessor *uuid.UUID
}

type Verifier struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewVerifier(pool *pgxpool.Pool) *Verifier {
	return &Verifier{pool: pool, now: time.Now}
}

func (v *Verifier) AuthenticateOAuthClientCredentials(ctx context.Context, clientID, clientSecret, clientIP string) (AuthenticatedContext, error) {
	clientID = strings.ToLower(strings.TrimSpace(clientID))
	if clientID == "" || strings.TrimSpace(clientSecret) == "" {
		return AuthenticatedContext{}, ErrInvalidClient
	}
	candidates, err := v.listOAuthCandidates(ctx, clientID)
	if err != nil {
		return AuthenticatedContext{}, err
	}
	var matched *credentialCandidate
	for i := range candidates {
		c := &candidates[i]
		if c.CredentialType != AuthSchemeOAuth {
			continue
		}
		if !v.credentialActive(c) {
			continue
		}
		if err := VerifySecret(clientSecret, c.SecretHash, c.HashAlgorithm); err != nil {
			continue
		}
		if matched != nil {
			return AuthenticatedContext{}, ErrInvalidClient
		}
		matched = c
	}
	if matched == nil {
		return AuthenticatedContext{}, ErrInvalidClient
	}
	if err := v.validatePrincipal(matched); err != nil {
		return AuthenticatedContext{}, err
	}
	ok, err := MatchAllowedCIDRs(matched.AllowedCIDRs, clientIP)
	if err != nil || !ok {
		return AuthenticatedContext{}, ErrCIDRDenied
	}
	scopes, err := v.listScopes(ctx, matched.TenantID, matched.PrincipalID)
	if err != nil {
		return AuthenticatedContext{}, err
	}
	return AuthenticatedContext{
		PrincipalID:  matched.PrincipalID,
		TenantID:     matched.TenantID,
		CompanyID:    matched.CompanyID,
		CredentialID: matched.CredentialID,
		AuthScheme:   AuthSchemeOAuth,
		Scopes:       scopes,
		ClientIP:     clientIP,
	}, nil
}

func (v *Verifier) AuthenticateAPIKeyBearer(ctx context.Context, apiKey, clientIP string) (AuthenticatedContext, error) {
	apiKey = strings.TrimSpace(apiKey)
	if !IsAPIKeyBearer(apiKey) {
		return AuthenticatedContext{}, ErrAuthSchemeDenied
	}
	fingerprint := APIKeyLookupFingerprint(apiKey)
	candidates, err := v.listAPIKeyCandidates(ctx, fingerprint)
	if err != nil {
		return AuthenticatedContext{}, err
	}
	var matched *credentialCandidate
	for i := range candidates {
		c := &candidates[i]
		if c.CredentialType != AuthSchemeAPIKey {
			continue
		}
		if !v.credentialActive(c) {
			continue
		}
		if err := VerifySecret(apiKey, c.SecretHash, c.HashAlgorithm); err != nil {
			continue
		}
		if matched != nil {
			return AuthenticatedContext{}, ErrInvalidCredentials
		}
		matched = c
	}
	if matched == nil {
		return AuthenticatedContext{}, ErrInvalidCredentials
	}
	if err := v.validatePrincipal(matched); err != nil {
		return AuthenticatedContext{}, err
	}
	ok, err := MatchAllowedCIDRs(matched.AllowedCIDRs, clientIP)
	if err != nil || !ok {
		return AuthenticatedContext{}, ErrCIDRDenied
	}
	scopes, err := v.listScopes(ctx, matched.TenantID, matched.PrincipalID)
	if err != nil {
		return AuthenticatedContext{}, err
	}
	return AuthenticatedContext{
		PrincipalID:  matched.PrincipalID,
		TenantID:     matched.TenantID,
		CompanyID:    matched.CompanyID,
		CredentialID: matched.CredentialID,
		AuthScheme:   AuthSchemeAPIKey,
		Scopes:       scopes,
		ClientIP:     clientIP,
	}, nil
}

func (v *Verifier) validatePrincipal(c *credentialCandidate) error {
	now := v.now().UTC()
	switch strings.TrimSpace(c.PrincipalStatus) {
	case "ACTIVE":
	default:
		if c.GraceEndsAt != nil && c.GraceEndsAt.After(now) {
			return nil
		}
		return ErrPrincipalInactive
	}
	if c.PrincipalRevoked != nil && (c.GraceEndsAt == nil || !c.GraceEndsAt.After(now)) {
		return ErrPrincipalInactive
	}
	return nil
}

func (v *Verifier) credentialActive(c *credentialCandidate) bool {
	now := v.now().UTC()
	if c.CredExpiresAt != nil && !c.CredExpiresAt.After(now) {
		return false
	}
	if c.CredRevokedAt == nil {
		return true
	}
	if c.RotationSuccessor != nil && c.GraceEndsAt != nil && c.GraceEndsAt.After(now) {
		return true
	}
	return false
}

func (v *Verifier) listOAuthCandidates(ctx context.Context, clientID string) ([]credentialCandidate, error) {
	rows, err := v.pool.Query(ctx, `
		SELECT p.id, p.tenant_id, p.company_id, p.credential_type, p.status, p.allowed_cidrs,
		       p.revoked_at, p.grace_ends_at,
		       c.id, c.secret_hash, c.hash_algorithm, c.expires_at, c.revoked_at, c.rotation_successor_id
		FROM rfx.rfx_integration_principals p
		JOIN rfx.rfx_integration_credentials c ON c.integration_principal_id = p.id
		WHERE lower(trim(p.client_id)) = $1
		  AND p.credential_type = 'OAUTH'
		  AND c.credential_type = 'OAUTH'
	`, clientID)
	if err != nil {
		return nil, fmt.Errorf("list oauth candidates: %w", err)
	}
	defer rows.Close()
	return scanCredentialCandidates(rows)
}

func (v *Verifier) listAPIKeyCandidates(ctx context.Context, fingerprint string) ([]credentialCandidate, error) {
	rows, err := v.pool.Query(ctx, `
		SELECT p.id, p.tenant_id, p.company_id, p.credential_type, p.status, p.allowed_cidrs,
		       p.revoked_at, p.grace_ends_at,
		       c.id, c.secret_hash, c.hash_algorithm, c.expires_at, c.revoked_at, c.rotation_successor_id
		FROM rfx.rfx_integration_credentials c
		JOIN rfx.rfx_integration_principals p ON p.id = c.integration_principal_id
		WHERE c.lookup_fingerprint = $1
		  AND p.credential_type = 'API_KEY'
		  AND c.credential_type = 'API_KEY'
	`, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("list api key candidates: %w", err)
	}
	defer rows.Close()
	return scanCredentialCandidates(rows)
}

func (v *Verifier) listScopes(ctx context.Context, tenantID, principalID uuid.UUID) ([]string, error) {
	rows, err := v.pool.Query(ctx, `
		SELECT scope FROM rfx.rfx_integration_scopes
		WHERE tenant_id = $1 AND integration_principal_id = $2
		ORDER BY scope
	`, tenantID, principalID)
	if err != nil {
		return nil, fmt.Errorf("list scopes: %w", err)
	}
	defer rows.Close()
	scopes := make([]string, 0)
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return NormalizeScopes(scopes)
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanCredentialCandidates(rows rowScanner) ([]credentialCandidate, error) {
	out := make([]credentialCandidate, 0)
	for rows.Next() {
		var c credentialCandidate
		if err := rows.Scan(
			&c.PrincipalID, &c.TenantID, &c.CompanyID, &c.CredentialType, &c.PrincipalStatus, &c.AllowedCIDRs,
			&c.PrincipalRevoked, &c.GraceEndsAt,
			&c.CredentialID, &c.SecretHash, &c.HashAlgorithm, &c.CredExpiresAt, &c.CredRevokedAt, &c.RotationSuccessor,
		); err != nil {
			return nil, err
		}
		if strings.TrimSpace(c.HashAlgorithm) != HashAlgorithmBcrypt {
			continue
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
