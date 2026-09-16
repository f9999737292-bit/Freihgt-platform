package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type IntegrationCredentialRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewIntegrationCredentialRepository(pool *pgxpool.Pool) *IntegrationCredentialRepository {
	return &IntegrationCredentialRepository{pool: pool}
}

func (r *IntegrationCredentialRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *IntegrationCredentialRepository) WithTx(tx pgx.Tx) *IntegrationCredentialRepository {
	return &IntegrationCredentialRepository{pool: r.pool, exec: tx}
}

func (r *IntegrationCredentialRepository) StoreHash(ctx context.Context, in domain.IntegrationCredential) (*domain.IntegrationCredential, error) {
	if err := validateIntegrationCredentialInput(in); err != nil {
		return nil, err
	}
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_integration_credentials (
			tenant_id, integration_principal_id, credential_type, secret_hash,
			hash_algorithm, hash_version, lookup_fingerprint
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, integration_principal_id, credential_type, secret_hash,
		          hash_algorithm, hash_version, lookup_fingerprint
	`, in.TenantID, in.IntegrationPrincipalID, in.CredentialType, in.SecretHash,
		in.HashAlgorithm, in.HashVersion, in.LookupFingerprint)
	return scanIntegrationCredential(row)
}

func (r *IntegrationCredentialRepository) GetByLookupFingerprint(ctx context.Context, tenantID uuid.UUID, fingerprint string) (*domain.IntegrationCredential, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, integration_principal_id, credential_type, secret_hash,
		       hash_algorithm, hash_version, lookup_fingerprint
		FROM rfx.rfx_integration_credentials
		WHERE tenant_id = $1 AND lookup_fingerprint = $2
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
	`, tenantID, strings.TrimSpace(fingerprint))
	credential, err := scanIntegrationCredential(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("integration credential not found")
		}
		return nil, mapDBError(err)
	}
	return credential, nil
}

func validateIntegrationCredentialInput(in domain.IntegrationCredential) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.IntegrationPrincipalID == uuid.Nil {
		return apperrors.Validation("integration_principal_id is required", map[string]any{"field": "integration_principal_id"})
	}
	if strings.TrimSpace(in.SecretHash) == "" {
		return apperrors.Validation("secret_hash is required", map[string]any{"field": "secret_hash"})
	}
	if strings.TrimSpace(in.HashAlgorithm) == "" {
		return apperrors.Validation("hash_algorithm is required", map[string]any{"field": "hash_algorithm"})
	}
	if strings.TrimSpace(in.LookupFingerprint) == "" {
		return apperrors.Validation("lookup_fingerprint is required", map[string]any{"field": "lookup_fingerprint"})
	}
	switch strings.TrimSpace(in.CredentialType) {
	case domain.IntegrationCredentialTypeOAuth, domain.IntegrationCredentialTypeAPIKey:
	default:
		return apperrors.Validation("invalid credential_type", map[string]any{"field": "credential_type"})
	}
	return nil
}

func scanIntegrationCredential(row pgx.Row) (*domain.IntegrationCredential, error) {
	var out domain.IntegrationCredential
	err := row.Scan(
		&out.ID, &out.TenantID, &out.IntegrationPrincipalID, &out.CredentialType,
		&out.SecretHash, &out.HashAlgorithm, &out.HashVersion, &out.LookupFingerprint,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CredentialPlaintextAbsent verifies no plaintext secret column exists on credentials table.
func (r *IntegrationCredentialRepository) CredentialPlaintextAbsent(ctx context.Context) (bool, error) {
	var count int
	err := r.db().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'rfx'
		  AND table_name = 'rfx_integration_credentials'
		  AND column_name IN ('secret', 'client_secret', 'api_key', 'plaintext_secret')
	`).Scan(&count)
	if err != nil {
		return false, mapDBError(err)
	}
	return count == 0, nil
}
