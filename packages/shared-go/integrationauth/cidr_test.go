package integrationauth

import "testing"

func TestMatchAllowedCIDRsEmptyAllowlist(t *testing.T) {
	ok, err := MatchAllowedCIDRs(nil, "127.0.0.1")
	if err != nil || !ok {
		t.Fatalf("expected allow without allowlist got ok=%v err=%v", ok, err)
	}
}

func TestMatchAllowedCIDRsDeny(t *testing.T) {
	ok, err := MatchAllowedCIDRs([]byte(`["203.0.113.0/24"]`), "198.51.100.1")
	if err != nil || ok {
		t.Fatalf("expected deny got ok=%v err=%v", ok, err)
	}
}

func TestMatchAllowedCIDRsMalformedFailClosed(t *testing.T) {
	_, err := MatchAllowedCIDRs([]byte(`["not-a-cidr"]`), "127.0.0.1")
	if err == nil {
		t.Fatal("expected malformed cidr error")
	}
}
