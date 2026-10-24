package config

import (
	"strings"
	"testing"
)

func TestHTTPListenAddrDefaultsToUnspecifiedBind(t *testing.T) {
	addr, err := HTTPListenAddr("", 8089)
	if err != nil {
		t.Fatal(err)
	}
	if addr != ":8089" {
		t.Fatalf("addr %q", addr)
	}
}

func TestHTTPListenAddrLoopback(t *testing.T) {
	addr, err := HTTPListenAddr("127.0.0.1", 18089)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1:18089" {
		t.Fatalf("addr %q", addr)
	}
}

func TestHTTPListenAddrRejectsNonLoopback(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "::", "localhost", "10.0.0.8"} {
		addr, err := HTTPListenAddr(host, 8089)
		if err == nil {
			t.Fatalf("host %q accepted as %q", host, addr)
		}
		if addr != "" {
			t.Fatalf("host %q returned %q", host, addr)
		}
	}
}

func TestLoadRejectsWildcardListenHost(t *testing.T) {
	const secret = "super-secret-value"
	t.Setenv("CONTROL_TOWER_HTTP_LISTEN_HOST", "0.0.0.0")
	t.Setenv("CONTROL_TOWER_DATABASE_URL", "postgres://stand_user:"+secret+"@postgres:5432/bintrans_e2e_stand?sslmode=disable")
	t.Setenv("CONTROL_TOWER_CONSUMER_ENABLED", "false")
	t.Setenv("CONTROL_TOWER_DRIVER_EVENTS_ENABLED", "false")
	_, err := Load()
	if err == nil {
		t.Fatal("accepted wildcard listen host")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("config error leaked credential material: %v", err)
	}
}

func TestLoadLoopbackListenHost(t *testing.T) {
	t.Setenv("CONTROL_TOWER_HTTP_LISTEN_HOST", "127.0.0.1")
	t.Setenv("CONTROL_TOWER_HTTP_PORT", "18089")
	t.Setenv("CONTROL_TOWER_DATABASE_URL", "postgres://stand_user:stand-pass@postgres:5432/bintrans_e2e_stand?sslmode=disable")
	t.Setenv("CONTROL_TOWER_CONSUMER_ENABLED", "false")
	t.Setenv("CONTROL_TOWER_DRIVER_EVENTS_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	addr, err := HTTPListenAddr(cfg.HTTPListenHost, cfg.HTTPPort)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1:18089" {
		t.Fatalf("addr %q", addr)
	}
}
