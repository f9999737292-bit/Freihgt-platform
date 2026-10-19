package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// RfxRelationshipActor is the verified JWT user. Tenant is not taken from a header or body.
type RfxRelationshipActor struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
}

// RfxRelationshipCreateResult is returned to the authenticated initiator.
// AcceptanceToken is present only when company-service created the request in this call.
type RfxRelationshipCreateResult struct {
	RequestID       uuid.UUID
	Status          string
	AcceptanceToken string
	Created         bool
}

// RfxRelationshipCreateInitiator starts an award or award-link relationship request.
// It reuses RfxSourceAuthority for the owner predicate and fingerprint.
// It does not store the acceptance token, and it does not replace a lost response
// with a new idempotency key. Status and cancel recover a lost token without Confirm,
// so a later fingerprint change does not block recovery.
type RfxRelationshipCreateInitiator struct {
	reader    RfxSourceReader
	authority *RfxSourceAuthority
	client    *CompanyRelationshipCreateClient
}

func NewRfxRelationshipCreateInitiator(reader RfxSourceReader, client *CompanyRelationshipCreateClient) *RfxRelationshipCreateInitiator {
	return &RfxRelationshipCreateInitiator{
		reader:    reader,
		authority: NewRfxSourceAuthority(reader),
		client:    client,
	}
}

func (s *RfxRelationshipCreateInitiator) Create(ctx context.Context, actor RfxRelationshipActor, sourceType string, sourceID uuid.UUID, idempotencyKey string) (RfxRelationshipCreateResult, error) {
	if s == nil || s.reader == nil || s.authority == nil || s.client == nil {
		return RfxRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	key := strings.TrimSpace(idempotencyKey)
	if actor.UserID == uuid.Nil || actor.TenantID == uuid.Nil || sourceID == uuid.Nil || key == "" || len(key) > 200 {
		return RfxRelationshipCreateResult{}, apperrors.Validation("relationship request is invalid", nil)
	}
	if sourceType != domain.RfxSourceTypeAward && sourceType != domain.RfxSourceTypeAwardLink {
		return RfxRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	subject, err := s.subjectFromRow(ctx, actor, sourceType, sourceID)
	if err != nil {
		return RfxRelationshipCreateResult{}, err
	}
	decision := s.authority.Confirm(ctx, subject)
	if err := rfxRelationshipDecisionError(decision); err != nil {
		return RfxRelationshipCreateResult{}, err
	}
	created, err := s.client.Create(ctx, CompanyRelationshipCreateRequest{
		SourceType:              subject.SourceType,
		SourceID:                subject.SourceID,
		SourceTenantID:          subject.SourceTenantID,
		Fingerprint:             subject.Fingerprint,
		ShipperCarrierCompanyID: subject.ShipperCarrierCompanyID,
		RequestingCompanyID:     subject.RequestingCompanyID,
		RequestingUserID:        subject.RequestingUserID,
		IdempotencyKey:          key,
	})
	if err != nil {
		return RfxRelationshipCreateResult{}, err
	}
	out := RfxRelationshipCreateResult{RequestID: created.RequestID, Status: created.Status, Created: created.Created}
	if created.Created {
		out.AcceptanceToken = created.AcceptanceToken
	}
	return out, nil
}

func (s *RfxRelationshipCreateInitiator) subjectFromRow(ctx context.Context, actor RfxRelationshipActor, sourceType string, sourceID uuid.UUID) (domain.RfxSourceSubject, error) {
	switch sourceType {
	case domain.RfxSourceTypeAward:
		row, err := s.reader.ReadAwardSource(ctx, sourceID)
		if err != nil {
			if errors.Is(err, domain.ErrRfxSourceMissing) {
				return domain.RfxSourceSubject{}, apperrors.NotFound("rfx award not found")
			}
			return domain.RfxSourceSubject{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
		}
		if row.TenantID != actor.TenantID {
			return domain.RfxSourceSubject{}, apperrors.NotFound("rfx award not found")
		}
		fingerprint, err := domain.RfxAwardSourceFingerprint(row)
		if err != nil {
			return domain.RfxSourceSubject{}, apperrors.NotFound("rfx award not found")
		}
		return domain.RfxSourceSubject{
			SourceType: domain.RfxSourceTypeAward, SourceID: row.ID, SourceTenantID: row.TenantID,
			Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
			RequestingCompanyID: row.OwnerCompanyID, RequestingUserID: actor.UserID,
		}, nil
	default:
		row, err := s.reader.ReadAwardLinkSource(ctx, sourceID)
		if err != nil {
			if errors.Is(err, domain.ErrRfxSourceMissing) {
				return domain.RfxSourceSubject{}, apperrors.NotFound("rfx award link not found")
			}
			return domain.RfxSourceSubject{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
		}
		if row.TenantID != actor.TenantID {
			return domain.RfxSourceSubject{}, apperrors.NotFound("rfx award link not found")
		}
		fingerprint, err := domain.RfxAwardLinkSourceFingerprint(row)
		if err != nil {
			return domain.RfxSourceSubject{}, apperrors.NotFound("rfx award link not found")
		}
		return domain.RfxSourceSubject{
			SourceType: domain.RfxSourceTypeAwardLink, SourceID: row.ID, SourceTenantID: row.TenantID,
			Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
			RequestingCompanyID: row.OwnerCompanyID, RequestingUserID: actor.UserID,
		}, nil
	}
}

func (s *RfxRelationshipCreateInitiator) Status(ctx context.Context, actor RfxRelationshipActor, sourceType string, sourceID uuid.UUID) ([]RfxRelationshipStatusItem, error) {
	if s == nil || s.reader == nil || s.client == nil {
		return nil, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if actor.UserID == uuid.Nil || actor.TenantID == uuid.Nil || sourceID == uuid.Nil {
		return nil, apperrors.Validation("relationship request is invalid", nil)
	}
	if err := s.requireVisibleSource(ctx, actor, sourceType, sourceID); err != nil {
		return nil, err
	}
	return s.client.ListStatus(ctx, sourceType, sourceID, actor.TenantID, actor.UserID)
}

func (s *RfxRelationshipCreateInitiator) Cancel(ctx context.Context, actor RfxRelationshipActor, sourceType string, sourceID, requestID uuid.UUID, idempotencyKey, reason string) (RfxRelationshipCancelResult, error) {
	if s == nil || s.reader == nil || s.client == nil {
		return RfxRelationshipCancelResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	key := strings.TrimSpace(idempotencyKey)
	if actor.UserID == uuid.Nil || actor.TenantID == uuid.Nil || sourceID == uuid.Nil || requestID == uuid.Nil || key == "" || len(key) > 200 || len(strings.TrimSpace(reason)) > 500 {
		return RfxRelationshipCancelResult{}, apperrors.Validation("relationship request is invalid", nil)
	}
	if err := s.requireVisibleSource(ctx, actor, sourceType, sourceID); err != nil {
		return RfxRelationshipCancelResult{}, err
	}
	return s.client.Cancel(ctx, sourceType, sourceID, actor.TenantID, actor.UserID, requestID, key, strings.TrimSpace(reason))
}

func (s *RfxRelationshipCreateInitiator) requireVisibleSource(ctx context.Context, actor RfxRelationshipActor, sourceType string, sourceID uuid.UUID) error {
	switch sourceType {
	case domain.RfxSourceTypeAward:
		row, err := s.reader.ReadAwardSource(ctx, sourceID)
		if err != nil {
			if errors.Is(err, domain.ErrRfxSourceMissing) {
				return apperrors.NotFound("rfx award not found")
			}
			return apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
		}
		if row.ID != sourceID || row.TenantID != actor.TenantID {
			return apperrors.NotFound("rfx award not found")
		}
	case domain.RfxSourceTypeAwardLink:
		row, err := s.reader.ReadAwardLinkSource(ctx, sourceID)
		if err != nil {
			if errors.Is(err, domain.ErrRfxSourceMissing) {
				return apperrors.NotFound("rfx award link not found")
			}
			return apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
		}
		if row.ID != sourceID || row.TenantID != actor.TenantID {
			return apperrors.NotFound("rfx award link not found")
		}
	default:
		return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	return nil
}

func rfxRelationshipDecisionError(decision string) error {
	switch decision {
	case domain.RfxSourceCreateConfirmed:
		return nil
	case domain.RfxSourceCreateNotFound, domain.RfxSourceCreateTenantMismatch:
		return apperrors.NotFound("rfx source not found")
	case domain.RfxSourceCreateStale:
		return apperrors.New(apperrors.CodeRelationshipRequestStale, "relationship command refused")
	case domain.RfxSourceCreateActorMismatch:
		return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	default:
		return apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
}
