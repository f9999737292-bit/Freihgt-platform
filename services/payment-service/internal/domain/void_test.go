package domain

import "testing"

func TestValidateVoidReason(t *testing.T) {
	t.Parallel()
	if _, err := ValidateVoidReason("  "); err == nil {
		t.Fatal("blank reason must be rejected")
	}
	if _, err := ValidateVoidReason(string(make([]byte, 256))); err == nil {
		t.Fatal("long reason must be rejected")
	}
	got, err := ValidateVoidReason("  duplicate allocation  ")
	if err != nil || got != "duplicate allocation" {
		t.Fatalf("expected trimmed reason, got %q err=%v", got, err)
	}
}
