//go:build integration

package contractrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
	"github.com/freight-platform/contract-rate-service/internal/repository"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

const maxMigrationNumber = 50

type testEnv struct {
	Pool           *pgxpool.Pool
	TenantID       uuid.UUID
	BuyerID        uuid.UUID
	CarrierID      uuid.UUID
	OriginID       uuid.UUID
	DestID         uuid.UUID
	Contracts      *repository.ContractRepository
	RateCards      *repository.RateCardRepository
	RateLines      *repository.RateLineRepository
	RateComponents *repository.RateComponentRepository
	Resolutions    *repository.ResolutionRepository
	Memberships    *repository.MembershipRepository
	ResolutionSvc  *service.ResolutionService
	Actor          domain.ActorInput
	Today          time.Time
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	pool := setupPool(t)
	ctx := context.Background()
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	originID := uuid.New()
	destID := uuid.New()
	seedTenantAndCompanies(t, ctx, pool, tenantID, buyerID, carrierID)
	seedLocations(t, ctx, pool, tenantID, buyerID, originID, destID)
	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	rateCards := repository.NewRateCardRepository(pool, contracts, audit)
	locations := repository.NewLocationRepository(pool)
	rateLines := repository.NewRateLineRepository(pool, rateCards, locations, audit)
	rateComponents := repository.NewRateComponentRepository(pool, rateLines, rateCards, audit)
	resolutions := repository.NewResolutionRepository(pool, audit)
	memberships := repository.NewMembershipRepository(pool)
	actor := domain.ActorInput{
		TenantID: tenantID, ActorUserID: uuid.New(),
		ActorCompanyID: buyerID, ActorKind: domain.ActorKindBuyer,
	}
	resolutionSvc := service.NewResolutionService(resolutions, memberships, nil)
	return &testEnv{
		Pool: pool, TenantID: tenantID, BuyerID: buyerID, CarrierID: carrierID,
		OriginID: originID, DestID: destID,
		Contracts: contracts, RateCards: rateCards, RateLines: rateLines,
		RateComponents: rateComponents, Resolutions: resolutions, Memberships: memberships,
		ResolutionSvc: resolutionSvc, Actor: actor,
		Today: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
	}
}

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := requireTestDatabaseURL(t)
	ctx := context.Background()
	pool, cleanup := createTempDB(t, ctx, adminURL)
	t.Cleanup(cleanup)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return pool
}

func requireTestDatabaseURL(t *testing.T) string {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required")
		}
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return adminURL
}

func (e *testEnv) createDraftContract(t *testing.T, number string) *domain.TransportContract {
	t.Helper()
	created, err := e.Contracts.Create(context.Background(), domain.CreateContractInput{
		TenantID: e.TenantID, BuyerCompanyID: e.BuyerID, CarrierCompanyID: e.CarrierID,
		ContractNumber: number, Name: "Contract " + number, ValidFrom: e.Today,
		CurrencyCode: "RUB", Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create draft %s: %v", number, err)
	}
	return created
}

func seedTenantAndCompanies(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, buyerID, carrierID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name, status)
		VALUES ($1, $2, 'Test Tenant', 'ACTIVE')
		ON CONFLICT DO NOTHING`, tenantID, "T-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{buyerID, tenantID, "SHIPPER", "Buyer Co"},
		{carrierID, tenantID, "CARRIER", "Carrier Co"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')
			ON CONFLICT DO NOTHING`, row.id, row.tenant, row.typ, row.name)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
}

func seedTenantOnly(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name, status)
		VALUES ($1, $2, 'Other Tenant', 'ACTIVE')
		ON CONFLICT DO NOTHING`, tenantID, "T-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
}

func createTempDB(t *testing.T, ctx context.Context, adminURL string) (*pgxpool.Pool, func()) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	dbName := "freight_contract_rate_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		adminPool.Close()
		t.Fatalf("create db: %v", err)
	}
	adminPool.Close()

	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, testCfg)
	if err != nil {
		t.Fatalf("test pool: %v", err)
	}
	cleanup := func() {
		pool.Close()
		adminPool, _ = pgxpool.NewWithConfig(context.Background(), adminCfg)
		if adminPool != nil {
			_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
			adminPool.Close()
		}
	}
	return pool, cleanup
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "infrastructure", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(name, "%d", &num); err != nil || num > maxMigrationNumber {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "infrastructure", "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found")
		}
		dir = parent
	}
}

func strPtr(v string) *string { return &v }

func auditCount(t *testing.T, pool *pgxpool.Pool, tenantID, entityID uuid.UUID, action string) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM contract_rate.audit_event
		WHERE tenant_id=$1 AND entity_id=$2 AND action=$3`, tenantID, entityID, action).Scan(&count)
	if err != nil {
		t.Fatalf("audit count: %v", err)
	}
	return count
}

func isAppErrorCode(err error, code apperrors.Code) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == code
}

func activateRateVersionSQL(t *testing.T, env *testEnv, versionID uuid.UUID) {
	t.Helper()
	_, err := env.Pool.Exec(context.Background(), `
		UPDATE contract_rate.rate_card_version
		SET status='ACTIVE', activated_at=now()
		WHERE tenant_id=$1 AND id=$2`, env.TenantID, versionID)
	if err != nil {
		t.Fatalf("activate version sql: %v", err)
	}
}

func seedLocations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, companyID, originID, destID uuid.UUID) {
	t.Helper()
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{
		{originID, "Origin WH"},
		{destID, "Destination WH"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO transport.locations (
				id, tenant_id, company_id, location_type, name, country_code, city, status
			) VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow','ACTIVE')`,
			row.id, tenantID, companyID, row.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
}

func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1,$2,$3,'Test User','ACTIVE')
		ON CONFLICT DO NOTHING`, userID, tenantID, userID.String()+"@example.test")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedCompanyMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, companyID uuid.UUID) {
	t.Helper()
	seedUser(t, ctx, pool, tenantID, userID)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1,$2,$3,'ACTIVE')
		ON CONFLICT (company_id, user_id) DO UPDATE SET status='ACTIVE', deleted_at=NULL`,
		tenantID, companyID, userID)
	if err != nil {
		t.Fatalf("seed company membership: %v", err)
	}
}

func resolveRoleID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, roleCode string) uuid.UUID {
	t.Helper()
	var roleID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO core.roles (code, name, scope, is_system)
		VALUES ($1,$1,'GLOBAL',true)
		ON CONFLICT DO NOTHING
		RETURNING id`, roleCode).Scan(&roleID)
	if err != nil {
		err = pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE code=$1 AND tenant_id IS NULL LIMIT 1`, roleCode).Scan(&roleID)
		if err != nil {
			t.Fatalf("resolve role %s: %v", roleCode, err)
		}
	}
	return roleID
}

func seedCompanyRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, companyID uuid.UUID, roleCode string) {
	t.Helper()
	seedCompanyMembership(t, ctx, pool, tenantID, userID, companyID)
	roleID := resolveRoleID(t, ctx, pool, roleCode)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT DO NOTHING`, tenantID, userID, companyID, roleID)
	if err != nil {
		t.Fatalf("seed company role: %v", err)
	}
}

func seedTenantRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID uuid.UUID, roleCode string) {
	t.Helper()
	seedUser(t, ctx, pool, tenantID, userID)
	roleID := resolveRoleID(t, ctx, pool, roleCode)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.user_roles (tenant_id, user_id, role_id)
		VALUES ($1,$2,$3)
		ON CONFLICT DO NOTHING`, tenantID, userID, roleID)
	if err != nil {
		t.Fatalf("seed tenant role: %v", err)
	}
}

func seedGlobalRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID uuid.UUID, roleCode string) {
	t.Helper()
	seedTenantRole(t, ctx, pool, tenantID, userID, roleCode)
}

func seedBuyerCompany(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, companyID uuid.UUID, name string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1,$2,'SHIPPER',$3,'ACTIVE')
		ON CONFLICT DO NOTHING`, companyID, tenantID, name)
	if err != nil {
		t.Fatalf("seed buyer company: %v", err)
	}
}

func manualSpotReq(env *testEnv, actor domain.ActorInput) domain.ResolveRateRequest {
	amount := decimal.RequireFromString("5000.00")
	currency := "RUB"
	req := env.resolveReq("TAUTLINER")
	req.Actor = actor
	req.ManualSpotAmount = &amount
	req.ManualSpotCurrency = &currency
	return req
}

func resolveManualSpot(t *testing.T, env *testEnv, actor domain.ActorInput) (domain.ResolveRateResult, error) {
	t.Helper()
	return env.ResolutionSvc.Resolve(context.Background(), manualSpotReq(env, actor), nil)
}

func (e *testEnv) createActiveContract(t *testing.T, number string) *domain.TransportContract {
	t.Helper()
	draft := e.createDraftContract(t, number)
	active, err := e.Contracts.Activate(context.Background(), e.TenantID, draft.ID, e.Actor, nil)
	if err != nil {
		t.Fatalf("activate contract %s: %v", number, err)
	}
	return active
}

func (e *testEnv) createDraftVersion(t *testing.T, contractNumber, cardName string) (*domain.RateCard, *domain.RateCardVersion) {
	t.Helper()
	contract := e.createDraftContract(t, contractNumber)
	card, err := e.RateCards.Create(context.Background(), domain.CreateRateCardInput{
		TenantID: e.TenantID, ContractID: contract.ID, Name: cardName, Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	version, err := e.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
		TenantID: e.TenantID, RateCardID: card.ID, ValidFrom: e.Today, Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	return card, version
}

func (e *testEnv) createRateLine(t *testing.T, versionID uuid.UUID, equipment string) *domain.RateLine {
	t.Helper()
	line, err := e.RateLines.Create(context.Background(), domain.CreateRateLineInput{
		TenantID: e.TenantID, RateCardVersionID: versionID,
		OriginLocationID: e.OriginID, DestinationLocationID: e.DestID,
		EquipmentType: equipment, TransportMode: domain.TransportModeRoad, Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("create rate line: %v", err)
	}
	return line
}

func dec(v string) *decimal.Decimal {
	d := decimal.RequireFromString(v)
	return &d
}

func (e *testEnv) addBaseFreight(t *testing.T, lineID uuid.UUID, amount string) {
	t.Helper()
	_, err := e.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: e.TenantID, RateLineID: lineID,
		ComponentType: domain.ComponentTypeBaseFreight, CalculationMethod: domain.CalcMethodFlat,
		Amount: dec(amount), Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("add base freight: %v", err)
	}
}

func (e *testEnv) addFuelSurcharge(t *testing.T, lineID uuid.UUID, percent string) {
	t.Helper()
	_, err := e.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: e.TenantID, RateLineID: lineID,
		ComponentType: domain.ComponentTypeFuelSurcharge, CalculationMethod: domain.CalcMethodPercent,
		PercentValue: dec(percent), Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("add fuel surcharge: %v", err)
	}
}

func (e *testEnv) addWaitingRule(t *testing.T, lineID uuid.UUID, amount string) {
	t.Helper()
	unit := domain.UnitCodeHour
	_, err := e.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: e.TenantID, RateLineID: lineID,
		ComponentType: domain.ComponentTypeWaiting, CalculationMethod: domain.CalcMethodUnitRate,
		Amount: dec(amount), UnitCode: &unit, Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("add waiting: %v", err)
	}
}

func (e *testEnv) addDetentionRule(t *testing.T, lineID uuid.UUID, amount string) {
	t.Helper()
	unit := domain.UnitCodeHour
	_, err := e.RateComponents.Create(context.Background(), domain.CreateRateComponentInput{
		TenantID: e.TenantID, RateLineID: lineID,
		ComponentType: domain.ComponentTypeDetention, CalculationMethod: domain.CalcMethodUnitRate,
		Amount: dec(amount), UnitCode: &unit, Actor: e.Actor,
	}, nil)
	if err != nil {
		t.Fatalf("add detention: %v", err)
	}
}

func (e *testEnv) activateVersion(t *testing.T, versionID uuid.UUID) *domain.RateCardVersion {
	t.Helper()
	version, err := e.RateCards.ActivateVersion(context.Background(), e.TenantID, versionID, e.Actor, nil)
	if err != nil {
		t.Fatalf("activate version: %v", err)
	}
	return version
}

func (e *testEnv) resolveReq(equipment string) domain.ResolveRateRequest {
	return domain.ResolveRateRequest{
		TenantID: e.TenantID, BuyerCompanyID: e.BuyerID, CarrierCompanyID: e.CarrierID,
		OriginLocationID: e.OriginID, DestinationLocationID: e.DestID,
		EquipmentType: equipment, TransportMode: domain.TransportModeRoad,
		PricingDate: e.Today, Actor: e.Actor,
	}
}
