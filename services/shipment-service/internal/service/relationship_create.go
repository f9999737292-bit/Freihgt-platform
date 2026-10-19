package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// RelationshipActor is the verified JWT user. Tenant is not taken from a header or body.
type RelationshipActor struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
}

// RelationshipCreateResult is returned to the authenticated initiator.
// AcceptanceToken is present only when company-service created the request in this call.
type RelationshipCreateResult struct {
	RequestID       uuid.UUID
	Status          string
	AcceptanceToken string
	Created         bool
}

// RelationshipCreateInitiator starts a shipment relationship request.
// It reuses ShipmentSourceAuthority for the owner predicate and fingerprint.
// It does not store the acceptance token, and it does not replace a lost response
// with a new idempotency key. Status and cancel recover a lost token without Confirm,
// so a later fingerprint change does not block recovery.
type RelationshipCreateInitiator struct {
	reader    ShipmentSourceReader
	authority *ShipmentSourceAuthority
	client    *CompanyRelationshipCreateClient
}

func NewRelationshipCreateInitiator(reader ShipmentSourceReader, client *CompanyRelationshipCreateClient) *RelationshipCreateInitiator {
	return &RelationshipCreateInitiator{
		reader:    reader,
		authority: NewShipmentSourceAuthority(reader),
		client:    client,
	}
}

func (s *RelationshipCreateInitiator) Create(ctx context.Context, actor RelationshipActor, shipmentID uuid.UUID, idempotencyKey string) (RelationshipCreateResult, error) {
	if s == nil || s.reader == nil || s.authority == nil || s.client == nil {
		return RelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	key := strings.TrimSpace(idempotencyKey)
	if actor.UserID == uuid.Nil || actor.TenantID == uuid.Nil || shipmentID == uuid.Nil || key == "" || len(key) > 200 {
		return RelationshipCreateResult{}, apperrors.Validation("relationship request is invalid", nil)
	}
	row, err := s.reader.ReadShipmentSource(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, domain.ErrShipmentSourceMissing) {
			return RelationshipCreateResult{}, apperrors.NotFound("shipment not found")
		}
		return RelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if row.TenantID != actor.TenantID {
		return RelationshipCreateResult{}, apperrors.NotFound("shipment not found")
	}
	fingerprint, err := domain.ShipmentSourceFingerprint(row)
	if err != nil {
		return RelationshipCreateResult{}, apperrors.NotFound("shipment not found")
	}
	decision := s.authority.Confirm(ctx, domain.ShipmentSourceSubject{
		SourceType:              domain.ShipmentSourceType,
		SourceID:                row.ID,
		SourceTenantID:          row.TenantID,
		Fingerprint:             fingerprint,
		ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID:     row.ShipperCompanyID,
		RequestingUserID:        actor.UserID,
	})
	if err := relationshipDecisionError(decision); err != nil {
		return RelationshipCreateResult{}, err
	}
	created, err := s.client.Create(ctx, CompanyRelationshipCreateRequest{
		SourceType:              domain.ShipmentSourceType,
		SourceID:                row.ID,
		SourceTenantID:          row.TenantID,
		Fingerprint:             fingerprint,
		ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID:     row.ShipperCompanyID,
		RequestingUserID:        actor.UserID,
		IdempotencyKey:          key,
	})
	if err != nil {
		return RelationshipCreateResult{}, err
	}
	out := RelationshipCreateResult{RequestID: created.RequestID, Status: created.Status, Created: created.Created}
	if created.Created {
		out.AcceptanceToken = created.AcceptanceToken
	}
	return out, nil
}

func (s *RelationshipCreateInitiator) Status(ctx context.Context, actor RelationshipActor, shipmentID uuid.UUID) ([]RelationshipStatusItem, error) {
	if s == nil || s.reader == nil || s.client == nil {
		return nil, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if actor.UserID == uuid.Nil || actor.TenantID == uuid.Nil || shipmentID == uuid.Nil {
		return nil, apperrors.Validation("relationship request is invalid", nil)
	}
	if err := s.requireVisibleShipment(ctx, actor, shipmentID); err != nil {
		return nil, err
	}
	return s.client.ListStatus(ctx, shipmentID, actor.TenantID, actor.UserID)
}

func (s *RelationshipCreateInitiator) Cancel(ctx context.Context, actor RelationshipActor, shipmentID, requestID uuid.UUID, idempotencyKey, reason string) (RelationshipCancelResult, error) {
	if s == nil || s.reader == nil || s.client == nil {
		return RelationshipCancelResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	key := strings.TrimSpace(idempotencyKey)
	if actor.UserID == uuid.Nil || actor.TenantID == uuid.Nil || shipmentID == uuid.Nil || requestID == uuid.Nil || key == "" || len(key) > 200 || len(strings.TrimSpace(reason)) > 500 {
		return RelationshipCancelResult{}, apperrors.Validation("relationship request is invalid", nil)
	}
	if err := s.requireVisibleShipment(ctx, actor, shipmentID); err != nil {
		return RelationshipCancelResult{}, err
	}
	return s.client.Cancel(ctx, shipmentID, actor.TenantID, actor.UserID, requestID, key, strings.TrimSpace(reason))
}

func (s *RelationshipCreateInitiator) requireVisibleShipment(ctx context.Context, actor RelationshipActor, shipmentID uuid.UUID) error {
	row, err := s.reader.ReadShipmentSource(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, domain.ErrShipmentSourceMissing) {
			return apperrors.NotFound("shipment not found")
		}
		return apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if row.ID != shipmentID || row.TenantID != actor.TenantID {
		return apperrors.NotFound("shipment not found")
	}
	return nil
}

func relationshipDecisionError(decision string) error {
	switch decision {
	case domain.ShipmentSourceCreateConfirmed:
		return nil
	case domain.ShipmentSourceCreateNotFound, domain.ShipmentSourceCreateTenantMismatch:
		return apperrors.NotFound("shipment not found")
	case domain.ShipmentSourceCreateStale:
		return apperrors.New(apperrors.CodeRelationshipRequestStale, "relationship command refused")
	case domain.ShipmentSourceCreateActorMismatch:
		return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	default:
		return apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
}
