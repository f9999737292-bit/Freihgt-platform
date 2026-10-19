package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

// ListSourceRequestsInput is the trusted source-owner status read.
// Tenant and user come from the source owner, which proved them from the JWT and the source row.
type ListSourceRequestsInput struct {
	ServiceToken     string
	SourceType       string
	SourceID         uuid.UUID
	SourceTenantID   uuid.UUID
	RequestingUserID uuid.UUID
}

// SourceRequestStatus is one status-list row. It has no acceptance token.
type SourceRequestStatus struct {
	RequestID        uuid.UUID
	Status           string
	CarrierCompanyID uuid.UUID
	TokenExpiresAt   time.Time
}

// CancelRelationshipInput is the trusted source-owner cancel command.
type CancelRelationshipInput struct {
	ServiceToken     string
	SourceType       string
	SourceID         uuid.UUID
	SourceTenantID   uuid.UUID
	RequestingUserID uuid.UUID
	RequestID        uuid.UUID
	IdempotencyKey   string
	Reason           string
	CorrelationID    string
	BodyTenantID     *uuid.UUID
	BodyCompanyID    *uuid.UUID
}

// CancelRelationshipOutput is HTTP 200 for both CANCELLED and EXPIRED. It has no acceptance token.
type CancelRelationshipOutput struct {
	RequestID uuid.UUID
	Status    string
}

func (s *RelationshipService) ListSourceRequests(ctx context.Context, in ListSourceRequestsInput) ([]SourceRequestStatus, error) {
	if err := s.commandsOpen(); err != nil {
		return nil, err
	}
	caller, ok := identifyRelationshipCaller(s.cfg.ShipmentServiceToken, s.cfg.RfxServiceToken, in.ServiceToken)
	if !ok {
		return nil, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required")
	}
	sourceType := strings.TrimSpace(in.SourceType)
	if !domain.ValidSourceType(sourceType) || in.SourceID == uuid.Nil || in.SourceTenantID == uuid.Nil || in.RequestingUserID == uuid.Nil {
		return nil, apperrors.Validation("relationship request is invalid", nil)
	}
	if !domain.CallerAllowsSource(caller, sourceType) {
		return nil, refuseCreate()
	}
	rows, err := s.store.ListSourceRelationshipRequests(ctx, sourceType, in.SourceID, in.SourceTenantID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []SourceRequestStatus{}, nil
	}
	visible := make([]SourceRequestStatus, 0, len(rows))
	memberWithoutRole := false
	active := false
	for _, row := range rows {
		actor, err := s.authority.LoadRelationshipActor(ctx, in.SourceTenantID, in.RequestingUserID, row.RequestingCompanyID)
		if err != nil {
			return nil, err
		}
		if actor.Active {
			active = true
		}
		if actor.Active && actor.ActiveMembership && !domain.CanCreateRelationshipRequest(actor, sourceType) {
			memberWithoutRole = true
		}
		if domain.CanCreateRelationshipRequest(actor, sourceType) {
			visible = append(visible, SourceRequestStatus{
				RequestID: row.RequestID, Status: row.Status, CarrierCompanyID: row.CarrierCompanyID, TokenExpiresAt: row.TokenExpiresAt,
			})
		}
	}
	if len(visible) > 0 {
		return visible, nil
	}
	if memberWithoutRole || !active {
		return nil, refuseCreate()
	}
	return nil, relationshipNotFound()
}

func (s *RelationshipService) Cancel(ctx context.Context, in CancelRelationshipInput) (CancelRelationshipOutput, error) {
	if err := s.commandsOpen(); err != nil {
		return CancelRelationshipOutput{}, err
	}
	caller, ok := identifyRelationshipCaller(s.cfg.ShipmentServiceToken, s.cfg.RfxServiceToken, in.ServiceToken)
	if !ok {
		return CancelRelationshipOutput{}, apperrors.New(apperrors.CodeAuthenticationRequired, "authentication required")
	}
	sourceType := strings.TrimSpace(in.SourceType)
	key := strings.TrimSpace(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if !domain.ValidSourceType(sourceType) || in.SourceID == uuid.Nil || in.SourceTenantID == uuid.Nil || in.RequestingUserID == uuid.Nil || in.RequestID == uuid.Nil || key == "" || len(key) > 200 || len(reason) > 500 {
		return CancelRelationshipOutput{}, apperrors.Validation("relationship request is invalid", nil)
	}
	if !domain.CallerAllowsSource(caller, sourceType) {
		return CancelRelationshipOutput{}, refuseCreate()
	}
	if in.BodyTenantID != nil && *in.BodyTenantID != in.SourceTenantID {
		return CancelRelationshipOutput{}, apperrors.New(apperrors.CodeTenantContextConflict, "tenant context conflict")
	}
	stored, err := s.store.CancelRelationshipRequest(ctx, domain.PersistCancel{
		CallerService: caller, SourceType: sourceType, SourceID: in.SourceID, SourceTenantID: in.SourceTenantID,
		ActorUserID: in.RequestingUserID, RequestID: in.RequestID, IdempotencyKey: key, Reason: reason,
		CorrelationID: strings.TrimSpace(in.CorrelationID), AssertedCompanyID: in.BodyCompanyID,
	})
	if err != nil {
		return CancelRelationshipOutput{}, err
	}
	return CancelRelationshipOutput{RequestID: stored.RequestID, Status: stored.Status}, nil
}
