package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
)

type fakeShipmentSource struct {
	row           domain.ShipmentSourceRecord
	missing       bool
	readErr       error
	actor         domain.ShipmentCreatorAuthority
	actorErr      error
	actorCalls    int
	carrierActive bool
	carrierErr    error
}

func (f *fakeShipmentSource) ReadShipmentSource(context.Context, uuid.UUID) (domain.ShipmentSourceRecord, error) {
	if f.readErr != nil {
		return domain.ShipmentSourceRecord{}, f.readErr
	}
	if f.missing {
		return domain.ShipmentSourceRecord{}, domain.ErrShipmentSourceMissing
	}
	return f.row, nil
}

func (f *fakeShipmentSource) ReadShipmentCreator(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.ShipmentCreatorAuthority, error) {
	f.actorCalls++
	if f.actorErr != nil {
		return domain.ShipmentCreatorAuthority{}, f.actorErr
	}
	return f.actor, nil
}

func (f *fakeShipmentSource) ActiveCarrierInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	if f.carrierErr != nil {
		return false, f.carrierErr
	}
	return f.carrierActive, nil
}

func liveRow() domain.ShipmentSourceRecord {
	return domain.ShipmentSourceRecord{
		TenantID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ID:               uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version:          4,
		ShipperCompanyID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		CarrierAssigned:  true,
		Live:             true,
	}
}

func confirmedSubject(t *testing.T, row domain.ShipmentSourceRecord) domain.ShipmentSourceSubject {
	t.Helper()
	fingerprint, err := domain.ShipmentSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	return domain.ShipmentSourceSubject{
		SourceType: domain.ShipmentSourceType, SourceID: row.ID, SourceTenantID: row.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID: row.ShipperCompanyID, RequestingUserID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
	}
}

func TestConfirmStaleAndUnavailable(t *testing.T) {
	row := liveRow()
	subject := confirmedSubject(t, row)
	reader := &fakeShipmentSource{
		row: row, carrierActive: true,
		actor: domain.ShipmentCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"SHIPPER_LOGIST"}},
	}
	authority := NewShipmentSourceAuthority(reader)
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateConfirmed {
		t.Fatalf("confirm = %s", got)
	}

	stale := subject
	stale.Fingerprint = strings.Repeat("ab", 32)
	if got := authority.Confirm(context.Background(), stale); got != domain.ShipmentSourceCreateStale {
		t.Fatalf("stale confirm = %s", got)
	}
	changedCarrier := subject
	changedCarrier.ShipperCarrierCompanyID = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	if got := authority.Confirm(context.Background(), changedCarrier); got != domain.ShipmentSourceCreateStale {
		t.Fatalf("carrier mismatch = %s", got)
	}

	unavailable := &fakeShipmentSource{readErr: errors.New("db down")}
	if got := NewShipmentSourceAuthority(unavailable).Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateUnavailable {
		t.Fatalf("unavailable confirm = %s", got)
	}
	reader.carrierErr = errors.New("company read failed")
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateUnavailable {
		t.Fatalf("carrier read failure = %s", got)
	}
	reader.carrierErr = nil
	reader.actorErr = errors.New("user read failed")
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateUnavailable {
		t.Fatalf("actor read failure = %s", got)
	}
}

func TestConfirmActorTenantAndMissing(t *testing.T) {
	row := liveRow()
	subject := confirmedSubject(t, row)
	reader := &fakeShipmentSource{
		row: row, carrierActive: true,
		actor: domain.ShipmentCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"CARRIER_ADMIN"}},
	}
	authority := NewShipmentSourceAuthority(reader)
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateActorMismatch {
		t.Fatalf("carrier admin = %s", got)
	}

	reader.actor = domain.ShipmentCreatorAuthority{Active: true, ActiveMembership: false, TenantGlobalPlatformAdmin: true}
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateActorMismatch {
		t.Fatalf("platform admin without membership = %s", got)
	}

	reader.actorCalls = 0
	otherCompany := subject
	otherCompany.RequestingCompanyID = uuid.MustParse("77777777-7777-7777-7777-777777777777")
	if got := authority.Confirm(context.Background(), otherCompany); got != domain.ShipmentSourceCreateActorMismatch {
		t.Fatalf("requesting company = %s", got)
	}
	if reader.actorCalls != 0 {
		t.Fatal("actor lookup must not run for a company other than the shipment shipper")
	}

	foreign := subject
	foreign.SourceTenantID = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	if got := authority.Confirm(context.Background(), foreign); got != domain.ShipmentSourceCreateTenantMismatch {
		t.Fatalf("tenant = %s", got)
	}

	reader.missing = true
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("missing = %s", got)
	}
	reader.missing = false
	reader.row.Live = false
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("deleted = %s", got)
	}
	reader.row.Live = true
	reader.carrierActive = false
	if got := authority.Confirm(context.Background(), subject); got != domain.ShipmentSourceCreateNotFound {
		t.Fatalf("inactive carrier = %s", got)
	}
}

func TestRevalidateStaleUnavailableAndChanged(t *testing.T) {
	row := liveRow()
	subject := confirmedSubject(t, row)
	snap := domain.ShipmentSourceSnapshot{
		SourceType: subject.SourceType, SourceID: subject.SourceID, SourceTenantID: subject.SourceTenantID,
		Fingerprint: subject.Fingerprint, ShipperCarrierCompanyID: subject.ShipperCarrierCompanyID,
	}
	reader := &fakeShipmentSource{row: row}
	authority := NewShipmentSourceAuthority(reader)
	if got := authority.Revalidate(context.Background(), snap); got != domain.ShipmentSourceMatch {
		t.Fatalf("match = %s", got)
	}

	reader.row.Version = 5
	if got := authority.Revalidate(context.Background(), snap); got != domain.ShipmentSourceChanged {
		t.Fatalf("stale = %s", got)
	}
	reader.row.Version = row.Version
	reader.row.Live = false
	if got := authority.Revalidate(context.Background(), snap); got != domain.ShipmentSourceChanged {
		t.Fatalf("deleted = %s", got)
	}
	reader.missing = true
	if got := authority.Revalidate(context.Background(), snap); got != domain.ShipmentSourceNotFound {
		t.Fatalf("missing = %s", got)
	}
	reader.missing = false
	reader.readErr = errors.New("timeout")
	if got := authority.Revalidate(context.Background(), snap); got != domain.ShipmentSourceUnavailable {
		t.Fatalf("unavailable = %s", got)
	}
	if got := (*ShipmentSourceAuthority)(nil).Revalidate(context.Background(), snap); got != domain.ShipmentSourceUnavailable {
		t.Fatalf("nil authority = %s", got)
	}
}
