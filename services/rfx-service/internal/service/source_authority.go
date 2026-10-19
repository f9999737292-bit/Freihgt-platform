package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

// RfxSourceReader loads award and award-link rows and the buyer-side creator proof.
// Implementations must not write relationship rows and must not read company.carrier_relationship*.
type RfxSourceReader interface {
	ReadAwardSource(ctx context.Context, id uuid.UUID) (domain.RfxAwardSourceRecord, error)
	ReadAwardLinkSource(ctx context.Context, id uuid.UUID) (domain.RfxAwardLinkSourceRecord, error)
	ReadRfxCreator(ctx context.Context, tenantID, userID, companyID uuid.UUID) (domain.RfxCreatorAuthority, error)
	ActiveCarrierInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error)
}

// RfxSourceAuthority answers company-service create confirmation and acceptance revalidation
// for RFX_AWARD and AWARD_TRANSPORT_ORDER_LINK.
type RfxSourceAuthority struct {
	reader RfxSourceReader
}

func NewRfxSourceAuthority(reader RfxSourceReader) *RfxSourceAuthority {
	return &RfxSourceAuthority{reader: reader}
}

func (s *RfxSourceAuthority) Confirm(ctx context.Context, subject domain.RfxSourceSubject) string {
	if s == nil || s.reader == nil {
		return domain.RfxSourceCreateUnavailable
	}
	switch subject.SourceType {
	case domain.RfxSourceTypeAward:
		return s.confirmAward(ctx, subject)
	case domain.RfxSourceTypeAwardLink:
		return s.confirmLink(ctx, subject)
	default:
		return domain.RfxSourceCreateNotFound
	}
}

func (s *RfxSourceAuthority) Revalidate(ctx context.Context, snap domain.RfxSourceSnapshot) string {
	if s == nil || s.reader == nil {
		return domain.RfxSourceUnavailable
	}
	switch snap.SourceType {
	case domain.RfxSourceTypeAward:
		return s.revalidateAward(ctx, snap)
	case domain.RfxSourceTypeAwardLink:
		return s.revalidateLink(ctx, snap)
	default:
		return domain.RfxSourceNotFound
	}
}

func (s *RfxSourceAuthority) confirmAward(ctx context.Context, subject domain.RfxSourceSubject) string {
	if subject.SourceID == uuid.Nil {
		return domain.RfxSourceCreateNotFound
	}
	row, missing, ok := s.loadAward(ctx, subject.SourceID)
	if !ok {
		return domain.RfxSourceCreateUnavailable
	}
	if missing {
		return domain.RfxSourceCreateNotFound
	}
	if row.TenantID != subject.SourceTenantID {
		return domain.RfxSourceCreateTenantMismatch
	}
	if !domain.AwardSourceAcceptable(row) {
		return domain.RfxSourceCreateNotFound
	}
	if row.CarrierCompanyID != subject.ShipperCarrierCompanyID {
		return domain.RfxSourceCreateStale
	}
	fingerprint, err := domain.RfxAwardSourceFingerprint(row)
	if err != nil || !domain.RfxSourceFingerprintMatch(fingerprint, subject.Fingerprint) {
		return domain.RfxSourceCreateStale
	}
	return s.confirmActor(ctx, row.TenantID, row.OwnerCompanyID, row.CarrierCompanyID, subject)
}

func (s *RfxSourceAuthority) confirmLink(ctx context.Context, subject domain.RfxSourceSubject) string {
	if subject.SourceID == uuid.Nil {
		return domain.RfxSourceCreateNotFound
	}
	row, missing, ok := s.loadLink(ctx, subject.SourceID)
	if !ok {
		return domain.RfxSourceCreateUnavailable
	}
	if missing {
		return domain.RfxSourceCreateNotFound
	}
	if row.TenantID != subject.SourceTenantID {
		return domain.RfxSourceCreateTenantMismatch
	}
	if !domain.AwardLinkSourceAcceptable(row) {
		return domain.RfxSourceCreateNotFound
	}
	if row.CarrierCompanyID != subject.ShipperCarrierCompanyID {
		return domain.RfxSourceCreateStale
	}
	fingerprint, err := domain.RfxAwardLinkSourceFingerprint(row)
	if err != nil || !domain.RfxSourceFingerprintMatch(fingerprint, subject.Fingerprint) {
		return domain.RfxSourceCreateStale
	}
	return s.confirmActor(ctx, row.TenantID, row.OwnerCompanyID, row.CarrierCompanyID, subject)
}

func (s *RfxSourceAuthority) confirmActor(ctx context.Context, tenantID, ownerID, carrierID uuid.UUID, subject domain.RfxSourceSubject) string {
	active, err := s.reader.ActiveCarrierInTenant(ctx, tenantID, carrierID)
	if err != nil {
		return domain.RfxSourceCreateUnavailable
	}
	if !active {
		return domain.RfxSourceCreateNotFound
	}
	if subject.RequestingCompanyID != ownerID || subject.RequestingUserID == uuid.Nil {
		return domain.RfxSourceCreateActorMismatch
	}
	actor, err := s.reader.ReadRfxCreator(ctx, tenantID, subject.RequestingUserID, ownerID)
	if err != nil {
		return domain.RfxSourceCreateUnavailable
	}
	if !domain.RfxCreatorAuthorized(actor) {
		return domain.RfxSourceCreateActorMismatch
	}
	return domain.RfxSourceCreateConfirmed
}

func (s *RfxSourceAuthority) revalidateAward(ctx context.Context, snap domain.RfxSourceSnapshot) string {
	if snap.SourceID == uuid.Nil {
		return domain.RfxSourceNotFound
	}
	row, missing, ok := s.loadAward(ctx, snap.SourceID)
	if !ok {
		return domain.RfxSourceUnavailable
	}
	if missing || row.TenantID != snap.SourceTenantID {
		return domain.RfxSourceNotFound
	}
	if !domain.AwardSourceAcceptable(row) || row.CarrierCompanyID != snap.ShipperCarrierCompanyID {
		return domain.RfxSourceChanged
	}
	fingerprint, err := domain.RfxAwardSourceFingerprint(row)
	if err != nil || !domain.RfxSourceFingerprintMatch(fingerprint, snap.Fingerprint) {
		return domain.RfxSourceChanged
	}
	return domain.RfxSourceMatch
}

func (s *RfxSourceAuthority) revalidateLink(ctx context.Context, snap domain.RfxSourceSnapshot) string {
	if snap.SourceID == uuid.Nil {
		return domain.RfxSourceNotFound
	}
	row, missing, ok := s.loadLink(ctx, snap.SourceID)
	if !ok {
		return domain.RfxSourceUnavailable
	}
	if missing || row.TenantID != snap.SourceTenantID {
		return domain.RfxSourceNotFound
	}
	if !domain.AwardLinkSourceAcceptable(row) || row.CarrierCompanyID != snap.ShipperCarrierCompanyID {
		return domain.RfxSourceChanged
	}
	fingerprint, err := domain.RfxAwardLinkSourceFingerprint(row)
	if err != nil || !domain.RfxSourceFingerprintMatch(fingerprint, snap.Fingerprint) {
		return domain.RfxSourceChanged
	}
	return domain.RfxSourceMatch
}

func (s *RfxSourceAuthority) loadAward(ctx context.Context, id uuid.UUID) (domain.RfxAwardSourceRecord, bool, bool) {
	row, err := s.reader.ReadAwardSource(ctx, id)
	if errors.Is(err, domain.ErrRfxSourceMissing) {
		return domain.RfxAwardSourceRecord{}, true, true
	}
	if err != nil {
		return domain.RfxAwardSourceRecord{}, false, false
	}
	return row, false, true
}

func (s *RfxSourceAuthority) loadLink(ctx context.Context, id uuid.UUID) (domain.RfxAwardLinkSourceRecord, bool, bool) {
	row, err := s.reader.ReadAwardLinkSource(ctx, id)
	if errors.Is(err, domain.ErrRfxSourceMissing) {
		return domain.RfxAwardLinkSourceRecord{}, true, true
	}
	if err != nil {
		return domain.RfxAwardLinkSourceRecord{}, false, false
	}
	return row, false, true
}
