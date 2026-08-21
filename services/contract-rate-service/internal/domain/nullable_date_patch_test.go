package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNullableDatePatchOmitted(t *testing.T) {
	current := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	ptr := &current
	patch, err := ParseNullableDatePatch(nil, "valid_to")
	if err != nil {
		t.Fatal(err)
	}
	got := ApplyNullableDatePatch(ptr, patch)
	if got == nil || !got.Equal(current) {
		t.Fatalf("omitted patch should preserve current value")
	}
}

func TestNullableDatePatchNullClears(t *testing.T) {
	current := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	ptr := &current
	patch, err := ParseNullableDatePatch(json.RawMessage("null"), "valid_to")
	if err != nil {
		t.Fatal(err)
	}
	got := ApplyNullableDatePatch(ptr, patch)
	if got != nil {
		t.Fatalf("null patch should clear valid_to")
	}
}

func TestNullableDatePatchDateSets(t *testing.T) {
	current := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	ptr := &current
	patch, err := ParseNullableDatePatch(json.RawMessage(`"2027-12-31"`), "valid_to")
	if err != nil {
		t.Fatal(err)
	}
	got := ApplyNullableDatePatch(ptr, patch)
	if got == nil || got.Format("2006-01-02") != "2027-12-31" {
		t.Fatalf("date patch should set valid_to, got %v", got)
	}
}
