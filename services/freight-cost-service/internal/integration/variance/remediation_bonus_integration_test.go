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
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

func TestBonus_MAP_WINDOW_001_ExpiredPlatformMappingExcludedFromActiveLoad(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	now := time.Now().UTC()
	expiredTo := now.Add(-1 * time.Hour)
	insertChargeMappingRow(t, env.pool, domain.MappingScopePlatform, nil,
		"BONUS_PLATFORM_EXPIRED", "FUEL", 50, now.Add(-72*time.Hour), &expiredTo)

	platformMappings, _, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, now)
	if err != nil {
		t.Fatalf("load active mappings: %v", err)
	}
	if _, ok := platformMappingCategory(t, platformMappings, "BONUS_PLATFORM_EXPIRED"); ok {
		t.Fatal("expired platform mapping must not be active at evaluation time")
	}
}

func TestBonus_MAP_WINDOW_002_FuturePlatformMappingExcludedFromActiveLoad(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	now := time.Now().UTC()
	insertChargeMappingRow(t, env.pool, domain.MappingScopePlatform, nil,
		"BONUS_PLATFORM_FUTURE", "DETENTION", 51, now.Add(48*time.Hour), nil)

	platformMappings, _, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, now)
	if err != nil {
		t.Fatalf("load active mappings: %v", err)
	}
	if _, ok := platformMappingCategory(t, platformMappings, "BONUS_PLATFORM_FUTURE"); ok {
		t.Fatal("future platform effective_from mapping must not be active at evaluation time")
	}
}

func TestBonus_MAP_WINDOW_003_ExpiredTenantMappingExcludedFromActiveLoad(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	now := time.Now().UTC()
	expiredTo := now.Add(-1 * time.Hour)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"BONUS_WINDOW_EXPIRED", "WAITING", 52, now.Add(-72*time.Hour), &expiredTo)

	_, tenantMappings, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, now)
	if err != nil {
		t.Fatalf("load active mappings: %v", err)
	}
	if _, ok := tenantMappingCategory(t, tenantMappings, "BONUS_WINDOW_EXPIRED"); ok {
		t.Fatal("expired tenant effective_to mapping must not be active at evaluation time")
	}
}

func TestBonus_MAP_WINDOW_004_FutureTenantMappingExcludedFromActiveLoad(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	now := time.Now().UTC()
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"BONUS_TENANT_FUTURE", "DETENTION", 53, now.Add(48*time.Hour), nil)

	_, tenantMappings, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, now)
	if err != nil {
		t.Fatalf("load active mappings: %v", err)
	}
	if _, ok := tenantMappingCategory(t, tenantMappings, "BONUS_TENANT_FUTURE"); ok {
		t.Fatal("future tenant effective_from mapping must not be active at evaluation time")
	}
}

func TestBonus_MAP_OVERLAP_001_OverlappingActiveWindowsRejected(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	start := time.Now().UTC().Add(-24 * time.Hour)
	end := start.Add(30 * 24 * time.Hour)
	_, err := env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "BONUS_OVERLAP_A",
		TargetCategory: "FUEL",
		EffectiveFrom:  start,
		EffectiveTo:    &end,
	})
	if err != nil {
		t.Fatalf("first mapping: %v", err)
	}
	overlapStart := start.Add(15 * 24 * time.Hour)
	overlapEnd := overlapStart.Add(30 * 24 * time.Hour)
	_, err = env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "BONUS_OVERLAP_A",
		TargetCategory: "DETENTION",
		EffectiveFrom:  overlapStart,
		EffectiveTo:    &overlapEnd,
	})
	if err == nil {
		t.Fatal("expected overlap constraint violation")
	}
	if !repository.IsOverlapConstraintViolation(err) {
		t.Fatalf("expected overlap violation, got %v", err)
	}
}

func TestBonus_MAP_OVERLAP_002_AdjacentNonOverlappingWindowsAllowed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	start := time.Now().UTC().Add(-48 * time.Hour)
	firstEnd := start.Add(30 * 24 * time.Hour)
	_, err := env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "BONUS_OVERLAP_B",
		TargetCategory: "FUEL",
		EffectiveFrom:  start,
		EffectiveTo:    &firstEnd,
	})
	if err != nil {
		t.Fatalf("first window: %v", err)
	}
	secondEnd := firstEnd.Add(30 * 24 * time.Hour)
	_, err = env.mappings.UpsertMapping(context.Background(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   domain.MappingScopeTenant,
		TenantID:       &fix.TenantID,
		SourceCode:     "BONUS_OVERLAP_B",
		TargetCategory: "DETENTION",
		EffectiveFrom:  firstEnd,
		EffectiveTo:    &secondEnd,
	})
	if err != nil {
		t.Fatalf("adjacent window must be allowed: %v", err)
	}
}

func TestBonus_MAP_OVERLAP_003_HTTPPutOverlapReturnsValidationError(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	start := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	firstBody := `{"mapping_scope":"TENANT","source_code":"BONUS_OVERLAP_HTTP","target_category":"FUEL","effective_from":"` + start + `","effective_to":"` + end + `"}`
	rec := putChargeMappingHTTP(t, env, fix, firstBody, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("first put status = %d body=%s", rec.Code, rec.Body.String())
	}
	overlapStart := time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339)
	overlapEnd := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	secondBody := `{"mapping_scope":"TENANT","source_code":"BONUS_OVERLAP_HTTP","target_category":"DETENTION","effective_from":"` + overlapStart + `","effective_to":"` + overlapEnd + `"}`
	rec = putChargeMappingHTTP(t, env, fix, secondBody, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("overlap put must return 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "overlapping active mapping window") {
		t.Fatalf("expected overlap validation message, body=%s", rec.Body.String())
	}
}

func TestBonus_MAP_PIN_001_PinnedLoadUsesHistoricallyPinnedEffectiveWindow(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	pinTime := time.Now().UTC()
	expiredTo := pinTime.Add(24 * time.Hour)
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"BONUS_PIN_HISTORIC", "LUMPER", 60, pinTime.Add(-72*time.Hour), &expiredTo)

	later := pinTime.Add(72 * time.Hour)
	_, tenantActive, _, err := env.mappings.LoadActiveMappings(context.Background(), nil, fix.TenantID, later)
	if err != nil {
		t.Fatalf("load active: %v", err)
	}
	if _, ok := tenantMappingCategory(t, tenantActive, "BONUS_PIN_HISTORIC"); ok {
		t.Fatal("expired mapping must not appear in active load at later wall time")
	}

	_, tenantPinned, _, err := env.mappings.LoadPinnedMappings(context.Background(), nil, fix.TenantID, 60, pinTime)
	if err != nil {
		t.Fatalf("load pinned: %v", err)
	}
	category, ok := tenantMappingCategory(t, tenantPinned, "BONUS_PIN_HISTORIC")
	if !ok || category != "LUMPER" {
		t.Fatalf("pinned load must use historical evaluation window, got ok=%v category=%q", ok, category)
	}
}

func TestBonus_MAP_PIN_002_PinnedLoadExcludesHigherMappingVersion(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	now := time.Now().UTC()
	insertChargeMappingRow(t, env.pool, domain.MappingScopeTenant, &fix.TenantID,
		"BONUS_PIN_HIGH", "FUEL", 80, now.Add(-24*time.Hour), nil)

	_, tenantPinned, _, err := env.mappings.LoadPinnedMappings(context.Background(), nil, fix.TenantID, 70, now)
	if err != nil {
		t.Fatalf("load pinned: %v", err)
	}
	if _, ok := tenantMappingCategory(t, tenantPinned, "BONUS_PIN_HIGH"); ok {
		t.Fatal("mapping_version above pin must be excluded from pinned load")
	}
}

func TestBonus_SEC_ADMIN_001_PlatformMappingPutDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	body := `{"mapping_scope":"PLATFORM","source_code":"BONUS_PLATFORM","target_category":"FUEL"}`
	rec := putChargeMappingHTTP(t, env, fix, body, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("platform mapping put must return 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "platform mapping mutation is disabled") {
		t.Fatalf("expected fail-closed platform denial, body=%s", rec.Body.String())
	}
}

func TestBonus_SEC_ADMIN_002_FakePlatformAdminHeaderDoesNotBypassDenial(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	body := `{"mapping_scope":"PLATFORM","source_code":"BONUS_PLATFORM_SPOOF","target_category":"FUEL"}`
	rec := putChargeMappingHTTP(t, env, fix, body, map[string]string{
		"X-Freight-Cost-Platform-Admin": "true",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fake platform-admin header must not bypass denial, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBonus_SEC_RECLASS_001_ReclassifyAcceptsEmptyBody(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	rec := reclassifyAttributionHTTP(t, env, fix, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reclassify without body must succeed, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSONBody(t, rec)
	if body["financial_amounts_changed"] != false {
		t.Fatalf("expected financial_amounts_changed=false, got %v", body["financial_amounts_changed"])
	}
}

func TestBonus_SEC_RECLASS_002_ReclassifyIgnoresBodyUsesCanonicalAPIs(t *testing.T) {
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
			payload := map[string]any{
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
						"charge_code":    "FUEL",
						"amount_ex_vat":  "100.00",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		}),
	})
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)

	fakeBody := `{"approved_accessorials":[{"accessorial_id":"` + uuid.NewString() + `","charge_code":"DETENTION","amount_ex_vat":"999.00"}]}`
	rec := reclassifyAttributionHTTP(t, env, fix, &fakeBody, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reclassify status = %d body=%s", rec.Code, rec.Body.String())
	}

	var reasonCode string
	err := env.pool.QueryRow(context.Background(), `
		SELECT reason_code FROM freight_cost.variance_attribution
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND semantic_class = 'VARIANCE_DRIVER' AND is_current = TRUE
		  AND variance_kind = 'CURRENT'
		ORDER BY recorded_at DESC LIMIT 1`, fix.TenantID, fix.OrderID).Scan(&reasonCode)
	if err != nil {
		t.Fatalf("query driver: %v", err)
	}
	if reasonCode != domain.ReasonFuel {
		t.Fatalf("reclassify must hydrate canonical FUEL accessorial, got reason=%q", reasonCode)
	}

	projection := getProjection(t, env, fix)
	currentVersion, err := env.mappings.CurrentPlatformMappingVersion(context.Background())
	if err != nil {
		t.Fatalf("current version: %v", err)
	}
	if projection.AttributionMappingVersion == nil || *projection.AttributionMappingVersion < currentVersion {
		t.Fatalf("reclassify must pin attribution_mapping_version to current mapping version, got %v want >= %d",
			projection.AttributionMappingVersion, currentVersion)
	}
	if projection.AttributionMappingEvaluatedAt == nil {
		t.Fatal("reclassify must pin attribution_mapping_evaluated_at")
	}
}

func TestBonus_REC_CANON_001_ProjectionDriftVsTransportSnapshot(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	projection := getProjection(t, env, fix)
	drifted := decimalAmount("900.00")
	projection.PlannedAmount = drifted
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := env.projections.Upsert(context.Background(), tx, projection); err != nil {
		t.Fatalf("upsert drift: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	count, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if count == 0 {
		t.Fatal("expected canonical projection drift finding")
	}
	var kind string
	err = env.pool.QueryRow(context.Background(), `
		SELECT finding_kind FROM freight_cost.reconciliation_finding
		WHERE tenant_id = $1 AND transport_order_id = $2 AND canonical_reference_key = 'planned_amount'
		LIMIT 1`, fix.TenantID, fix.OrderID).Scan(&kind)
	if err != nil {
		t.Fatalf("query finding: %v", err)
	}
	if kind != domain.FindingProjectionDrift {
		t.Fatalf("finding kind = %q", kind)
	}
}

func TestBonus_REC_CANON_002_MissingPlannedWhenSnapshotAbsent(t *testing.T) {
	env := setupEnvConfigured(t, envConfig{
		transportHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}),
	})
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)

	count, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if count == 0 {
		t.Fatal("expected finding when canonical snapshot missing but projection has planned amount")
	}
	var kind string
	err = env.pool.QueryRow(context.Background(), `
		SELECT finding_kind FROM freight_cost.reconciliation_finding
		WHERE tenant_id = $1 AND transport_order_id = $2 AND canonical_reference_key = 'planned'
		LIMIT 1`, fix.TenantID, fix.OrderID).Scan(&kind)
	if err != nil {
		t.Fatalf("query finding: %v", err)
	}
	if kind != domain.FindingMissingPlannedFact {
		t.Fatalf("finding kind = %q", kind)
	}
}

func TestBonus_REC_CANON_003_ReconcileDoesNotMutateProjectionRevision(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)
	before := getProjection(t, env, fix).ProjectionRevision

	rec := reconcileTransportOrderHTTP(t, env, fix)
	if rec.Code != http.StatusOK {
		t.Fatalf("reconcile status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSONBody(t, rec)
	if body["auto_rebuild"] != false || body["auto_repair"] != false {
		t.Fatalf("reconcile must not auto-rebuild/repair: %v", body)
	}

	after := getProjection(t, env, fix).ProjectionRevision
	if after != before {
		t.Fatalf("reconcile must not mutate projection revision: before=%d after=%d", before, after)
	}
}

func TestBonus_REC_CANON_004_SettlementAccrualDriftDetected(t *testing.T) {
	settlementID := uuid.New()
	env := setupEnvConfigured(t, envConfig{
		billingHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/by-transport-order/") {
				http.NotFound(w, r)
				return
			}
			orderID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/internal/v1/freight-settlements/by-transport-order/"))
			tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			payload := map[string]any{
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
				"accrual_amount_ex_vat":              "1200.00",
				"total_without_vat":                  "1200.00",
				"proposed_accessorial_source_status": domain.ProposedSourceUnknown,
				"updated_at":                         time.Now().UTC().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(payload)
		}),
	})
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)

	count, err := env.derived.ReconcileTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if count == 0 {
		t.Fatal("expected canonical settlement drift finding")
	}
	var kind string
	err = env.pool.QueryRow(context.Background(), `
		SELECT finding_kind FROM freight_cost.reconciliation_finding
		WHERE tenant_id = $1 AND transport_order_id = $2 AND canonical_reference_key = 'accrual_amount'
		LIMIT 1`, fix.TenantID, fix.OrderID).Scan(&kind)
	if err != nil {
		t.Fatalf("query finding: %v", err)
	}
	if kind != domain.FindingProjectionDrift {
		t.Fatalf("finding kind = %q", kind)
	}
}

func TestBonus_FINGERPRINT_001_LegacyNullFingerprintBootstrapDeterministic(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env.pool)
	ingestPlannedAndActual(t, env, fix)

	_, err := env.pool.Exec(context.Background(), `
		UPDATE freight_cost.cost_summary_projection
		SET derived_state_fingerprint = NULL,
		    forecast_exposure = planned_amount + 500.00,
		    forecast_source_status = 'KNOWN'
		WHERE tenant_id = $1 AND transport_order_id = $2`, fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("simulate legacy row: %v", err)
	}

	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("first rebuild: %v", err)
	}
	afterFirst := getProjection(t, env, fix)
	if afterFirst.DerivedStateFingerprint == nil || *afterFirst.DerivedStateFingerprint == "" {
		t.Fatal("legacy NULL fingerprint must bootstrap to deterministic value on recompute")
	}
	if afterFirst.ForecastSourceStatus != domain.ForecastSourceUnknown {
		t.Fatalf("legacy row must not treat forecast_exposure as proposed total, status=%q", afterFirst.ForecastSourceStatus)
	}
	if afterFirst.ForecastExposure != nil {
		t.Fatal("forecast_exposure must be NULL when proposed source is UNKNOWN after bootstrap")
	}
	firstFP := *afterFirst.DerivedStateFingerprint

	if _, err := env.rebuild.RebuildTransportOrder(context.Background(), fix.TenantID, fix.OrderID); err != nil {
		t.Fatalf("second rebuild: %v", err)
	}
	afterSecond := getProjection(t, env, fix)
	if afterSecond.DerivedStateFingerprint == nil || *afterSecond.DerivedStateFingerprint != firstFP {
		t.Fatalf("bootstrap fingerprint must remain stable on idempotent rebuild: first=%q second=%v", firstFP, afterSecond.DerivedStateFingerprint)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
