package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type fakeRfxSource struct {
	award         domain.RfxAwardSourceRecord
	awardMissing  bool
	awardErr      error
	link          domain.RfxAwardLinkSourceRecord
	linkMissing   bool
	linkErr       error
	actor         domain.RfxCreatorAuthority
	actorErr      error
	actorCalls    int
	actorCompany  uuid.UUID
	carrierActive bool
	carrierErr    error
}

func (f *fakeRfxSource) ReadAwardSource(context.Context, uuid.UUID) (domain.RfxAwardSourceRecord, error) {
	if f.awardErr != nil {
		return domain.RfxAwardSourceRecord{}, f.awardErr
	}
	if f.awardMissing {
		return domain.RfxAwardSourceRecord{}, domain.ErrRfxSourceMissing
	}
	return f.award, nil
}

func (f *fakeRfxSource) ReadAwardLinkSource(context.Context, uuid.UUID) (domain.RfxAwardLinkSourceRecord, error) {
	if f.linkErr != nil {
		return domain.RfxAwardLinkSourceRecord{}, f.linkErr
	}
	if f.linkMissing {
		return domain.RfxAwardLinkSourceRecord{}, domain.ErrRfxSourceMissing
	}
	return f.link, nil
}

func (f *fakeRfxSource) ReadRfxCreator(_ context.Context, _, _ uuid.UUID, companyID uuid.UUID) (domain.RfxCreatorAuthority, error) {
	f.actorCalls++
	f.actorCompany = companyID
	if f.actorErr != nil {
		return domain.RfxCreatorAuthority{}, f.actorErr
	}
	return f.actor, nil
}

func (f *fakeRfxSource) ActiveCarrierInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	if f.carrierErr != nil {
		return false, f.carrierErr
	}
	return f.carrierActive, nil
}

func awardSubject(t *testing.T, row domain.RfxAwardSourceRecord) domain.RfxSourceSubject {
	t.Helper()
	fingerprint, err := domain.RfxAwardSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	return domain.RfxSourceSubject{
		SourceType: domain.RfxSourceTypeAward, SourceID: row.ID, SourceTenantID: row.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID: row.OwnerCompanyID, RequestingUserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
	}
}

func linkSubject(t *testing.T, row domain.RfxAwardLinkSourceRecord) domain.RfxSourceSubject {
	t.Helper()
	fingerprint, err := domain.RfxAwardLinkSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	return domain.RfxSourceSubject{
		SourceType: domain.RfxSourceTypeAwardLink, SourceID: row.ID, SourceTenantID: row.TenantID,
		Fingerprint: fingerprint, ShipperCarrierCompanyID: row.CarrierCompanyID,
		RequestingCompanyID: row.OwnerCompanyID, RequestingUserID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
	}
}

func buyerActor() domain.RfxCreatorAuthority {
	return domain.RfxCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"PROCUREMENT_MANAGER"}}
}

func TestConfirmAwardStaleUnavailableAndActor(t *testing.T) {
	row := awardedServiceRow()
	subject := awardSubject(t, row)
	reader := &fakeRfxSource{award: row, carrierActive: true, actor: buyerActor()}
	authority := NewRfxSourceAuthority(reader)
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateConfirmed {
		t.Fatalf("confirm = %s", got)
	}
	if reader.actorCompany != row.OwnerCompanyID {
		t.Fatal("creator proof must use the event owner company")
	}

	stale := subject
	stale.Fingerprint = strings.Repeat("ab", 32)
	if got := authority.Confirm(context.Background(), stale); got != domain.RfxSourceCreateStale {
		t.Fatalf("stale confirm = %s", got)
	}
	changedCarrier := subject
	changedCarrier.ShipperCarrierCompanyID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if got := authority.Confirm(context.Background(), changedCarrier); got != domain.RfxSourceCreateStale {
		t.Fatalf("carrier mismatch = %s", got)
	}

	reader.award.EventStatus = domain.RfxStatusArchived
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("archived event = %s", got)
	}
	reader.award.EventStatus = domain.RfxStatusAwarded
	reader.award.EventLive = false
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("deleted event = %s", got)
	}
	reader.award.EventLive = true

	foreign := subject
	foreign.SourceTenantID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	if got := authority.Confirm(context.Background(), foreign); got != domain.RfxSourceCreateTenantMismatch {
		t.Fatalf("tenant = %s", got)
	}

	reader.actorCalls = 0
	otherCompany := subject
	otherCompany.RequestingCompanyID = uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	if got := authority.Confirm(context.Background(), otherCompany); got != domain.RfxSourceCreateActorMismatch {
		t.Fatalf("requesting company = %s", got)
	}
	if reader.actorCalls != 0 {
		t.Fatal("actor lookup must not run for a company other than the event owner")
	}

	reader.actor = domain.RfxCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"CARRIER_ADMIN"}}
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateActorMismatch {
		t.Fatalf("carrier admin = %s", got)
	}
	reader.actor = domain.RfxCreatorAuthority{Active: true, ActiveMembership: false, TenantGlobalPlatformAdmin: true}
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateActorMismatch {
		t.Fatalf("platform admin without membership = %s", got)
	}
	reader.actor = domain.RfxCreatorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{"PLATFORM_ADMIN"}}
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateActorMismatch {
		t.Fatalf("company-scoped platform admin = %s", got)
	}
	reader.actor = domain.RfxCreatorAuthority{Active: true, ActiveMembership: true, TenantGlobalPlatformAdmin: true}
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateConfirmed {
		t.Fatalf("tenant-global platform admin = %s", got)
	}

	reader.carrierActive = false
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("inactive carrier = %s", got)
	}
	reader.carrierErr = errors.New("company read failed")
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateUnavailable {
		t.Fatalf("carrier read failure = %s", got)
	}
	reader.carrierErr = nil
	reader.carrierActive = true
	reader.actorErr = errors.New("user read failed")
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateUnavailable {
		t.Fatalf("actor read failure = %s", got)
	}
	reader.awardErr = errors.New("db down")
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateUnavailable {
		t.Fatalf("unavailable confirm = %s", got)
	}
	reader.awardErr = nil
	reader.awardMissing = true
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("missing = %s", got)
	}
}

func TestConfirmAwardLinkParentAndOrder(t *testing.T) {
	row := linkedServiceRow()
	subject := linkSubject(t, row)
	reader := &fakeRfxSource{link: row, carrierActive: true, actor: buyerActor()}
	authority := NewRfxSourceAuthority(reader)
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateConfirmed {
		t.Fatalf("confirm = %s", got)
	}

	reader.link.AwardPresent = false
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("missing parent award = %s", got)
	}
	reader.link.AwardPresent = true
	reader.link.AwardCarrierID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("parent carrier mismatch = %s", got)
	}
	reader.link.AwardCarrierID = row.AwardCarrierID
	reader.link.EventStatus = domain.RfxStatusAwarded
	reader.link.BuyerCompanyID = uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("buyer is not the event owner = %s", got)
	}
	reader.link.BuyerCompanyID = row.BuyerCompanyID
	reader.link.AwardTenantID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	if got := authority.Confirm(context.Background(), subject); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("parent tenant mismatch = %s", got)
	}

	shipment := subject
	shipment.SourceType = domain.RfxSourceTypeAward
	reader.linkMissing = false
	reader.awardMissing = true
	if got := authority.Confirm(context.Background(), shipment); got != domain.RfxSourceCreateNotFound {
		t.Fatalf("award type must not read a link id as an award, got %s", got)
	}
}

func TestRevalidateAwardAndLinkFreshness(t *testing.T) {
	award := awardedServiceRow()
	awardSubject := awardSubject(t, award)
	awardSnap := domain.RfxSourceSnapshot{
		SourceType: awardSubject.SourceType, SourceID: awardSubject.SourceID, SourceTenantID: awardSubject.SourceTenantID,
		Fingerprint: awardSubject.Fingerprint, ShipperCarrierCompanyID: awardSubject.ShipperCarrierCompanyID,
	}
	link := linkedServiceRow()
	linkSubject := linkSubject(t, link)
	linkSnap := domain.RfxSourceSnapshot{
		SourceType: linkSubject.SourceType, SourceID: linkSubject.SourceID, SourceTenantID: linkSubject.SourceTenantID,
		Fingerprint: linkSubject.Fingerprint, ShipperCarrierCompanyID: linkSubject.ShipperCarrierCompanyID,
	}
	reader := &fakeRfxSource{award: award, link: link}
	authority := NewRfxSourceAuthority(reader)
	if got := authority.Revalidate(context.Background(), awardSnap); got != domain.RfxSourceMatch {
		t.Fatalf("award match = %s", got)
	}
	if got := authority.Revalidate(context.Background(), linkSnap); got != domain.RfxSourceMatch {
		t.Fatalf("link match = %s", got)
	}

	reader.award.Version = 9
	if got := authority.Revalidate(context.Background(), awardSnap); got != domain.RfxSourceChanged {
		t.Fatalf("award version = %s", got)
	}
	reader.award.Version = award.Version
	reader.award.EventStatus = domain.RfxStatusArchived
	if got := authority.Revalidate(context.Background(), awardSnap); got != domain.RfxSourceChanged {
		t.Fatalf("archived award = %s", got)
	}
	reader.awardMissing = true
	if got := authority.Revalidate(context.Background(), awardSnap); got != domain.RfxSourceNotFound {
		t.Fatalf("missing award = %s", got)
	}

	reader.link.TransportOrderID = uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	if got := authority.Revalidate(context.Background(), linkSnap); got != domain.RfxSourceChanged {
		t.Fatalf("transport order change = %s", got)
	}
	reader.link.TransportOrderID = link.TransportOrderID
	reader.link.EventLive = false
	if got := authority.Revalidate(context.Background(), linkSnap); got != domain.RfxSourceChanged {
		t.Fatalf("deleted parent event = %s", got)
	}
	reader.linkErr = errors.New("timeout")
	if got := authority.Revalidate(context.Background(), linkSnap); got != domain.RfxSourceUnavailable {
		t.Fatalf("unavailable = %s", got)
	}
	if got := (*RfxSourceAuthority)(nil).Revalidate(context.Background(), awardSnap); got != domain.RfxSourceUnavailable {
		t.Fatalf("nil authority = %s", got)
	}
}

func awardedServiceRow() domain.RfxAwardSourceRecord {
	return domain.RfxAwardSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version: 4, RfxEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		EventPresent:     true, EventTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), OwnerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		EventStatus: domain.RfxStatusAwarded, EventLive: true,
	}
}

func linkedServiceRow() domain.RfxAwardLinkSourceRecord {
	return domain.RfxAwardLinkSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		Version: 2, RfxAwardID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		RfxEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), TransportOrderID: uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), BuyerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		AwardPresent: true, AwardTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		AwardEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), AwardCarrierID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		EventPresent: true, EventTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), OwnerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		EventStatus: domain.RfxStatusAwarded, EventLive: true,
	}
}
