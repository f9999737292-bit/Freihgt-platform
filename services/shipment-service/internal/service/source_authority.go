package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

// ShipmentSourceReader loads the shipment row and the shipper-side creator proof.
// Implementations must not write relationship rows.
type ShipmentSourceReader interface {
	ReadShipmentSource(ctx context.Context, id uuid.UUID) (domain.ShipmentSourceRecord, error)
	ReadShipmentCreator(ctx context.Context, tenantID, userID, companyID uuid.UUID) (domain.ShipmentCreatorAuthority, error)
	ActiveCarrierInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error)
}

// ShipmentSourceAuthority answers company-service create confirmation and acceptance revalidation.
type ShipmentSourceAuthority struct {
	reader ShipmentSourceReader
}

func NewShipmentSourceAuthority(reader ShipmentSourceReader) *ShipmentSourceAuthority {
	return &ShipmentSourceAuthority{reader: reader}
}

func (s *ShipmentSourceAuthority) Confirm(ctx context.Context, subject domain.ShipmentSourceSubject) string {
	if s == nil || s.reader == nil {
		return domain.ShipmentSourceCreateUnavailable
	}
	if subject.SourceType != domain.ShipmentSourceType || subject.SourceID == uuid.Nil {
		return domain.ShipmentSourceCreateNotFound
	}
	row, ok := s.load(ctx, subject.SourceID)
	if !ok {
		return domain.ShipmentSourceCreateUnavailable
	}
	if row.missing {
		return domain.ShipmentSourceCreateNotFound
	}
	if row.record.TenantID != subject.SourceTenantID {
		return domain.ShipmentSourceCreateTenantMismatch
	}
	if !row.record.Live || !row.record.CarrierAssigned {
		return domain.ShipmentSourceCreateNotFound
	}
	if row.record.CarrierCompanyID != subject.ShipperCarrierCompanyID {
		return domain.ShipmentSourceCreateStale
	}
	fingerprint, err := domain.ShipmentSourceFingerprint(row.record)
	if err != nil || !domain.ShipmentSourceFingerprintMatch(fingerprint, subject.Fingerprint) {
		return domain.ShipmentSourceCreateStale
	}
	active, err := s.reader.ActiveCarrierInTenant(ctx, row.record.TenantID, row.record.CarrierCompanyID)
	if err != nil {
		return domain.ShipmentSourceCreateUnavailable
	}
	if !active {
		return domain.ShipmentSourceCreateNotFound
	}
	if subject.RequestingCompanyID != row.record.ShipperCompanyID || subject.RequestingUserID == uuid.Nil {
		return domain.ShipmentSourceCreateActorMismatch
	}
	actor, err := s.reader.ReadShipmentCreator(ctx, row.record.TenantID, subject.RequestingUserID, row.record.ShipperCompanyID)
	if err != nil {
		return domain.ShipmentSourceCreateUnavailable
	}
	if !domain.ShipmentCreatorAuthorized(actor) {
		return domain.ShipmentSourceCreateActorMismatch
	}
	return domain.ShipmentSourceCreateConfirmed
}

func (s *ShipmentSourceAuthority) Revalidate(ctx context.Context, snap domain.ShipmentSourceSnapshot) string {
	if s == nil || s.reader == nil {
		return domain.ShipmentSourceUnavailable
	}
	if snap.SourceType != domain.ShipmentSourceType || snap.SourceID == uuid.Nil {
		return domain.ShipmentSourceNotFound
	}
	row, ok := s.load(ctx, snap.SourceID)
	if !ok {
		return domain.ShipmentSourceUnavailable
	}
	if row.missing || row.record.TenantID != snap.SourceTenantID {
		return domain.ShipmentSourceNotFound
	}
	if !row.record.Live || !row.record.CarrierAssigned || row.record.CarrierCompanyID != snap.ShipperCarrierCompanyID {
		return domain.ShipmentSourceChanged
	}
	fingerprint, err := domain.ShipmentSourceFingerprint(row.record)
	if err != nil || !domain.ShipmentSourceFingerprintMatch(fingerprint, snap.Fingerprint) {
		return domain.ShipmentSourceChanged
	}
	return domain.ShipmentSourceMatch
}

type loadedShipment struct {
	record  domain.ShipmentSourceRecord
	missing bool
}

func (s *ShipmentSourceAuthority) load(ctx context.Context, id uuid.UUID) (loadedShipment, bool) {
	row, err := s.reader.ReadShipmentSource(ctx, id)
	if errors.Is(err, domain.ErrShipmentSourceMissing) {
		return loadedShipment{missing: true}, true
	}
	if err != nil {
		return loadedShipment{}, false
	}
	return loadedShipment{record: row}, true
}
