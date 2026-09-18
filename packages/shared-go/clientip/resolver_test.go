package clientip

import (
	"net"
	"net/http/httptest"
	"testing"
)

func mustParseTrusted(t *testing.T, raw string) []*net.IPNet {
	t.Helper()
	out, err := ParseTrustedProxyCIDRs(raw)
	if err != nil {
		t.Fatalf("parse trusted: %v", err)
	}
	return out
}

func TestDirectClientIgnoresSpoofedForwardedHeaders(t *testing.T) {
	resolver := NewResolver(nil)
	req := httptest.NewRequest("POST", "/token", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.20")
	req.Header.Set("X-Real-IP", "198.51.100.21")

	ip, err := resolver.ClientIP(req)
	if err != nil {
		t.Fatalf("client ip: %v", err)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("ip=%q want direct remote addr", ip)
	}
}

func TestTrustedProxyUsesForwardedChain(t *testing.T) {
	resolver := NewResolver(mustParseTrusted(t, "127.0.0.1/32,10.0.0.0/8"))
	req := httptest.NewRequest("POST", "/token", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.5")

	ip, err := resolver.ClientIP(req)
	if err != nil {
		t.Fatalf("client ip: %v", err)
	}
	if ip != "203.0.113.10" {
		t.Fatalf("ip=%q want first untrusted hop", ip)
	}
}

func TestMalformedForwardedChainFailsClosed(t *testing.T) {
	resolver := NewResolver(mustParseTrusted(t, "127.0.0.1/32"))
	req := httptest.NewRequest("POST", "/token", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "not-an-ip")

	if _, err := resolver.ClientIP(req); err == nil {
		t.Fatal("expected malformed chain error")
	}
}

func TestTrustedProxyRealIPFallback(t *testing.T) {
	resolver := NewResolver(mustParseTrusted(t, "127.0.0.1/32"))
	req := httptest.NewRequest("POST", "/token", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Real-IP", "203.0.113.44")

	ip, err := resolver.ClientIP(req)
	if err != nil {
		t.Fatalf("client ip: %v", err)
	}
	if ip != "203.0.113.44" {
		t.Fatalf("ip=%q want x-real-ip", ip)
	}
}

func TestParseTrustedProxyCIDRsRejectsMalformed(t *testing.T) {
	if _, err := ParseTrustedProxyCIDRs("not-a-cidr"); err == nil {
		t.Fatal("expected parse error")
	}
}
