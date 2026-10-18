package domain

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
)

func TestRelationshipAuthorizationMatrix(t *testing.T) {
	activeMember := ActorAuthority{Active: true, ActiveMembership: true}
	inactive := ActorAuthority{Active: false, ActiveMembership: true, CompanyRoles: []string{RoleShipperAdmin}}
	deletedLike := ActorAuthority{Active: false, ActiveMembership: true, TenantGlobalPlatformAdmin: true}
	memberOnly := activeMember
	platformWithoutMembership := ActorAuthority{Active: true, TenantGlobalPlatformAdmin: true}
	platformWithMembership := ActorAuthority{Active: true, ActiveMembership: true, TenantGlobalPlatformAdmin: true}
	shipperLogist := ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{RoleShipperLogist}}
	carrierAdmin := ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{RoleCarrierAdmin}}
	procurement := ActorAuthority{Active: true, ActiveMembership: true, CompanyRoles: []string{RoleProcurementManager}}

	if CanCreateRelationshipRequest(inactive, SourceTypeShipment) || CanAcceptRelationship(inactive) {
		t.Fatal("inactive user must not create or accept")
	}
	if CanCreateRelationshipRequest(deletedLike, SourceTypeShipment) || CanAcceptRelationship(deletedLike) {
		t.Fatal("deleted user must not create or accept")
	}
	if CanCreateRelationshipRequest(platformWithoutMembership, SourceTypeShipment) || CanAcceptRelationship(platformWithoutMembership) {
		t.Fatal("platform admin must not replace membership")
	}
	if CanAcceptRelationship(memberOnly) || CanAcceptRelationship(shipperLogist) {
		t.Fatal("ordinary membership must not accept")
	}
	if !CanCreateRelationshipRequest(platformWithMembership, SourceTypeShipment) || !CanAcceptRelationship(platformWithMembership) {
		t.Fatal("platform admin with membership satisfies the role alternative")
	}
	if !CanCreateRelationshipRequest(shipperLogist, SourceTypeShipment) || CanCreateRelationshipRequest(carrierAdmin, SourceTypeShipment) {
		t.Fatal("shipment create role mismatch")
	}
	if !CanCreateRelationshipRequest(procurement, SourceTypeRfxAward) || CanCreateRelationshipRequest(procurement, SourceTypeShipment) {
		t.Fatal("procurement role is bound to the RFx source")
	}
	if !CanAcceptRelationship(carrierAdmin) || CanCreateRelationshipRequest(carrierAdmin, SourceTypeAwardLink) {
		t.Fatal("carrier admin accepts and does not create")
	}
	if !CallerAllowsSource(CallerShipmentService, SourceTypeShipment) || CallerAllowsSource(CallerShipmentService, SourceTypeRfxAward) || CallerAllowsSource(CallerShipmentService, SourceTypeAwardLink) {
		t.Fatal("shipment identity is bound to shipment sources")
	}
	if CallerAllowsSource(CallerRfxService, SourceTypeShipment) || !CallerAllowsSource(CallerRfxService, SourceTypeRfxAward) || !CallerAllowsSource(CallerRfxService, SourceTypeAwardLink) {
		t.Fatal("rfx identity is bound to rfx sources")
	}
	if CallerAllowsSource("company-service", SourceTypeShipment) || CallerAllowsSource("", SourceTypeShipment) {
		t.Fatal("company-service is not a source owner")
	}
}

func TestAcceptanceTokenFingerprintDoesNotKeepTheRawToken(t *testing.T) {
	raw, err := NewAcceptanceToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 32 {
		t.Fatalf("token length %d", len(raw))
	}
	fingerprint := FingerprintAcceptanceToken("pepper-value", raw)
	if bytes.Equal(fingerprint, raw) || bytes.Contains(fingerprint, raw) {
		t.Fatal("fingerprint must not contain the raw token")
	}
	encoded := EncodeAcceptanceToken(raw)
	if bytes.Contains(fingerprint, []byte(encoded)) {
		t.Fatal("fingerprint must not contain the encoded token")
	}
	decoded, err := DecodeAcceptanceToken(encoded)
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatal("token did not round-trip")
	}
}

func TestCreateRequestHashChangesWithPayload(t *testing.T) {
	sourceID := uuid.New()
	carrierID := uuid.New()
	left := CreateRequestHash(SourceTypeShipment, sourceID, "v1", carrierID)
	right := CreateRequestHash(SourceTypeShipment, sourceID, "v2", carrierID)
	if bytes.Equal(left, right) {
		t.Fatal("fingerprint change must change the request hash")
	}
	again := CreateRequestHash(SourceTypeShipment, sourceID, "v1", carrierID)
	if !bytes.Equal(left, again) {
		t.Fatal("request hash is not stable")
	}
}
