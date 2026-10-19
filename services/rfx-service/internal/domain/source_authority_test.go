package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func TestRfxAwardSourceFingerprintCanonicalDocument(t *testing.T) {
	row := awardedRow()
	got, err := RfxAwardSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"tenant_id":"11111111-1111-1111-1111-111111111111","id":"22222222-2222-2222-2222-222222222222","version":7,"rfx_event_id":"33333333-3333-3333-3333-333333333333","carrier_company_id":"44444444-4444-4444-4444-444444444444"}`
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
	if !RfxSourceFingerprintMatch(got, want) {
		t.Fatal("equal fingerprints must match")
	}
	row.Version = 8
	changed, err := RfxAwardSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	if RfxSourceFingerprintMatch(changed, want) {
		t.Fatal("version change must not match the stored fingerprint")
	}
}

func TestRfxAwardLinkSourceFingerprintCanonicalDocument(t *testing.T) {
	row := linkedRow()
	got, err := RfxAwardLinkSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"tenant_id":"11111111-1111-1111-1111-111111111111","id":"66666666-6666-6666-6666-666666666666","version":3,"rfx_award_id":"22222222-2222-2222-2222-222222222222","transport_order_id":"77777777-7777-7777-7777-777777777777","carrier_company_id":"44444444-4444-4444-4444-444444444444"}`
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
	row.TransportOrderID = uuid.MustParse("88888888-8888-8888-8888-888888888888")
	changed, err := RfxAwardLinkSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	if RfxSourceFingerprintMatch(changed, want) {
		t.Fatal("transport order change must not match the stored fingerprint")
	}
	row.TransportOrderID = uuid.Nil
	if _, err := RfxAwardLinkSourceFingerprint(row); err == nil {
		t.Fatal("missing transport order must not produce a fingerprint")
	}
}

func TestAwardSourceAcceptableRequiresAwardedLiveEvent(t *testing.T) {
	row := awardedRow()
	if !AwardSourceAcceptable(row) {
		t.Fatal("awarded live event must be acceptable")
	}
	for _, mutate := range []func(*RfxAwardSourceRecord){
		func(row *RfxAwardSourceRecord) { row.EventStatus = RfxStatusArchived },
		func(row *RfxAwardSourceRecord) { row.EventStatus = RfxStatusPartiallyAwarded },
		func(row *RfxAwardSourceRecord) { row.EventLive = false },
		func(row *RfxAwardSourceRecord) { row.EventPresent = false },
		func(row *RfxAwardSourceRecord) {
			row.EventTenantID = uuid.MustParse("99999999-9999-9999-9999-999999999999")
		},
		func(row *RfxAwardSourceRecord) { row.EventID = uuid.MustParse("99999999-9999-9999-9999-999999999999") },
		func(row *RfxAwardSourceRecord) { row.OwnerCompanyID = uuid.Nil },
	} {
		next := row
		mutate(&next)
		if AwardSourceAcceptable(next) {
			t.Fatalf("mutated award must not be acceptable: %+v", next)
		}
	}
}

func TestAwardLinkSourceAcceptableRequiresParentAwardAndOrder(t *testing.T) {
	row := linkedRow()
	if !AwardLinkSourceAcceptable(row) {
		t.Fatal("consistent awarded link must be acceptable")
	}
	for _, mutate := range []func(*RfxAwardLinkSourceRecord){
		func(row *RfxAwardLinkSourceRecord) { row.AwardPresent = false },
		func(row *RfxAwardLinkSourceRecord) {
			row.AwardTenantID = uuid.MustParse("99999999-9999-9999-9999-999999999999")
		},
		func(row *RfxAwardLinkSourceRecord) {
			row.AwardEventID = uuid.MustParse("99999999-9999-9999-9999-999999999999")
		},
		func(row *RfxAwardLinkSourceRecord) {
			row.AwardCarrierID = uuid.MustParse("99999999-9999-9999-9999-999999999999")
		},
		func(row *RfxAwardLinkSourceRecord) { row.EventStatus = RfxStatusArchived },
		func(row *RfxAwardLinkSourceRecord) { row.EventLive = false },
		func(row *RfxAwardLinkSourceRecord) {
			row.BuyerCompanyID = uuid.MustParse("99999999-9999-9999-9999-999999999999")
		},
		func(row *RfxAwardLinkSourceRecord) { row.TransportOrderID = uuid.Nil },
		func(row *RfxAwardLinkSourceRecord) {
			row.EventTenantID = uuid.MustParse("99999999-9999-9999-9999-999999999999")
		},
	} {
		next := row
		mutate(&next)
		if AwardLinkSourceAcceptable(next) {
			t.Fatalf("mutated link must not be acceptable: %+v", next)
		}
	}
}

func TestRfxCreatorAuthorized(t *testing.T) {
	base := RfxCreatorAuthority{Active: true, ActiveMembership: true}
	for _, code := range []string{"PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER"} {
		actor := base
		actor.CompanyRoles = []string{code}
		if !RfxCreatorAuthorized(actor) {
			t.Fatalf("%s must be an RFx source creator role", code)
		}
	}
	for _, actor := range []RfxCreatorAuthority{
		{Active: true, ActiveMembership: true, CompanyRoles: []string{"CARRIER_ADMIN"}},
		{Active: true, ActiveMembership: true, CompanyRoles: []string{"CARRIER_DISPATCHER"}},
		{Active: true, ActiveMembership: true, CompanyRoles: []string{"PLATFORM_ADMIN"}},
		{Active: false, ActiveMembership: true, CompanyRoles: []string{"PROCUREMENT_MANAGER"}},
		{Active: true, ActiveMembership: false, TenantGlobalPlatformAdmin: true},
		{Active: true, ActiveMembership: false, CompanyRoles: []string{"SHIPPER_ADMIN"}},
	} {
		if RfxCreatorAuthorized(actor) {
			t.Fatalf("actor %+v must not create an RFx relationship request", actor)
		}
	}
	admin := base
	admin.TenantGlobalPlatformAdmin = true
	if !RfxCreatorAuthorized(admin) {
		t.Fatal("active member platform admin must pass the role alternative")
	}
}

func awardedRow() RfxAwardSourceRecord {
	return RfxAwardSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version: 7, RfxEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		EventPresent:     true, EventTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), OwnerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		EventStatus: RfxStatusAwarded, EventLive: true,
	}
}

func linkedRow() RfxAwardLinkSourceRecord {
	return RfxAwardLinkSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		Version: 3, RfxAwardID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		RfxEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), TransportOrderID: uuid.MustParse("77777777-7777-7777-7777-777777777777"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), BuyerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		AwardPresent: true, AwardTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		AwardEventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), AwardCarrierID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		EventPresent: true, EventTenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		EventID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), OwnerCompanyID: uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		EventStatus: RfxStatusAwarded, EventLive: true,
	}
}
