package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func TestShipmentSourceFingerprintCanonicalDocument(t *testing.T) {
	row := ShipmentSourceRecord{
		TenantID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ID:               uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version:          7,
		ShipperCompanyID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		CarrierAssigned:  true,
		Live:             true,
	}
	got, err := ShipmentSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"tenant_id":"11111111-1111-1111-1111-111111111111","id":"22222222-2222-2222-2222-222222222222","version":7,"shipper_company_id":"33333333-3333-3333-3333-333333333333","carrier_company_id":"44444444-4444-4444-4444-444444444444","deleted_at_null":true}`
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("fingerprint = %s, want %s", got, want)
	}
	if !ShipmentSourceFingerprintMatch(got, want) {
		t.Fatal("equal fingerprints must match")
	}
	row.Version = 8
	changed, err := ShipmentSourceFingerprint(row)
	if err != nil {
		t.Fatal(err)
	}
	if ShipmentSourceFingerprintMatch(changed, want) {
		t.Fatal("version change must not match the stored fingerprint")
	}
}

func TestShipmentSourceFingerprintRejectsUnassignedOrDeleted(t *testing.T) {
	row := ShipmentSourceRecord{
		TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Version: 1, ShipperCompanyID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CarrierCompanyID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), CarrierAssigned: true, Live: true,
	}
	deleted := row
	deleted.Live = false
	if _, err := ShipmentSourceFingerprint(deleted); err == nil {
		t.Fatal("deleted shipment must not produce a live fingerprint")
	}
	unassigned := row
	unassigned.CarrierAssigned = false
	unassigned.CarrierCompanyID = uuid.Nil
	if _, err := ShipmentSourceFingerprint(unassigned); err == nil {
		t.Fatal("null carrier must not produce a fingerprint")
	}
}

func TestShipmentCreatorAuthorized(t *testing.T) {
	base := ShipmentCreatorAuthority{Active: true, ActiveMembership: true}
	for _, code := range []string{"SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER"} {
		actor := base
		actor.CompanyRoles = []string{code}
		if !ShipmentCreatorAuthorized(actor) {
			t.Fatalf("%s must be a shipment creator role", code)
		}
	}
	for _, actor := range []ShipmentCreatorAuthority{
		{Active: true, ActiveMembership: true, CompanyRoles: []string{"CARRIER_ADMIN"}},
		{Active: true, ActiveMembership: true, CompanyRoles: []string{"CARRIER_DISPATCHER"}},
		{Active: true, ActiveMembership: true, CompanyRoles: []string{"PROCUREMENT_MANAGER"}},
		{Active: false, ActiveMembership: true, CompanyRoles: []string{"SHIPPER_ADMIN"}},
		{Active: true, ActiveMembership: false, TenantGlobalPlatformAdmin: true},
		{Active: true, ActiveMembership: false, CompanyRoles: []string{"SHIPPER_ADMIN"}},
	} {
		if ShipmentCreatorAuthorized(actor) {
			t.Fatalf("actor %+v must not create a shipment relationship request", actor)
		}
	}
	admin := base
	admin.TenantGlobalPlatformAdmin = true
	if !ShipmentCreatorAuthorized(admin) {
		t.Fatal("active member platform admin must pass the role alternative")
	}
}
