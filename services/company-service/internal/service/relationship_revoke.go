package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

// RevokeRelationshipInput is the verified-user revoke command.
// The body does not choose the tenant, the company, or the side.
type RevokeRelationshipInput struct {
	Principal                   domain.Principal
	SelectedCompanyID           uuid.UUID
	RelationshipID              uuid.UUID
	IdempotencyKey              string
	Reason                      string
	AcceptanceToken             string
	ExpectedRelationshipVersion *int
	BodyTenantID                *uuid.UUID
	BodyCompanyID               *uuid.UUID
	CorrelationID               string
}

// RevokeRelationshipOutput is HTTP 200. It has no foreign tenant, company, or token.
type RevokeRelationshipOutput struct {
	RelationshipID      uuid.UUID
	RelationshipVersion int
	Status              string
}

func (s *RelationshipService) Revoke(ctx context.Context, in RevokeRelationshipInput) (RevokeRelationshipOutput, error) {
	if err := s.commandsOpen(); err != nil {
		return RevokeRelationshipOutput{}, err
	}
	if in.BodyTenantID != nil && *in.BodyTenantID != in.Principal.TenantID {
		return RevokeRelationshipOutput{}, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict")
	}
	if in.BodyCompanyID != nil && *in.BodyCompanyID != in.SelectedCompanyID {
		return RevokeRelationshipOutput{}, revokeForbidden()
	}
	if in.SelectedCompanyID == uuid.Nil {
		return RevokeRelationshipOutput{}, revokeForbidden()
	}
	if strings.TrimSpace(in.AcceptanceToken) != "" {
		return RevokeRelationshipOutput{}, apperrors.Validation("relationship request is invalid", nil)
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" || len(key) > 200 {
		return RevokeRelationshipOutput{}, apperrors.Validation("relationship request is invalid", nil)
	}
	reason := strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(reason) > 500 {
		return RevokeRelationshipOutput{}, apperrors.Validation("relationship request is invalid", nil)
	}
	if in.RelationshipID == uuid.Nil {
		return RevokeRelationshipOutput{}, relationshipRevokeNotFound()
	}
	stored, err := s.store.RevokeRelationship(ctx, domain.PersistRevoke{
		ActorTenantID:               in.Principal.TenantID,
		ActorUserID:                 in.Principal.UserID,
		SelectedCompanyID:           in.SelectedCompanyID,
		RelationshipID:              in.RelationshipID,
		IdempotencyKey:              key,
		Reason:                      reason,
		ExpectedRelationshipVersion: in.ExpectedRelationshipVersion,
		CorrelationID:               strings.TrimSpace(in.CorrelationID),
	})
	if err != nil {
		return RevokeRelationshipOutput{}, err
	}
	return RevokeRelationshipOutput{
		RelationshipID: stored.RelationshipID, RelationshipVersion: stored.RelationshipVersion, Status: stored.Status,
	}, nil
}

func revokeForbidden() error {
	return apperrors.New(apperrors.CodeRelationshipRevokeForbidden, "relationship command refused")
}

func relationshipRevokeNotFound() error {
	return apperrors.New(apperrors.CodeRelationshipNotFound, "relationship not found")
}
