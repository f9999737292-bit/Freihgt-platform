package config

import (
	"strings"
	"testing"
)

func TestHTTPListenAddrDefaultsToUnspecifiedBind(t *testing.T) {
	addr, err := HTTPListenAddr("", 8085)
	if err != nil {
		t.Fatal(err)
	}
	if addr != ":8085" {
		t.Fatalf("addr %q", addr)
	}
}

func TestHTTPListenAddrLoopback(t *testing.T) {
	addr, err := HTTPListenAddr(" 127.0.0.1 ", 18085)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1:18085" {
		t.Fatalf("addr %q", addr)
	}
}

func TestHTTPListenAddrRejectsNonLoopback(t *testing.T) {
	const secret = "super-secret-value"
	for _, host := range []string{"0.0.0.0", "::", "localhost", "10.0.0.8", secret} {
		addr, err := HTTPListenAddr(host, 8085)
		if err == nil {
			t.Fatalf("host %q accepted as %q", host, addr)
		}
		if addr != "" {
			t.Fatalf("host %q returned %q", host, addr)
		}
		if strings.Contains(err.Error(), secret) && host != secret {
			t.Fatalf("error leaked credential material: %v", err)
		}
	}
}

func TestLoadRejectsWildcardListenHost(t *testing.T) {
	const secret = "super-secret-value"
	t.Setenv("SHIPMENT_HTTP_LISTEN_HOST", "0.0.0.0")
	t.Setenv("DATABASE_URL", "postgres://stand_user:"+secret+"@postgres:5432/bintrans_e2e_stand?sslmode=disable")
	t.Setenv("SHIPMENT_OUTBOX_ENABLED", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("accepted wildcard listen host")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("config error leaked credential material: %v", err)
	}
}
