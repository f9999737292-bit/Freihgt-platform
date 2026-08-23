//go:build integration

package ledger

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/client/billing_register"
	"github.com/freight-platform/freight-cost-service/internal/client/payment"
	"github.com/freight-platform/freight-cost-service/internal/client/transport_order"
	"github.com/freight-platform/freight-cost-service/internal/config"
	"github.com/freight-platform/freight-cost-service/internal/domain"
	httpserver "github.com/freight-platform/freight-cost-service/internal/http"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

const maxMigrationFile = "000063_freight_cost_accessorial_enrichment_v2.2D.up.sql"
const testToken = "fc-ledger-test-token"

type env struct {
	pool        *pgxpool.Pool
	ingest      *service.IngestService
	rebuild     *service.RebuildService
	costs       *service.CostService
	entries     *repository.CostEntryRepository
	projections *repository.CostSummaryProjectionRepository
	cursors     *repository.SourceCursorRepository
	router      http.Handler
}

type fixture struct {
	TenantID      uuid.UUID
	OtherTenantID uuid.UUID
	BuyerID       uuid.UUID
	CarrierID     uuid.UUID
	OrderID       uuid.UUID
	ShipmentID    uuid.UUID
	OriginID      uuid.UUID
	DestID        uuid.UUID
	CargoID       uuid.UUID
	SnapshotID    uuid.UUID
	RfxEventID    uuid.UUID
	PlannedAmount decimal.Decimal
}

func setupEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	metrics := fcmetrics.New()
	entries := repository.NewCostEntryRepository(pool)
	cursors := repository.NewSourceCursorRepository(pool)
	projections := repository.NewCostSummaryProjectionRepository(pool)
	attributions := repository.NewVarianceAttributionRepository()
	findings := repository.NewReconciliationFindingRepository()
	mappings := repository.NewChargeCodeMappingRepository(pool)

	mockTransport := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/internal/v1/transport-orders/")
		orderIDStr := strings.TrimSuffix(path, "/rate-snapshot")
		orderID, parseErr := uuid.Parse(orderIDStr)
		if parseErr != nil {
			http.Error(w, "invalid transport order id", http.StatusBadRequest)
			return
		}
		var tenantID, buyerID, carrierID, snapshotID uuid.UUID
		var totalAmount string
		queryErr := pool.QueryRow(context.Background(), `
			SELECT tenant_id, buyer_company_id, carrier_company_id, id, total_amount::text
			FROM transport.transport_order_rate_snapshots
			WHERE transport_order_id = $1
			ORDER BY resolved_at DESC
			LIMIT 1`, orderID).Scan(&tenantID, &buyerID, &carrierID, &snapshotID, &totalAmount)
		if queryErr != nil {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"transport_order_id":    orderID.String(),
			"tenant_id":             tenantID.String(),
			"buyer_company_id":      buyerID.String(),
			"carrier_company_id":    carrierID.String(),
			"snapshot_id":           snapshotID.String(),
			"currency_code":         "RUB",
			"total_amount":          totalAmount,
			"pricing_source":        "RFQ_AWARD",
			"pricing_model_version": "SNAPSHOT_V1",
			"resolved_at":           time.Now().UTC().Format(time.RFC3339),
		})
	}))
	t.Cleanup(mockTransport.Close)

	mockBilling := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(mockBilling.Close)
	mockPayment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(mockPayment.Close)

	cfg := config.Config{
		ServiceName:          "freight-cost-service",
		Environment:          "test",
		InternalServiceToken: testToken,
		TransportOrderURL:    mockTransport.URL,
		BillingRegisterURL:   mockBilling.URL,
		PaymentServiceURL:    mockPayment.URL,
	}
	transportClient := transport_order.NewClient(cfg.TransportOrderURL, cfg.InternalServiceToken, metrics)
	billingClient := billing_register.NewClient(cfg.BillingRegisterURL, testToken, metrics)
	paymentClient := payment.NewClient(cfg.PaymentServiceURL, testToken, metrics)
	derived := service.NewDerivedProjectionService(pool, projections, attributions, findings, mappings, cursors, billingClient, transportClient, metrics)
	ingest := service.NewIngestService(pool, entries, cursors, projections, derived, nil, metrics)
	rebuild := service.NewRebuildService(ingest, derived, transportClient, billingClient, paymentClient, metrics)
	costs := service.NewCostService(transportClient, projections)
	workspace := service.NewWorkspaceService(projections, nil, costs, transportClient)
	log := slog.New(slog.DiscardHandler)
	router := httpserver.NewRouter(log, pool, cfg, costs, ingest, rebuild, derived, workspace, mappings, nil, nil, nil, metrics)

	return &env{
		pool: pool, ingest: ingest, rebuild: rebuild, costs: costs,
		entries: entries, projections: projections, cursors: cursors, router: router,
	}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.Compare(filepath.Base(file), maxMigrationFile) > 0 {
			break
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			msg := execErr.Error()
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
				continue
			}
			return execErr
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func seedFixture(t *testing.T, pool *pgxpool.Pool) fixture {
	t.Helper()
	ctx := context.Background()
	fix := fixture{
		TenantID:      uuid.New(),
		OtherTenantID: uuid.New(),
		BuyerID:       uuid.New(),
		CarrierID:     uuid.New(),
		OrderID:       uuid.New(),
		ShipmentID:    uuid.New(),
		OriginID:      uuid.New(),
		DestID:        uuid.New(),
		CargoID:       uuid.New(),
		SnapshotID:    uuid.New(),
		RfxEventID:    uuid.New(),
		PlannedAmount: decimal.RequireFromString("1000.00"),
	}
	for _, row := range []struct {
		tenant uuid.UUID
		code   string
		name   string
	}{
		{fix.TenantID, "t-" + fix.TenantID.String()[:8], "Ledger Tenant"},
		{fix.OtherTenantID, "t-" + fix.OtherTenantID.String()[:8], "Other Tenant"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
			row.tenant, row.code, row.name); err != nil {
			t.Fatalf("tenant: %v", err)
		}
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{fix.BuyerID, fix.TenantID, "SHIPPER", "Buyer Co"},
		{fix.CarrierID, fix.TenantID, "CARRIER", "Carrier Co"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')`, row.id, row.tenant, row.typ, row.name); err != nil {
			t.Fatalf("company: %v", err)
		}
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{fix.OriginID, "Origin"},
		{fix.DestID, "Destination"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow')`, loc.id, fix.TenantID, fix.BuyerID, loc.name); err != nil {
			t.Fatalf("location: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description, gross_weight)
		VALUES ($1,$2,'GENERAL','Cargo',1000)`, fix.CargoID, fix.TenantID); err != nil {
		t.Fatalf("cargo: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.transport_orders (
		id, tenant_id, order_number, shipper_company_id, consignee_company_id,
		origin_location_id, destination_location_id, cargo_id, transport_mode, status, pricing_model_version
	) VALUES ($1,$2,$3,$4,$4,$5,$6,$7,'ROAD','CONVERTED_TO_SHIPMENT','SNAPSHOT_V1')`,
		fix.OrderID, fix.TenantID, "TO-"+fix.OrderID.String()[:8], fix.BuyerID, fix.OriginID, fix.DestID, fix.CargoID); err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.transport_order_rate_snapshots (
		id, tenant_id, transport_order_id, buyer_company_id, carrier_company_id,
		pricing_source, rfx_event_id, origin_location_id, destination_location_id, equipment_type, transport_mode,
		currency_code, component_breakdown_status, components, accessorial_rules,
		total_amount, pricing_date, resolved_at, resolved_by_service, resolver_version, resolution_request_hash
	) VALUES ($1,$2,$3,$4,$5,'RFQ_AWARD',$6,$7,$8,'TAUTLINER','ROAD','RUB','UNAVAILABLE','[]'::jsonb,'[]'::jsonb,
		$9,CURRENT_DATE,now(),'integration-test','v2.1B','cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc')`,
		fix.SnapshotID, fix.TenantID, fix.OrderID, fix.BuyerID, fix.CarrierID, fix.RfxEventID,
		fix.OriginID, fix.DestID, fix.PlannedAmount.StringFixed(domain.MoneyScale)); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO transport.shipments (
		id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
		carrier_company_id, origin_location_id, destination_location_id, cargo_id, transport_mode, status, actual_delivery_at
	) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8,$9,'ROAD','DELIVERED',now())`,
		fix.ShipmentID, fix.TenantID, "SHP-"+fix.ShipmentID.String()[:8], fix.OrderID, fix.BuyerID, fix.CarrierID,
		fix.OriginID, fix.DestID, fix.CargoID); err != nil {
		t.Fatalf("shipment: %v", err)
	}
	return fix
}

func settlementSourceID() uuid.UUID {
	return uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
}

func baseIngestInput(fix fixture, opts ingestOpts) service.SourceEventInput {
	if opts.eventID == uuid.Nil {
		opts.eventID = uuid.New()
	}
	if opts.occurredAt.IsZero() {
		opts.occurredAt = time.Now().UTC()
	}
	if opts.sourceID == uuid.Nil {
		opts.sourceID = settlementSourceID()
	}
	if opts.entryKind == "" {
		opts.entryKind = domain.EntryKindAccrualCostSnapshot
	}
	if opts.sourceType == "" {
		opts.sourceType = domain.SourceTypeFreightSettlement
	}
	if opts.sourceService == "" {
		opts.sourceService = domain.SourceServiceBillingRegister
	}
	if opts.taxBasis == "" {
		opts.taxBasis = domain.TaxBasisExVAT
	}
	if opts.amountAvailability == "" {
		opts.amountAvailability = domain.AmountAvailabilityAvailable
	}
	if opts.eventOrigin == "" {
		opts.eventOrigin = domain.EventOriginLiveOutbox
	}
	if opts.currencyCode == "" {
		opts.currencyCode = "RUB"
	}
	if opts.settlementStatus == "" {
		opts.settlementStatus = domain.SettlementStatusApproved
	}
	return service.SourceEventInput{
		EventID:              opts.eventID,
		EventType:            opts.eventType,
		SchemaVersion:        1,
		TenantID:             fix.TenantID,
		TransportOrderID:     fix.OrderID,
		ShipmentID:           &fix.ShipmentID,
		BuyerCompanyID:       fix.BuyerID,
		CarrierCompanyID:     fix.CarrierID,
		EntryKind:            opts.entryKind,
		SourceService:        opts.sourceService,
		SourceType:           opts.sourceType,
		SourceID:             opts.sourceID,
		SourceRevision:       opts.sourceRevision,
		SourceRevisionSemantic: opts.revisionSemantic,
		CurrencyCode:         opts.currencyCode,
		TaxBasis:             opts.taxBasis,
		AmountAvailability:   opts.amountAvailability,
		Amount:               opts.amount,
		OccurredAt:           opts.occurredAt,
		EventOrigin:          opts.eventOrigin,
		SettlementStatus:     opts.settlementStatus,
		OpenDisputeCount:     opts.openDisputeCount,
	}
}

type ingestOpts struct {
	eventID            uuid.UUID
	eventType          string
	entryKind          string
	sourceService      string
	sourceType         string
	sourceID           uuid.UUID
	sourceRevision     int64
	revisionSemantic   string
	amount             *decimal.Decimal
	amountAvailability string
	taxBasis           domain.TaxBasis
	currencyCode       string
	eventOrigin        string
	settlementStatus   string
	openDisputeCount   int
	occurredAt         time.Time
}

func ingest(t *testing.T, env *env, input service.SourceEventInput) service.IngestResult {
	t.Helper()
	result, err := env.ingest.Ingest(context.Background(), input)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	return result
}

func countCostEntries(t *testing.T, pool *pgxpool.Pool, tenantID, transportOrderID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM freight_cost.cost_entry
		WHERE tenant_id = $1 AND transport_order_id = $2`, tenantID, transportOrderID).Scan(&count)
	if err != nil {
		t.Fatalf("count cost entries: %v", err)
	}
	return count
}

func countCostEntriesByFact(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, sourceFactID uuid.UUID) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM freight_cost.cost_entry
		WHERE tenant_id = $1 AND source_fact_id = $2`, tenantID, sourceFactID).Scan(&count)
	if err != nil {
		t.Fatalf("count by fact: %v", err)
	}
	return count
}

func getProjection(t *testing.T, env *env, fix fixture) *domain.CostSummaryProjection {
	t.Helper()
	projection, err := env.projections.GetByTransportOrder(context.Background(), fix.TenantID, fix.OrderID)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	return projection
}

func deriveFactID(fix fixture, opts ingestOpts) uuid.UUID {
	input := baseIngestInput(fix, opts)
	semantic := input.SourceRevisionSemantic
	if semantic == "" {
		semantic = domain.SourceRevisionSemantic(input.SourceType, input.SourceRevision)
	}
	return domain.DeriveSourceFactID(
		fix.TenantID, input.SourceService, input.SourceType, input.SourceID, semantic, input.EntryKind,
	)
}

func rebuildDeliveryID(tenantID, sourceFactID uuid.UUID) uuid.UUID {
	return domain.DeriveRebuildDeliveryID(tenantID, sourceFactID)
}

func decimalAmount(raw string) *decimal.Decimal {
	v := decimal.RequireFromString(raw)
	return &v
}

func postSourceEventHTTP(t *testing.T, env *env, fix fixture, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/freight-cost/source-events", strings.NewReader(string(raw)))
	req.Header.Set(internalauth.HeaderName, testToken)
	req.Header.Set("X-Tenant-ID", fix.TenantID.String())
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func getCostSummaryHTTP(t *testing.T, env *env, fix fixture, tenantID, companyID uuid.UUID, actorKind string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/freight-cost/transport-orders/"+fix.OrderID.String(), nil)
	req.Header.Set(internalauth.HeaderName, testToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", uuid.NewString())
	req.Header.Set("X-Company-ID", companyID.String())
	req.Header.Set("X-Actor-Kind", actorKind)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return payload
}

func assertActorKindCarrier(t *testing.T) {
	t.Helper()
	if security.ActorKindCarrier != "CARRIER" {
		t.Fatalf("unexpected carrier kind constant")
	}
}
