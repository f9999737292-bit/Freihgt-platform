//go:build integration

package controltowerreadmodelintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	gatewaylegacy "github.com/freight-platform/api-gateway/internal/controltower/legacyaggregate"
	gatewayrm "github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

type fullBaselineHarness struct {
	t *testing.T
	*shipmentBaselineHarness
	readModelBaseURL string
}

func newFullBaselineHarness(t *testing.T) *fullBaselineHarness {
	t.Helper()
	shipmentHarness := newShipmentBaselineHarness(t)
	readModelURL, _, err := startReadModelProcess(t, shipmentHarness.databaseURL)
	if err != nil {
		t.Fatalf("start read-model: %v", err)
	}
	return &fullBaselineHarness{
		t:                       t,
		shipmentBaselineHarness: shipmentHarness,
		readModelBaseURL:        readModelURL,
	}
}

func (h *fullBaselineHarness) legacyClient(timeout time.Duration) *gatewaylegacy.Client {
	return gatewaylegacy.NewClient(&http.Client{Timeout: timeout}, gatewaylegacy.Config{
		BaseURL:          h.shipmentBaseURL,
		Timeout:          timeout,
		MaxResponseBytes: 256 * 1024,
	}, gatewaylegacy.NewMetrics())
}

func (h *fullBaselineHarness) client(timeout time.Duration) *gatewayrm.Client {
	return gatewayrm.NewClient(&http.Client{Timeout: timeout}, gatewayrm.Config{
		BaseURL:          h.readModelBaseURL,
		Timeout:          timeout,
		MaxResponseBytes: 256 * 1024,
	}, gatewayrm.NewMetrics())
}

func (h *fullBaselineHarness) seedAuthoritativeAndProjection(tenantID uuid.UUID, statuses []string) []uuid.UUID {
	return seedAuthoritativeAndProjection(h.t, h.pool, tenantID, statuses)
}

func seedAuthoritativeAndProjection(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, statuses []string) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	fix := seedMinimalTenantFixture(t, pool, tenantID)
	shipmentIDs := make([]uuid.UUID, 0, len(statuses))
	for i, status := range statuses {
		shipmentID := uuid.New()
		shipmentIDs = append(shipmentIDs, shipmentID)
		number := fmt.Sprintf("SH-BB-%s-%d", tenantID.String()[:8], i+1)
		_, err := pool.Exec(ctx, `
INSERT INTO transport.shipments (
	id, tenant_id, shipment_number, transport_order_id,
	shipper_company_id, consignee_company_id, carrier_company_id,
	origin_location_id, destination_location_id, transport_mode, status, version
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1)`,
			shipmentID, fix.tenantID, number, fix.transportOrderID,
			fix.shipperID, fix.consigneeID, fix.carrierID, fix.originID, fix.destinationID, status,
		)
		if err != nil {
			t.Fatalf("seed shipment: %v", err)
		}
		seedSingleProjection(t, pool, fix.tenantID, shipmentID, status)
	}
	return shipmentIDs
}

type minimalTenantFixture struct {
	tenantID         uuid.UUID
	shipperID        uuid.UUID
	consigneeID      uuid.UUID
	carrierID        uuid.UUID
	originID         uuid.UUID
	destinationID    uuid.UUID
	transportOrderID uuid.UUID
}

func seedMinimalTenantFixture(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) minimalTenantFixture {
	t.Helper()
	ctx := context.Background()
	fix := minimalTenantFixture{
		tenantID:      tenantID,
		shipperID:     uuid.New(),
		consigneeID:   uuid.New(),
		carrierID:     uuid.New(),
		originID:      uuid.New(),
		destinationID: uuid.New(),
	}
	fix.transportOrderID = uuid.New()
	suffix := tenantID.String()[:8]
	if _, err := pool.Exec(ctx, `
INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
		fix.tenantID, "T-"+suffix, "Tenant "+suffix); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id   uuid.UUID
		name string
		typ  string
	}{
		{fix.shipperID, "Shipper " + suffix, "SHIPPER"},
		{fix.consigneeID, "Consignee " + suffix, "CONSIGNEE"},
		{fix.carrierID, "Carrier " + suffix, "CARRIER"},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO core.companies (id, tenant_id, legal_name, company_type)
VALUES ($1, $2, $3, $4)`, row.id, fix.tenantID, row.name, row.typ); err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.originID, "Origin " + suffix},
		{fix.destinationID, "Destination " + suffix},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code)
VALUES ($1, $2, 'WAREHOUSE', $3, 'RU')`, loc.id, fix.tenantID, loc.name); err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO transport.transport_orders (
	id, tenant_id, order_number, status, shipper_company_id, consignee_company_id,
	origin_location_id, destination_location_id, transport_mode
) VALUES ($1, $2, $3, 'ASSIGNED', $4, $5, $6, $7, 'ROAD')`,
		fix.transportOrderID, fix.tenantID, "TO-"+suffix,
		fix.shipperID, fix.consigneeID, fix.originID, fix.destinationID); err != nil {
		t.Fatalf("seed transport order: %v", err)
	}
	return fix
}

func seedSingleProjection(t *testing.T, pool *pgxpool.Pool, tenantID, shipmentID uuid.UUID, status string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
	tenant_id, shipment_id, shipment_version, current_status,
	last_event_id, last_source_event_id, last_event_type,
	last_occurred_at, last_consumed_at, complete, gap_detected
) VALUES ($1, $2, 1, $3, $4, $5, 'shipment.status.changed', $6, $6, true, false)`,
		tenantID, shipmentID, status, uuid.New(), uuid.New(), now,
	)
	if err != nil {
		t.Fatalf("seed projection: %v", err)
	}
}

func TestFullBaselineShadowMatch(t *testing.T) {
	h := newFullBaselineHarness(t)
	tenantA := uuid.New()
	statuses := []string{"IN_TRANSIT", "IN_TRANSIT", "DELIVERED"}
	h.seedAuthoritativeAndProjection(tenantA, statuses)

	legacyClient := h.legacyClient(2 * time.Second)
	legacySummary, legacyErr := legacyClient.FetchStatusSummary(context.Background(), "shadow", tenantA.String(), "bb-shadow-match")
	if legacyErr != nil {
		t.Fatalf("legacy aggregate: %v", legacyErr)
	}

	rmClient := h.client(2 * time.Second)
	rmPayload, rmErr := rmClient.FetchStatusSummary(context.Background(), gatewayrm.ModeShadow, tenantA.String(), "bb-shadow-match")
	if rmErr != nil {
		t.Fatalf("read-model: %v", rmErr)
	}

	legacyInput := gatewayrm.LegacyStatusInput{
		TotalShipments:         legacySummary.TotalShipments,
		CountedShipments:       legacySummary.CountedShipments,
		ByStatus:               legacySummary.ByStatus,
		LimitedDataset:         false,
		FullAggregateAvailable: true,
	}
	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode:                   gatewayrm.ModeShadow,
		Legacy:                 legacyInput,
		ReadModel:              rmPayload,
		RequireConsumerRunning: false,
	})
	if out.Comparison != gatewayrm.ComparisonMatch {
		t.Fatalf("comparison=%q want MATCH", out.Comparison)
	}
	if out.StatusSummary == nil || out.StatusSummary.LimitedDataset || out.StatusSummary.CountedShipments != out.StatusSummary.TotalShipments {
		t.Fatalf("summary=%+v", out.StatusSummary)
	}
}

func TestFullBaselineShadowMismatch(t *testing.T) {
	h := newFullBaselineHarness(t)
	tenantA := uuid.New()
	shipmentIDs := h.seedAuthoritativeAndProjection(tenantA, []string{"IN_TRANSIT", "DELIVERED"})

	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection
SET current_status = 'DELIVERED'
WHERE tenant_id = $1 AND shipment_id = $2`, tenantA, shipmentIDs[0])
	if err != nil {
		t.Fatalf("skew projection: %v", err)
	}

	legacyClient := h.legacyClient(2 * time.Second)
	legacySummary, legacyErr := legacyClient.FetchStatusSummary(context.Background(), "shadow", tenantA.String(), "bb-shadow-mismatch")
	if legacyErr != nil {
		t.Fatalf("legacy aggregate: %v", legacyErr)
	}
	rmClient := h.client(2 * time.Second)
	rmPayload, rmErr := rmClient.FetchStatusSummary(context.Background(), gatewayrm.ModeShadow, tenantA.String(), "bb-shadow-mismatch")
	if rmErr != nil {
		t.Fatalf("read-model: %v", rmErr)
	}

	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModeShadow,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:         legacySummary.TotalShipments,
			CountedShipments:       legacySummary.CountedShipments,
			ByStatus:               legacySummary.ByStatus,
			FullAggregateAvailable: true,
		},
		ReadModel:              rmPayload,
		RequireConsumerRunning: false,
	})
	if out.Comparison != gatewayrm.ComparisonStatusCountMismatch {
		t.Fatalf("comparison=%q want STATUS_COUNT_MISMATCH", out.Comparison)
	}
	if out.StatusSummary == nil || out.StatusSummary.Source != gatewayrm.SourceLegacy {
		t.Fatal("user-facing shadow response must remain legacy aggregate")
	}
}

func TestFullBaselinePrimaryFullFallback(t *testing.T) {
	h := newShipmentBaselineHarness(t)
	tenantA := uuid.New()
	seedAuthoritativeAndProjection(t, h.pool, tenantA, []string{"IN_TRANSIT", "IN_TRANSIT"})

	legacyClient := gatewaylegacy.NewClient(&http.Client{Timeout: 2 * time.Second}, gatewaylegacy.Config{
		BaseURL: h.shipmentBaseURL, Timeout: 2 * time.Second, MaxResponseBytes: 256 * 1024,
	}, gatewaylegacy.NewMetrics())
	legacySummary, legacyErr := legacyClient.FetchStatusSummary(context.Background(), "primary", tenantA.String(), "bb-primary-fallback")
	if legacyErr != nil {
		t.Fatalf("legacy aggregate: %v", legacyErr)
	}

	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModePrimary,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:         legacySummary.TotalShipments,
			CountedShipments:       legacySummary.CountedShipments,
			ByStatus:               legacySummary.ByStatus,
			FullAggregateAvailable: true,
		},
		ReadModelErr:           &gatewayrm.DependencyError{Reason: gatewayrm.ReasonNon2XX, Status: 503},
		RequireConsumerRunning: true,
	})
	if out.StatusSummary.Source != gatewayrm.SourceLegacy || out.StatusSummary.LimitedDataset {
		t.Fatalf("expected full legacy fallback, got %+v", out.StatusSummary)
	}
	if out.StatusSummaryFreshness == nil || !out.StatusSummaryFreshness.FallbackUsed || out.StatusSummaryFreshness.Partial {
		t.Fatalf("freshness=%+v", out.StatusSummaryFreshness)
	}
}

func TestFullBaselineDoubleFailurePageLimited(t *testing.T) {
	out := gatewayrm.Merge(gatewayrm.MergeInput{
		Mode: gatewayrm.ModePrimary,
		Legacy: gatewayrm.LegacyStatusInput{
			TotalShipments:         1200,
			CountedShipments:       100,
			ByStatus:               map[string]int64{"IN_TRANSIT": 100},
			LimitedDataset:         true,
			FullAggregateAvailable: false,
		},
		ReadModelErr:           &gatewayrm.DependencyError{Reason: gatewayrm.ReasonNon2XX},
		RequireConsumerRunning: true,
	})
	if !out.StatusSummary.LimitedDataset || !out.StatusSummaryFreshness.Partial {
		t.Fatalf("expected page-limited fallback, summary=%+v freshness=%+v", out.StatusSummary, out.StatusSummaryFreshness)
	}
	found := false
	for _, w := range out.Warnings {
		if w == gatewayrm.WarningLegacyLimited {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings=%v", out.Warnings)
	}
}

func TestFullBaselineTenantIsolationAggregate(t *testing.T) {
	h := newShipmentBaselineHarness(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	seedAuthoritativeAndProjection(t, h.pool, tenantA, []string{"IN_TRANSIT", "IN_TRANSIT", "DELIVERED"})
	seedAuthoritativeAndProjection(t, h.pool, tenantB, []string{"CANCELLED", "CANCELLED"})

	client := gatewaylegacy.NewClient(&http.Client{Timeout: 2 * time.Second}, gatewaylegacy.Config{
		BaseURL: h.shipmentBaseURL, Timeout: 2 * time.Second, MaxResponseBytes: 256 * 1024,
	}, gatewaylegacy.NewMetrics())
	summaryA, errA := client.FetchStatusSummary(context.Background(), "disabled", tenantA.String(), "bb-tenant-a")
	if errA != nil {
		t.Fatalf("tenant A: %v", errA)
	}
	if summaryA.TotalShipments != 3 {
		t.Fatalf("tenant A total=%d", summaryA.TotalShipments)
	}
	summaryB, errB := client.FetchStatusSummary(context.Background(), "disabled", tenantB.String(), "bb-tenant-b")
	if errB != nil {
		t.Fatalf("tenant B: %v", errB)
	}
	if summaryB.ByStatus["CANCELLED"] != 2 {
		t.Fatalf("tenant B byStatus=%v", summaryB.ByStatus)
	}
}

func applyShipmentMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		if strings.HasPrefix(base, "000015") {
			continue
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("apply %s: %w", base, execErr)
		}
	}
	return nil
}

func startShipmentProcess(t *testing.T, databaseURL string) (baseURL string, stop func(), err error) {
	t.Helper()
	binPath := buildServiceBinaryOnce(t, "services/shipment-service", shipmentServiceBinaryKey)
	baseURL = startManagedHTTPProcess(t, binPath, []string{
		"DATABASE_URL=" + databaseURL,
		integrationDBPoolEnv,
		integrationDBIdleEnv,
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
	}, "HTTP_PORT", "/ready")
	return baseURL, func() {}, nil
}

func TestFullBaselineAggregateResponseHasNoTenant(t *testing.T) {
	h := newShipmentBaselineHarness(t)
	tenantA := uuid.New()
	seedAuthoritativeAndProjection(t, h.pool, tenantA, []string{"IN_TRANSIT"})

	req, err := http.NewRequest(http.MethodGet, h.shipmentBaseURL+"/internal/v1/shipments/status-summary", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", tenantA.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(payload)
	if strings.Contains(strings.ToLower(string(body)), "tenant") {
		t.Fatalf("response must not expose tenant fields: %s", body)
	}
}
