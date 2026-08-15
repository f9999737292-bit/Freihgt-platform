//go:build integration

package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/client"
	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/domain/tender"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

type testEnv struct {
	pool *pgxpool.Pool
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping live PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	_, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("create temp database: %v", err)
	}
	t.Cleanup(func() { dropDB(context.Background()) })

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return &testEnv{pool: pool}
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}

	dbName = "freight_platform_rfx_enterprise_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"

	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, fmt.Errorf("connect admin database: %w", err)
	}
	defer adminPool.Close()

	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, fmt.Errorf("create database: %w", err)
	}

	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL = buildDSN(testCfg)

	cleanup = func(cctx context.Context) {
		cadmin, cerr := pgxpool.NewWithConfig(cctx, adminCfg)
		if cerr != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return dbName, testURL, cleanup, nil
}

func buildDSN(cfg *pgxpool.Config) string {
	user := url.QueryEscape(cfg.ConnConfig.User)
	pass := url.QueryEscape(cfg.ConnConfig.Password)
	host := cfg.ConnConfig.Host
	port := cfg.ConnConfig.Port
	db := cfg.ConnConfig.Database
	ssl := "disable"
	if cfg.ConnConfig.TLSConfig != nil {
		ssl = "require"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, pass, host, port, db, ssl)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
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
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(file), execErr)
		}
	}
	return nil
}

func locateMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	wd, _ := os.Getwd()
	return "", fmt.Errorf("migrations dir not found from %s", wd)
}

type carrierBid struct {
	CompanyID      uuid.UUID
	PriceAmount    float64
	CapacityUnits  float64
	TransitHours   float64
	SLAScoreInput  float64
	CarrierKPI     float64
	Reliability    float64
	ResponseID     uuid.UUID
}

type enterpriseFixture struct {
	TenantID          uuid.UUID
	OwnerID           uuid.UUID
	EventID           uuid.UUID
	Carriers          [4]carrierBid
	TemplateVer       uuid.UUID
	RfxType           string
	FreightRequestID  uuid.UUID
	TransportOrderID  uuid.UUID
}

func defaultScoringFactors() []tender.ScoringFactorWeight {
	return []tender.ScoringFactorWeight{
		{Factor: tender.FactorPrice, Weight: 35},
		{Factor: tender.FactorSLA, Weight: 20},
		{Factor: tender.FactorCarrierKPI, Weight: 15},
		{Factor: tender.FactorCapacity, Weight: 10},
		{Factor: tender.FactorReliability, Weight: 10},
		{Factor: tender.FactorTransitTime, Weight: 10},
	}
}

func seedEnterpriseFixture(t *testing.T, pool *pgxpool.Pool, suffix string) enterpriseFixture {
	t.Helper()
	ctx := context.Background()
	fix := enterpriseFixture{
		TenantID: uuid.New(),
		OwnerID:  uuid.New(),
		EventID:  uuid.New(),
	}
	for i := range fix.Carriers {
		fix.Carriers[i].CompanyID = uuid.New()
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)
	`, fix.TenantID, "t-"+suffix, "RFx Enterprise Tenant "+suffix)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
		VALUES ($1, $2, $3, 'SHIPPER', 'ACTIVE')
	`, fix.OwnerID, fix.TenantID, "Owner "+suffix)
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	carrierSpecs := []struct {
		name  string
		price float64
		sla   float64
		cap   float64
		trans float64
		kpi   float64
		rel   float64
	}{
		{"Carrier A", 100, 90, 500, 24, 85, 80},
		{"Carrier B", 90, 70, 400, 20, 88, 75},
		{"Carrier C", 80, 95, 300, 18, 92, 90},
		{"Carrier D", 110, 88, 200, 22, 80, 85},
	}
	for i, spec := range carrierSpecs {
		_, err = pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
			VALUES ($1, $2, $3, 'CARRIER', 'ACTIVE')
		`, fix.Carriers[i].CompanyID, fix.TenantID, spec.name+" "+suffix)
		if err != nil {
			t.Fatalf("seed carrier %d: %v", i, err)
		}
		fix.Carriers[i].PriceAmount = spec.price
		fix.Carriers[i].CapacityUnits = spec.cap
		fix.Carriers[i].TransitHours = spec.trans
		fix.Carriers[i].SLAScoreInput = spec.sla
		fix.Carriers[i].CarrierKPI = spec.kpi
		fix.Carriers[i].Reliability = spec.rel
	}

	deadline := time.Now().UTC().Add(48 * time.Hour)
	rfxType := "CONTRACT_TENDER"
	if fix.RfxType != "" {
		rfxType = fix.RfxType
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO rfx.rfx_events (
			id, tenant_id, rfx_number, rfx_type, category, title, owner_company_id, status, response_deadline
		) VALUES ($1, $2, $3, $4, 'FREIGHT', $5, $6, 'PUBLISHED', $7)
	`, fix.EventID, fix.TenantID, "RFX-"+suffix, rfxType, "Enterprise Tender "+suffix, fix.OwnerID, deadline)
	if err != nil {
		t.Fatalf("seed rfx event: %v", err)
	}

	for i := range fix.Carriers {
		_, err = pool.Exec(ctx, `
			INSERT INTO rfx.rfx_participants (tenant_id, rfx_event_id, company_id, participant_type, status, invited_at)
			VALUES ($1, $2, $3, 'CARRIER', 'INVITED', now())
		`, fix.TenantID, fix.EventID, fix.Carriers[i].CompanyID)
		if err != nil {
			t.Fatalf("seed participant %d: %v", i, err)
		}

		var responseID uuid.UUID
		err = pool.QueryRow(ctx, `
			INSERT INTO rfx.rfx_responses (
				tenant_id, rfx_event_id, participant_company_id, status,
				price_amount, capacity_units, transit_hours,
				sla_score_input, carrier_kpi_score_input, reliability_score_input,
				submitted_at
			) VALUES ($1, $2, $3, 'SUBMITTED', $4, $5, $6, $7, $8, $9, now())
			RETURNING id
		`, fix.TenantID, fix.EventID, fix.Carriers[i].CompanyID,
			fix.Carriers[i].PriceAmount, fix.Carriers[i].CapacityUnits, fix.Carriers[i].TransitHours,
			fix.Carriers[i].SLAScoreInput, fix.Carriers[i].CarrierKPI, fix.Carriers[i].Reliability,
		).Scan(&responseID)
		if err != nil {
			t.Fatalf("seed response %d: %v", i, err)
		}
		fix.Carriers[i].ResponseID = responseID

		_, err = pool.Exec(ctx, `
			UPDATE rfx.rfx_participants SET status = 'RESPONSE_SUBMITTED', responded_at = now()
			WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
		`, fix.EventID, fix.Carriers[i].CompanyID, fix.TenantID)
		if err != nil {
			t.Fatalf("update participant %d: %v", i, err)
		}
	}

	tenderRepo := repository.NewTenderRepository(pool)
	_, versionID, err := tenderRepo.CreateScoringTemplate(ctx, fix.TenantID, "tpl-"+suffix, "Enterprise Template "+suffix, defaultScoringFactors(), nil)
	if err != nil {
		t.Fatalf("seed scoring template: %v", err)
	}
	fix.TemplateVer = versionID
	seedActiveRevisionsForFixture(t, pool, fix)
	return fix
}

func seedActiveRevisionsForFixture(t *testing.T, pool *pgxpool.Pool, fix enterpriseFixture) {
	t.Helper()
	ctx := context.Background()
	for i := range fix.Carriers {
		c := fix.Carriers[i]
		_, err := pool.Exec(ctx, `
			INSERT INTO rfx.rfx_response_revisions (
				tenant_id, rfx_response_id, revision_number, is_active,
				price_amount, currency_code, capacity_units, transit_hours,
				sla_score_input, carrier_kpi_score_input, reliability_score_input,
				submitted_at
			) VALUES ($1, $2, 1, true, $3, 'RUB', $4, $5, $6, $7, $8, now())
		`, fix.TenantID, c.ResponseID, c.PriceAmount, c.CapacityUnits, c.TransitHours,
			c.SLAScoreInput, c.CarrierKPI, c.Reliability)
		if err != nil {
			t.Fatalf("seed active revision %d: %v", i, err)
		}
		_, err = pool.Exec(ctx, `
			UPDATE rfx.rfx_responses SET active_revision_number = 1, updated_at = now()
			WHERE id = $1 AND tenant_id = $2
		`, c.ResponseID, fix.TenantID)
		if err != nil {
			t.Fatalf("update active revision number %d: %v", i, err)
		}
	}
}

func seedPrimaryE2EFixture(t *testing.T, pool *pgxpool.Pool, suffix string) enterpriseFixture {
	t.Helper()
	fix := seedEnterpriseFixture(t, pool, suffix)
	fix.RfxType = "MINI_TENDER"
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		UPDATE rfx.rfx_events SET rfx_type = 'MINI_TENDER' WHERE id = $1 AND tenant_id = $2
	`, fix.EventID, fix.TenantID)
	if err != nil {
		t.Fatalf("update rfx type: %v", err)
	}

	originID := uuid.New()
	destID := uuid.New()
	consigneeID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
		VALUES ($1, $2, $3, 'CONSIGNEE', 'ACTIVE')
	`, consigneeID, fix.TenantID, "Consignee "+suffix)
	if err != nil {
		t.Fatalf("seed consignee: %v", err)
	}
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{originID, "Origin " + suffix},
		{destID, "Destination " + suffix},
	} {
		_, err = pool.Exec(ctx, `
			INSERT INTO transport.locations (
				id, tenant_id, location_type, name, country_code, city, status
			) VALUES ($1, $2, 'WAREHOUSE', $3, 'RU', 'Moscow', 'ACTIVE')
		`, loc.id, fix.TenantID, loc.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}

	fix.TransportOrderID = uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'READY_FOR_SOURCING')
	`, fix.TransportOrderID, fix.TenantID, "TO-"+suffix, fix.OwnerID, consigneeID, originID, destID)
	if err != nil {
		t.Fatalf("seed transport order: %v", err)
	}

	fix.FreightRequestID = uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO rfx.freight_requests (
			id, tenant_id, freight_request_number, request_type, shipper_company_id,
			transport_order_id, rfx_event_id, status, response_deadline
		) VALUES ($1, $2, $3, 'MINI_TENDER', $4, $5, $6, 'PUBLISHED', now() + interval '48 hours')
	`, fix.FreightRequestID, fix.TenantID, "FR-"+suffix, fix.OwnerID, fix.TransportOrderID, fix.EventID)
	if err != nil {
		t.Fatalf("seed freight request: %v", err)
	}
	return fix
}

func newEvaluationService(pool *pgxpool.Pool) *service.EvaluationService {
	return service.NewEvaluationService(repository.NewTenderRepository(pool), service.StaticCarrierPerformanceProvider{}, nil)
}

func newEvaluationServiceWithConversion(t *testing.T, pool *pgxpool.Pool, shipmentURL string) *service.EvaluationService {
	t.Helper()
	tenderRepo := repository.NewTenderRepository(pool)
	bidRepo := repository.NewBidRepository(pool)
	shipmentClient := client.NewShipmentClient(shipmentURL)
	conv := service.NewAwardConversionService(tenderRepo, bidRepo, shipmentClient)
	return service.NewEvaluationService(tenderRepo, service.StaticCarrierPerformanceProvider{}, conv)
}

func stubShipmentServer(t *testing.T) (*httptest.Server, func(uuid.UUID) uuid.UUID) {
	t.Helper()
	created := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/shipments/from-bid" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		shipmentID := uuid.New().String()
		created[fmt.Sprint(body["bid_id"])] = shipmentID
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                 shipmentID,
			"shipment_number":    body["shipment_number"],
			"transport_order_id": body["transport_order_id"],
			"status":             "CREATED",
		})
	}))
	t.Cleanup(srv.Close)
	return srv, func(bidID uuid.UUID) uuid.UUID {
		id, err := uuid.Parse(created[bidID.String()])
		if err != nil {
			t.Fatalf("shipment not created for bid %s", bidID)
		}
		return id
	}
}

func newRfxService(pool *pgxpool.Pool) *service.RfxService {
	return service.NewRfxService(repository.NewRfxRepository(pool))
}

func runEvaluationChain(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	fix enterpriseFixture,
	rules tender.QualificationRules,
	requiredVolume float64,
	quotaTargets []tender.QuotaTarget,
	actualShares map[string]float64,
) (evalID, scenarioID, proposalID uuid.UUID) {
	t.Helper()
	evalSvc := newEvaluationService(pool)

	result, err := evalSvc.RunEvaluation(ctx, service.RunEvaluationInput{
		TenantID:                 fix.TenantID,
		RfxEventID:               fix.EventID,
		ScoringTemplateVersionID: fix.TemplateVer,
		QualificationRules:       rules,
		RequiredVolume:           requiredVolume,
	})
	if err != nil {
		t.Fatalf("run evaluation: %v", err)
	}
	evalID = result.EvaluationID

	scenarioInput := service.CreateAllocationScenarioInput{
		TenantID:     fix.TenantID,
		EvaluationID: evalID,
		Name:         "primary-scenario",
		Config: tender.AllocationConfig{
			Strategy: tender.StrategyDiversified,
			RankShares: []float64{40, 30, 20, 10},
			Constraints: tender.AllocationConstraints{
				TotalVolume:        requiredVolume,
				MinSuppliers:       3,
				MaxCarrierSharePct: 50,
			},
		},
		QuotaPolicy: tender.QuotaBalancePolicy{
			TolerancePct:     5,
			CarryBalance:     len(quotaTargets) > 0 && actualShares != nil,
			MaxCorrectionPct: 10,
			PeriodType:       "CONTRACT_PERIOD",
		},
		ActualShares: actualShares,
	}
	if len(quotaTargets) > 0 {
		scenarioInput.QuotaTargets = quotaTargets
	}
	scenarioID, outcome, _, err := evalSvc.RunAllocationScenario(ctx, scenarioInput)
	if err != nil {
		t.Fatalf("run allocation scenario: %v", err)
	}
	if outcome.Status != tender.AllocationStatusComputed {
		t.Fatalf("allocation not computed: status=%s reasons=%v", outcome.Status, outcome.Reasons)
	}

	idem := "proposal-" + fix.EventID.String()
	proposalID, err = evalSvc.CreateAwardProposal(ctx, fix.TenantID, fix.EventID, evalID, scenarioID, nil, &idem)
	if err != nil {
		t.Fatalf("create award proposal: %v", err)
	}
	if err := evalSvc.SubmitAwardProposal(ctx, proposalID, fix.TenantID); err != nil {
		t.Fatalf("submit award proposal: %v", err)
	}
	return evalID, scenarioID, proposalID
}

func quotaTargetsFromFixture(fix enterpriseFixture) []tender.QuotaTarget {
	out := make([]tender.QuotaTarget, 0, len(fix.Carriers))
	for _, c := range fix.Carriers {
		out = append(out, tender.QuotaTarget{
			CarrierCompanyID: c.CompanyID.String(),
			TargetSharePct: 25,
		})
	}
	return out
}

func defaultQuotaPolicy() tender.QuotaBalancePolicy {
	return tender.QuotaBalancePolicy{
		TolerancePct:     5,
		CarryBalance:     false,
		MaxCorrectionPct: 10,
		PeriodType:       "CONTRACT_PERIOD",
	}
}

func balancedActualShares(fix enterpriseFixture) map[string]float64 {
	out := make(map[string]float64, len(fix.Carriers))
	for _, c := range fix.Carriers {
		out[c.CompanyID.String()] = 25
	}
	return out
}

func balancedActualSharesWith(fix enterpriseFixture, overrides map[int]float64) map[string]float64 {
	out := balancedActualShares(fix)
	for idx, pct := range overrides {
		out[fix.Carriers[idx].CompanyID.String()] = pct
	}
	return out
}

func strPtr(s string) *string {
	return &s
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, schema, table string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`, schema, table).Scan(&exists)
	return exists, err
}

func constraintExists(ctx context.Context, pool *pgxpool.Pool, schema, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints
			WHERE constraint_schema = $1 AND constraint_name = $2
		)
	`, schema, name).Scan(&exists)
	return exists, err
}

func indexExists(ctx context.Context, pool *pgxpool.Pool, schema, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = $1 AND indexname = $2
		)
	`, schema, name).Scan(&exists)
	return exists, err
}

func columnExists(ctx context.Context, pool *pgxpool.Pool, schema, table, column string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
}

func assertRfxEventStatus(t *testing.T, pool *pgxpool.Pool, eventID, tenantID uuid.UUID, want string) {
	t.Helper()
	var status string
	err := pool.QueryRow(context.Background(), `
		SELECT status FROM rfx.rfx_events WHERE id = $1 AND tenant_id = $2
	`, eventID, tenantID).Scan(&status)
	if err != nil {
		t.Fatalf("load event status: %v", err)
	}
	if status != want {
		t.Fatalf("event status=%s want %s", status, want)
	}
}

func createFreightRequestBid(t *testing.T, pool *pgxpool.Pool, tenantID, carrierID uuid.UUID, suffix string) (freightRequestID, bidID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	shipperID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, legal_name, company_type, status)
		VALUES ($1, $2, $3, 'SHIPPER', 'ACTIVE')
	`, shipperID, tenantID, "Shipper "+suffix)
	if err != nil {
		t.Fatalf("seed shipper: %v", err)
	}

	freightRequestID = uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO rfx.freight_requests (
			id, tenant_id, freight_request_number, request_type, shipper_company_id, status, response_deadline
		) VALUES ($1, $2, $3, 'MINI_TENDER', $4, 'PUBLISHED', now() + interval '48 hours')
	`, freightRequestID, tenantID, "FR-"+suffix, shipperID)
	if err != nil {
		t.Fatalf("seed freight request: %v", err)
	}

	bidID = uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO rfx.bids (
			id, tenant_id, freight_request_id, carrier_company_id, bid_number, status, total_amount, currency_code
		) VALUES ($1, $2, $3, $4, $5, 'DRAFT', 1000, 'RUB')
	`, bidID, tenantID, freightRequestID, carrierID, "BID-"+suffix)
	if err != nil {
		t.Fatalf("seed bid: %v", err)
	}
	return freightRequestID, bidID
}

func carrierScopedResponseQuery(ctx context.Context, pool *pgxpool.Pool, eventID, tenantID, viewerCarrierID uuid.UUID) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_responses
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND participant_company_id = $3 AND deleted_at IS NULL
	`, eventID, tenantID, viewerCarrierID).Scan(&count)
	return count, err
}

func carrierCanAccessResponse(ctx context.Context, pool *pgxpool.Pool, responseID, tenantID, viewerCarrierID uuid.UUID) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM rfx.rfx_responses
			WHERE id = $1 AND tenant_id = $2 AND participant_company_id = $3 AND deleted_at IS NULL
		)
	`, responseID, tenantID, viewerCarrierID).Scan(&exists)
	return exists, err
}

var _ = domain.RfxStatusPublished
