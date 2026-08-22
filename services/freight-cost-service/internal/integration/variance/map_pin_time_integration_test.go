//go:build integration

package variance

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

func TestBonus_MAP_PIN_TIME_001_FutureRuleBelowPinExcludedFromHistoricalRebuild(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	pinTime := time.Now().UTC().Truncate(time.Second)
	v11End := pinTime.Add(12 * time.Hour)
	futureStart := pinTime.Add(24 * time.Hour)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "DETENTION", 11, pinTime.Add(-24*time.Hour), &v11End)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "FUEL", 10, futureStart, nil)

	_, tenantAtPin, _, err := env.mappings.LoadPinnedMappings(context.Background(), nil, fix.TenantID, 11, pinTime)
	if err != nil {
		t.Fatalf("pinned at pin time: %v", err)
	}
	category, ok := tenantMappingCategory(t, tenantAtPin, "PIN_TIME_FUEL")
	if !ok || category != "DETENTION" {
		t.Fatalf("historical pin at pinTime expected DETENTION, got ok=%v category=%q", ok, category)
	}

	later := pinTime.Add(48 * time.Hour)
	_, tenantHistorical, _, err := env.mappings.LoadPinnedMappings(context.Background(), nil, fix.TenantID, 11, pinTime)
	if err != nil {
		t.Fatalf("pinned at later wall time: %v", err)
	}
	category, ok = tenantMappingCategory(t, tenantHistorical, "PIN_TIME_FUEL")
	if !ok || category != "DETENTION" {
		t.Fatalf("historical rebuild must not leak future FUEL rule, got ok=%v category=%q", ok, category)
	}
	_, tenantActiveLater, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, later)
	if err != nil {
		t.Fatalf("active at later: %v", err)
	}
	if cat, activeOK := tenantMappingCategory(t, tenantActiveLater, "PIN_TIME_FUEL"); !activeOK || cat != "FUEL" {
		t.Fatalf("active load at later wall time should see FUEL rule, got ok=%v category=%q", activeOK, cat)
	}
}

func TestBonus_MAP_PIN_TIME_002_ExpiredRuleActiveAtPinRemainsSelected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	pinTime := time.Now().UTC().Truncate(time.Second)
	expiredTo := pinTime.Add(24 * time.Hour)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_EXPIRED_ACTIVE", "WAITING", 62, pinTime.Add(-72*time.Hour), &expiredTo)

	later := pinTime.Add(96 * time.Hour)
	_, tenantActive, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, later)
	if err != nil {
		t.Fatalf("active later: %v", err)
	}
	if _, ok := tenantMappingCategory(t, tenantActive, "PIN_TIME_EXPIRED_ACTIVE"); ok {
		t.Fatal("rule expired after pin window must not be active at later wall time")
	}

	_, tenantPinned, _, err := env.mappings.LoadPinnedMappings(context.Background(), nil, fix.TenantID, 62, pinTime)
	if err != nil {
		t.Fatalf("pinned historical: %v", err)
	}
	category, ok := tenantMappingCategory(t, tenantPinned, "PIN_TIME_EXPIRED_ACTIVE")
	if !ok || category != "WAITING" {
		t.Fatalf("rule active at pin time must remain in historical rebuild, got ok=%v category=%q", ok, category)
	}
}

func TestBonus_MAP_PIN_TIME_003_RuleExpiredBeforePinRemainsExcluded(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	pinTime := time.Now().UTC().Truncate(time.Second)
	expiredTo := pinTime.Add(-1 * time.Hour)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_ALREADY_EXPIRED", "LUMPER", 63, pinTime.Add(-72*time.Hour), &expiredTo)

	_, tenantPinned, _, err := env.mappings.LoadPinnedMappings(context.Background(), nil, fix.TenantID, 63, pinTime)
	if err != nil {
		t.Fatalf("pinned: %v", err)
	}
	if _, ok := tenantMappingCategory(t, tenantPinned, "PIN_TIME_ALREADY_EXPIRED"); ok {
		t.Fatal("rule expired before pin evaluation time must never appear in historical rebuild")
	}
}

func TestBonus_MAP_PIN_TIME_004_ReclassificationUsesFutureRuleAndRepins(t *testing.T) {
	settlementID := uuid.New()
	accessorialID := uuid.New()
	env := setupEnvConfigured(t, envConfig{
		billingHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/by-transport-order/") {
				http.NotFound(w, r)
				return
			}
			orderID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/internal/v1/freight-settlements/by-transport-order/"))
			tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settlement_id":                      settlementID.String(),
				"transport_order_id":                 orderID,
				"tenant_id":                          tenantID,
				"buyer_company_id":                   uuid.NewString(),
				"carrier_company_id":                 uuid.NewString(),
				"shipment_id":                        uuid.NewString(),
				"status":                             domain.SettlementStatusApproved,
				"open_dispute_count":                 0,
				"version":                            1,
				"billing_link_revision":              0,
				"billing_link_state":                 domain.BillingLinkStateUnlinked,
				"currency_code":                      "RUB",
				"accrual_amount_ex_vat":              "1100.00",
				"total_without_vat":                  "1100.00",
				"proposed_accessorial_source_status": domain.ProposedSourceUnknown,
				"updated_at":                         time.Now().UTC().Format(time.RFC3339),
				"approved_accessorials": []map[string]string{
					{
						"accessorial_id": accessorialID.String(),
						"charge_code":    "PIN_TIME_FUEL",
						"amount_ex_vat":  "100.00",
					},
				},
			})
		}),
	})
	fix := seedFixture(t, env.pool)
	pinTime := time.Now().UTC().Truncate(time.Second)
	fuelStart := pinTime.Add(-1 * time.Hour)
	detentionEnd := fuelStart
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "DETENTION", 71, pinTime.Add(-24*time.Hour), &detentionEnd)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "FUEL", 72, fuelStart, nil)

	ingestPlannedAndActual(t, env, fix)
	before := getProjection(t, env, fix)
	if before.AttributionMappingEvaluatedAt == nil {
		t.Fatal("initial compute must persist attribution_mapping_evaluated_at")
	}

	inserted, err := env.derived.ReclassifyAttribution(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reclassify: %v", err)
	}
	if inserted == 0 {
		t.Fatal("expected reclassified driver rows")
	}
	after := getProjection(t, env, fix)
	if after.AttributionMappingEvaluatedAt == nil {
		t.Fatal("reclassify must persist attribution_mapping_evaluated_at")
	}
	if before.AttributionMappingEvaluatedAt != nil && !after.AttributionMappingEvaluatedAt.After(*before.AttributionMappingEvaluatedAt) {
		t.Fatal("reclassify must advance mapping evaluation timestamp")
	}
	reason := currentDriverReasonCode(t, env, fix)
	if reason != domain.ReasonFuel {
		t.Fatalf("reclassify with active FUEL mapping must produce FUEL driver, got %q", reason)
	}
}

func TestBonus_MAP_PIN_TIME_005_StandardRebuildAfterReclassificationReproducesResult(t *testing.T) {
	settlementID := uuid.New()
	accessorialID := uuid.New()
	env := setupEnvConfigured(t, envConfig{
		billingHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/by-transport-order/") {
				http.NotFound(w, r)
				return
			}
			orderID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/internal/v1/freight-settlements/by-transport-order/"))
			tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settlement_id":                      settlementID.String(),
				"transport_order_id":                 orderID,
				"tenant_id":                          tenantID,
				"buyer_company_id":                   uuid.NewString(),
				"carrier_company_id":                 uuid.NewString(),
				"shipment_id":                        uuid.NewString(),
				"status":                             domain.SettlementStatusApproved,
				"open_dispute_count":                 0,
				"version":                            1,
				"billing_link_revision":              0,
				"billing_link_state":                 domain.BillingLinkStateUnlinked,
				"currency_code":                      "RUB",
				"accrual_amount_ex_vat":              "1100.00",
				"total_without_vat":                  "1100.00",
				"proposed_accessorial_source_status": domain.ProposedSourceUnknown,
				"updated_at":                         time.Now().UTC().Format(time.RFC3339),
				"approved_accessorials": []map[string]string{
					{
						"accessorial_id": accessorialID.String(),
						"charge_code":    "PIN_TIME_FUEL",
						"amount_ex_vat":  "100.00",
					},
				},
			})
		}),
	})
	fix := seedFixture(t, env.pool)
	pinTime := time.Now().UTC().Truncate(time.Second)
	fuelStart := pinTime.Add(-1 * time.Hour)
	detentionEnd := fuelStart
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "DETENTION", 81, pinTime.Add(-24*time.Hour), &detentionEnd)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "FUEL", 82, fuelStart, nil)

	ingestPlannedAndActual(t, env, fix)
	if _, err := env.derived.ReclassifyAttribution(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("reclassify: %v", err)
	}
	afterReclassify := getProjection(t, env, fix)
	reclassifyReason := currentDriverReasonCode(t, env, fix)
	pinnedVersion := afterReclassify.AttributionMappingVersion
	pinnedEval := afterReclassify.AttributionMappingEvaluatedAt
	if pinnedVersion == nil || pinnedEval == nil {
		t.Fatal("expected complete mapping pin after reclassify")
	}

	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"PIN_TIME_FUEL", "OTHER", 99, pinTime.Add(1*time.Hour), nil)

	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	afterRebuild := getProjection(t, env, fix)
	if afterRebuild.AttributionMappingVersion == nil || *afterRebuild.AttributionMappingVersion != *pinnedVersion {
		t.Fatalf("standard rebuild must preserve pinned version: before=%v after=%v", pinnedVersion, afterRebuild.AttributionMappingVersion)
	}
	if afterRebuild.AttributionMappingEvaluatedAt == nil || !afterRebuild.AttributionMappingEvaluatedAt.Equal(*pinnedEval) {
		t.Fatalf("standard rebuild must preserve pinned evaluation time")
	}
	rebuildReason := currentDriverReasonCode(t, env, fix)
	if rebuildReason != reclassifyReason {
		t.Fatalf("standard rebuild must reproduce reclassified attribution: reclassify=%q rebuild=%q", reclassifyReason, rebuildReason)
	}
}

func TestBonus_MAP_PIN_TIME_006_LegacyMissingEvaluatedAtBootstrapsCompletePin(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)

	_, err := env.pool.Exec(context.Background(), `
		UPDATE freight_cost.cost_summary_projection
		SET attribution_mapping_evaluated_at = NULL
		WHERE tenant_id = $1 AND transport_order_id = $2`, fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("simulate legacy pin: %v", err)
	}

	if err := env.derived.EnsureLegacyDerivedStateBootstrapped(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	projection := getProjection(t, env, fix)
	if projection.AttributionMappingVersion == nil {
		t.Fatal("legacy bootstrap must persist attribution_mapping_version")
	}
	if projection.AttributionMappingEvaluatedAt == nil {
		t.Fatal("legacy bootstrap must persist attribution_mapping_evaluated_at")
	}
}

func currentDriverReasonCode(t *testing.T, env *env, fix fixture) string {
	t.Helper()
	var reasonCode string
	err := env.pool.QueryRow(context.Background(), `
		SELECT reason_code FROM freight_cost.variance_attribution
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND semantic_class = 'VARIANCE_DRIVER' AND is_current = TRUE
		  AND variance_kind = 'CURRENT'
		ORDER BY recorded_at DESC LIMIT 1`, fix.TenantID, fix.OrderID).Scan(&reasonCode)
	if err != nil {
		t.Fatalf("query driver reason: %v", err)
	}
	return reasonCode
}
