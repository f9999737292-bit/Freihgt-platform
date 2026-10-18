package sourceauth

import "testing"

func TestTokenMatchRejectsEmptyWrongAndDifferentLength(t *testing.T) {
	if tokenMatches("", "secret") || tokenMatches("secret", "") || tokenMatches("secret", "secret-extra") || tokenMatches("secret", "secre") {
		t.Fatal("empty, wrong, and different-length credentials must not match")
	}
	if !tokenMatches("secret-value", "secret-value") {
		t.Fatal("equal dedicated credentials must match")
	}
	if tokenMatches("secret-value", "secret-valuE") {
		t.Fatal("credential compare must reject a one-character difference")
	}
}
