package driver

import "testing"

func TestNormalizeDriverExceptionEventID(t *testing.T) {
	raw := "A1B2C3D4-E5F6-7890-ABCD-EF1234567890"
	got := normalizeDriverExceptionEventID(raw)
	want := "a1b2c3d4e5f67890abcdef1234567890"
	if got != want {
		t.Fatalf("normalizeDriverExceptionEventID(%q) = %q, want %q", raw, got, want)
	}
}
