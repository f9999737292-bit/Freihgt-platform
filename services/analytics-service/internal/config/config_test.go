package config

import (
	"strings"
	"testing"
)

func TestValidateCallerSecrets(t *testing.T) {
	const gateway = "gateway-caller-secret"
	const source = "source-caller-secret"
	const shared = "shared-platform-secret"
	if err := validateCallerSecrets(gateway, source, shared); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		gateway string
		source  string
		shared  string
	}{
		{name: "empty gateway", source: source, shared: shared},
		{name: "empty source", gateway: gateway, shared: shared},
		{name: "equal hop secrets", gateway: gateway, source: gateway, shared: shared},
		{name: "gateway reuses shared", gateway: shared, source: source, shared: shared},
		{name: "source reuses shared", gateway: gateway, source: shared, shared: shared},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCallerSecrets(tt.gateway, tt.source, tt.shared)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if strings.Contains(err.Error(), gateway) || strings.Contains(err.Error(), source) || strings.Contains(err.Error(), shared) {
				t.Fatalf("error disclosed a credential: %s", err.Error())
			}
		})
	}
}
