//go:build integration

package variance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

func TestFC_C_CHG_001_PlatformDefaultViaNullTenantID(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	tx, err := env.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	platform, _, version, err := env.mappings.LoadActiveMappings(ctx, tx, fix.TenantID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	if version < 1 {
		t.Fatalf("mapping version = %d", version)
	}
	for _, m := range platform {
		if m.MappingScope != domain.MappingScopePlatform {
			t.Fatalf("expected platform scope, got %q", m.MappingScope)
		}
		if m.TenantID != nil {
			t.Fatal("platform mapping must use tenant_id IS NULL")
		}
	}
}

func TestFC_C_CHG_003_CrossTenantMappingLookupDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ctx := context.Background()
	_, tenantRows, _, err := env.mappings.LoadActiveMappings(ctx, nil, fix.OtherTenantID, time.Now().UTC())
	if err != nil {
		t.Fatalf("load other tenant: %v", err)
	}
	for _, row := range tenantRows {
		if row.TenantID != nil && *row.TenantID == fix.TenantID {
			t.Fatal("tenant A mapping must not appear in tenant B lookup")
		}
	}
}

func TestFC_C_CHG_009_PlatformMappingRequiresNullTenant(t *testing.T) {
	env := setupEnv(t)
	tenantID := uuid.New()
	_, err := env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopePlatform,
		TenantID:       &tenantID,
		SourceCode:     "TEST_PLATFORM",
		TargetCategory: "OTHER",
		EffectiveFrom:  time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error when platform mapping has tenant_id")
	}
}

func TestFC_C_CHG_010_TenantMappingRequiresTenantID(t *testing.T) {
	env := setupEnv(t)
	_, err := env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       nil,
		SourceCode:     "TEST_TENANT",
		TargetCategory: "OTHER",
		EffectiveFrom:  time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error when tenant mapping missing tenant_id")
	}
}
