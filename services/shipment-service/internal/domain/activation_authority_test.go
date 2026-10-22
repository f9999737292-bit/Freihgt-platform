package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPinSetFingerprintSortsSubjects(t *testing.T) {
	operating := uuid.MustParse("00000000-0000-4000-8000-000000000010")
	carrier := uuid.MustParse("00000000-0000-4000-8000-000000000020")
	first := ActivationPinSubject{
		ShipmentTenantID: uuid.MustParse("00000000-0000-4000-8000-0000000000b0"),
		ShipmentID:       uuid.MustParse("00000000-0000-4000-8000-0000000000b2"),
		SourceCompanyID:  uuid.MustParse("00000000-0000-4000-8000-0000000000b3"),
	}
	second := ActivationPinSubject{
		ShipmentTenantID: uuid.MustParse("00000000-0000-4000-8000-0000000000a0"),
		ShipmentID:       uuid.MustParse("00000000-0000-4000-8000-0000000000a2"),
		SourceCompanyID:  uuid.MustParse("00000000-0000-4000-8000-0000000000a3"),
	}
	left, err := PinSetFingerprint(operating, carrier, []ActivationPinSubject{first, second})
	if err != nil {
		t.Fatal(err)
	}
	right, err := PinSetFingerprint(operating, carrier, []ActivationPinSubject{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if left != right || len(left) != 64 || strings.ToLower(left) != left {
		t.Fatalf("fingerprint=%s swapped=%s", left, right)
	}
}

func TestJSONContainsRelationshipAuthority(t *testing.T) {
	if !JSONContainsRelationshipAuthority([]byte(`{"relationship_version_id":"00000000-0000-4000-8000-000000000001"}`)) {
		t.Fatal("top-level version was accepted")
	}
	if !JSONContainsRelationshipAuthority([]byte(`{"execution_subjects":[{"relationship_id":"00000000-0000-4000-8000-000000000001"}]}`)) {
		t.Fatal("nested relationship id was accepted")
	}
	if JSONContainsRelationshipAuthority([]byte(`{"activation_id":"00000000-0000-4000-8000-000000000001"}`)) {
		t.Fatal("activation id was treated as relationship authority")
	}
}
