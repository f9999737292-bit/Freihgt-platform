package bintranse2e

import (
	"errors"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kfake"
)

func TestBrokerLogLinesRecordEmptyBroker(t *testing.T) {
	modeLine, brokerLine := brokerLogLines("stand", "")
	if modeLine != "bintrans_e2e_mode=stand" {
		t.Fatalf("mode line %q", modeLine)
	}
	if brokerLine != "bintrans_e2e_broker=" {
		t.Fatalf("broker line %q", brokerLine)
	}
}

func TestBrokerStandUnsetFails(t *testing.T) {
	assertBrokerRejected(t, "stand", "", "", false)
}

func TestBrokerStandLocalhostFails(t *testing.T) {
	assertBrokerRejected(t, "stand", "localhost:9092", "", false)
	assertBrokerRejected(t, "stand", "localhost", "", false)
}

func TestBrokerStandLoopbackFails(t *testing.T) {
	assertBrokerRejected(t, "stand", "127.0.0.1:9092", "", false)
}

func TestBrokerStandKfakeAddressFails(t *testing.T) {
	assertBrokerRejected(t, "stand", "kfake:9092", "", false)
}

func TestBrokerStandUnavailableDoesNotUseKfake(t *testing.T) {
	dialed := false
	plan, err := decideBroker("stand", standBrokerAddress, "", func(addr string) error {
		dialed = true
		if addr != standBrokerAddress {
			t.Fatalf("dialed %s", addr)
		}
		return errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("expected dial failure")
	}
	if !dialed {
		t.Fatal("expected dial attempt")
	}
	if plan.UseKfake {
		t.Fatal("kfake authorized after dial failure")
	}
	if strings.Contains(err.Error(), "kfake.NewCluster") {
		t.Fatalf("error suggests kfake startup: %v", err)
	}
}

func TestBrokerStandReadyAllowsRedpanda(t *testing.T) {
	plan, err := decideBroker("stand", " redpanda:9092 ", "", func(addr string) error {
		if addr != standBrokerAddress {
			t.Fatalf("dialed %s", addr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.UseKfake || plan.Broker != standBrokerAddress {
		t.Fatalf("plan %+v", plan)
	}
}

func TestBrokerStandWithAllowFails(t *testing.T) {
	assertBrokerRejected(t, "stand", standBrokerAddress, "1", false)
}

func TestBrokerCIWithoutAllowFails(t *testing.T) {
	assertBrokerRejected(t, "ci", "", "", false)
	assertBrokerRejected(t, "ci", standBrokerAddress, "", false)
}

func TestBrokerMissingModeFails(t *testing.T) {
	assertBrokerRejected(t, "", standBrokerAddress, "1", false)
	assertBrokerRejected(t, "prod", standBrokerAddress, "", false)
}

func TestBrokerCIAllowUsesKfake(t *testing.T) {
	dialed := false
	plan, err := decideBroker("ci", "", "1", func(string) error {
		dialed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.UseKfake || dialed {
		t.Fatalf("plan %+v dialed %v", plan, dialed)
	}
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, "shipment.status.v1", "driver.events.v1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cluster.Close() })
	if len(cluster.ListenAddrs()) == 0 {
		t.Fatal("kfake cluster has no listen address")
	}
}

func TestStandDatabaseHostAllowlist(t *testing.T) {
	const secret = "super-secret-value"
	ok, err := validateStandDatabaseURL("postgres://stand_user:" + secret + "@postgres:5432/bintrans_e2e_stand?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ok, "@postgres:5432/") {
		t.Fatal("accepted URL lost the postgres host")
	}
	rejected := []string{
		"",
		"postgres://stand_user:" + secret + "@localhost:5432/bintrans_e2e_stand?sslmode=disable",
		"postgres://stand_user:" + secret + "@127.0.0.1:5432/bintrans_e2e_stand?sslmode=disable",
		"postgres://stand_user:" + secret + "@freight-postgres:5432/bintrans_e2e_stand?sslmode=disable",
		"postgres://stand_user:" + secret + "@postgres:5433/bintrans_e2e_stand?sslmode=disable",
		"mysql://stand_user:" + secret + "@postgres:5432/bintrans_e2e_stand",
	}
	for _, raw := range rejected {
		_, err := validateStandDatabaseURL(raw)
		if err == nil {
			t.Fatalf("accepted %q", raw)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("host error leaked credential material: %v", err)
		}
	}
}

func TestStandSchemaGate(t *testing.T) {
	clean := standSchemaState{Version: 105, RuntimeRolesNoLogin: true}
	if err := evaluateStandSchema(clean); err != nil {
		t.Fatal(err)
	}
	cases := []standSchemaState{
		{Version: 104, RuntimeRolesNoLogin: true},
		{Version: 105, Dirty: true, RuntimeRolesNoLogin: true},
		{Version: 105, TriggerCount: 1, RuntimeRolesNoLogin: true},
		{Version: 105, ExtraLoginRoles: 1, RuntimeRolesNoLogin: true},
		{Version: 105, RuntimeRolesNoLogin: false},
	}
	for _, state := range cases {
		if err := evaluateStandSchema(state); err == nil {
			t.Fatalf("accepted %+v", state)
		}
	}
}

func assertBrokerRejected(t *testing.T, mode, broker, allow string, expectDial bool) {
	t.Helper()
	dialed := false
	plan, err := decideBroker(mode, broker, allow, func(string) error {
		dialed = true
		return nil
	})
	if err == nil {
		t.Fatalf("mode %q broker %q allow %q was accepted", mode, broker, allow)
	}
	if plan.UseKfake {
		t.Fatal("kfake authorized")
	}
	if dialed != expectDial {
		t.Fatalf("dialed=%v want %v", dialed, expectDial)
	}
}
