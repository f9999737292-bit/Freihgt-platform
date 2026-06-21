package security

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("StrongPassword123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "StrongPassword123!" {
		t.Fatalf("hash must not equal plain password")
	}
	if !VerifyPassword(hash, "StrongPassword123!") {
		t.Fatalf("expected password verification to succeed")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatalf("expected password verification to fail")
	}
}
