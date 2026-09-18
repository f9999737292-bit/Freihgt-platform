package integrationauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	AuditActionAuthenticated = "rfx.erp.client.authenticated.v1"
	AuditActionAccessDenied  = "rfx.erp.access.denied.v1"
)

type AuditRecorder struct {
	pool *pgxpool.Pool
}

func NewAuditRecorder(pool *pgxpool.Pool) *AuditRecorder {
	return &AuditRecorder{pool: pool}
}

func (a *AuditRecorder) RecordAuthSuccess(ctx context.Context, authCtx AuthenticatedContext, requestID string) error {
	companyID := authCtx.CompanyID
	return a.record(ctx, authCtx.TenantID, authCtx.PrincipalID, &companyID, AuditActionAuthenticated, map[string]any{
		"auth_scheme":              authCtx.AuthScheme,
		"integration_principal_id": authCtx.PrincipalID.String(),
		"credential_id":            authCtx.CredentialID.String(),
		"request_id":               requestID,
		"client_ip":                authCtx.ClientIP,
		"result":                   "success",
	})
}

func (a *AuditRecorder) RecordAuthFailure(ctx context.Context, tenantID *uuid.UUID, principalID *uuid.UUID, reason, authScheme, requestID, clientIP string) error {
	var tenant uuid.UUID
	var entity uuid.UUID
	if tenantID != nil {
		tenant = *tenantID
	}
	if principalID != nil {
		entity = *principalID
	} else {
		entity = uuid.Nil
	}
	meta := map[string]any{
		"result":      "denied",
		"reason_code": reason,
		"request_id":  requestID,
		"client_ip":   clientIP,
	}
	if authScheme != "" {
		meta["auth_scheme"] = authScheme
	}
	return a.record(ctx, tenant, entity, nil, AuditActionAccessDenied, meta)
}

func (a *AuditRecorder) record(ctx context.Context, tenantID, entityID uuid.UUID, companyID *uuid.UUID, action string, metadata map[string]any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	_, err = a.pool.Exec(ctx, `
		INSERT INTO rfx.audit_events (
			tenant_id, entity_type, entity_id, action, actor_company_id, metadata
		) VALUES ($1, 'integration_principal', $2, $3, $4, $5::jsonb)
	`, tenantID, entityID, action, companyID, string(raw))
	return err
}
