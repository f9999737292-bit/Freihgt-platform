package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type IntegrationPrincipalRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewIntegrationPrincipalRepository(pool *pgxpool.Pool) *IntegrationPrincipalRepository {
	return &IntegrationPrincipalRepository{pool: pool}
}

func (r *IntegrationPrincipalRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *IntegrationPrincipalRepository) WithTx(tx pgx.Tx) *IntegrationPrincipalRepository {
	return &IntegrationPrincipalRepository{pool: r.pool, exec: tx}
}

func (r *IntegrationPrincipalRepository) Create(ctx context.Context, in domain.IntegrationPrincipal) (*domain.IntegrationPrincipal, error) {
	if err := validateIntegrationPrincipalInput(in); err != nil {
		return nil, err
	}
	clientID := domain.NormalizeIntegrationClientID(in.ClientID)
	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_integration_principals (
			tenant_id, company_id, client_id, credential_type, status, allowed_cidrs
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING id, tenant_id, company_id, client_id, credential_type, status,
		          allowed_cidrs, revoked_at, grace_ends_at, created_at, updated_at
	`, in.TenantID, in.CompanyID, clientID, in.CredentialType, in.Status, nullableJSONBytes(in.AllowedCIDRs))
	return scanIntegrationPrincipal(row)
}

func (r *IntegrationPrincipalRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.IntegrationPrincipal, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, company_id, client_id, credential_type, status,
		       allowed_cidrs, revoked_at, grace_ends_at, created_at, updated_at
		FROM rfx.rfx_integration_principals
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	principal, err := scanIntegrationPrincipal(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("integration principal not found")
		}
		return nil, mapDBError(err)
	}
	return principal, nil
}

func (r *IntegrationPrincipalRepository) GetByClientID(ctx context.Context, tenantID uuid.UUID, clientID string) (*domain.IntegrationPrincipal, error) {
	normalized := domain.NormalizeIntegrationClientID(clientID)
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, company_id, client_id, credential_type, status,
		       allowed_cidrs, revoked_at, grace_ends_at, created_at, updated_at
		FROM rfx.rfx_integration_principals
		WHERE tenant_id = $1 AND lower(trim(client_id)) = $2
	`, tenantID, normalized)
	principal, err := scanIntegrationPrincipal(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("integration principal not found")
		}
		return nil, mapDBError(err)
	}
	return principal, nil
}

func validateIntegrationPrincipalInput(in domain.IntegrationPrincipal) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.CompanyID == uuid.Nil {
		return apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	if strings.TrimSpace(in.ClientID) == "" {
		return apperrors.Validation("client_id is required", map[string]any{"field": "client_id"})
	}
	switch strings.TrimSpace(in.CredentialType) {
	case domain.IntegrationCredentialTypeOAuth, domain.IntegrationCredentialTypeAPIKey:
	default:
		return apperrors.Validation("invalid credential_type", map[string]any{"field": "credential_type"})
	}
	switch strings.TrimSpace(in.Status) {
	case domain.IntegrationPrincipalStatusActive, domain.IntegrationPrincipalStatusRevoked, domain.IntegrationPrincipalStatusSuspended:
	default:
		return apperrors.Validation("invalid status", map[string]any{"field": "status"})
	}
	return nil
}

func scanIntegrationPrincipal(row pgx.Row) (*domain.IntegrationPrincipal, error) {
	var out domain.IntegrationPrincipal
	var allowedCIDRs []byte
	var revokedAt *time.Time
	var graceEndsAt *time.Time
	err := row.Scan(
		&out.ID, &out.TenantID, &out.CompanyID, &out.ClientID, &out.CredentialType, &out.Status,
		&allowedCIDRs, &revokedAt, &graceEndsAt, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	out.AllowedCIDRs = allowedCIDRs
	out.RevokedAt = revokedAt
	out.GraceEndsAt = graceEndsAt
	return &out, nil
}

func nullableJSONBytes(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
