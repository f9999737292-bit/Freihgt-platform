package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestBonus_MAP_PRECEDENCE_001_TenantActiveMappingBeatsPlatform(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	platform := []ChargeCodeMapping{{
		MappingScope:               MappingScopePlatform,
		SourceChargeCodeNormalized: "BONUS_TENANT_WIN",
		NormalizedCategory:         "DETENTION",
		MappingVersion:             1,
	}}
	tenant := []ChargeCodeMapping{{
		MappingScope:               MappingScopeTenant,
		TenantID:                   &tenantID,
		SourceChargeCodeNormalized: "BONUS_TENANT_WIN",
		NormalizedCategory:         "FUEL",
		MappingVersion:             2,
	}}
	got := ResolveChargeCategory("bonus_tenant_win", platform, tenant)
	if got != "FUEL" {
		t.Fatalf("tenant active mapping must beat platform at evaluation time, got %q", got)
	}
}
