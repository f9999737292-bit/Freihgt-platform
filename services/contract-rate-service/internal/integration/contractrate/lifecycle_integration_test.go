//go:build integration

package contractrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/repository"
)

const maxMigrationNumber = 48

func setupEnv(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, cleanup := createTempDB(t, ctx, adminURL)
	t.Cleanup(cleanup)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	seedCompanies(t, ctx, pool, tenantID, buyerID, carrierID)
	return pool, tenantID, buyerID, carrierID
}

func TestContractLifecycleIntegration(t *testing.T) {
	pool, tenantID, buyerID, carrierID := setupEnv(t)
	ctx := context.Background()
	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	rateCards := repository.NewRateCardRepository(pool, contracts, audit)
	actor := domain.ActorInput{TenantID: tenantID, ActorUserID: uuid.New(), ActorCompanyID: buyerID, ActorKind: domain.ActorKindBuyer}
	today := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	validTo := today.AddDate(0, 6, 0)

	created, err := contracts.Create(ctx, domain.CreateContractInput{
		TenantID: tenantID, BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
		ContractNumber: "CTR-001", Name: "Main", ValidFrom: today, ValidTo: &validTo, CurrencyCode: "RUB", Actor: actor,
	}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := contracts.Create(ctx, domain.CreateContractInput{
		TenantID: tenantID, BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
		ContractNumber: "CTR-001", Name: "Dup", ValidFrom: today, CurrencyCode: "RUB", Actor: actor,
	}, nil); err == nil {
		t.Fatal("duplicate contract number must deny")
	}
	otherTenant := uuid.New()
	if _, err := contracts.GetByIDAndTenant(ctx, otherTenant, created.ID); err == nil {
		t.Fatal("cross-tenant read must deny")
	}
	active, err := contracts.Activate(ctx, tenantID, created.ID, actor, nil)
	if err != nil || active.Status != domain.ContractStatusActive {
		t.Fatalf("activate: %v status=%s", err, active.Status)
	}
	active2, err := contracts.Activate(ctx, tenantID, created.ID, actor, nil)
	if err != nil || active2.Status != domain.ContractStatusActive {
		t.Fatalf("idempotent activate: %v", err)
	}
	suspended, err := contracts.Suspend(ctx, tenantID, created.ID, actor, nil)
	if err != nil || suspended.Status != domain.ContractStatusSuspended {
		t.Fatalf("suspend: %v", err)
	}
	reactivated, err := contracts.Reactivate(ctx, tenantID, created.ID, actor, nil)
	if err != nil || reactivated.Status != domain.ContractStatusActive {
		t.Fatalf("reactivate: %v", err)
	}
	card, err := rateCards.Create(ctx, domain.CreateRateCardInput{TenantID: tenantID, ContractID: created.ID, Name: "Card 1", Actor: actor}, nil)
	if err != nil {
		t.Fatalf("rate card create: %v", err)
	}
	v1, err := rateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{TenantID: tenantID, RateCardID: card.ID, ValidFrom: today, Actor: actor}, nil)
	if err != nil || v1.VersionNumber != 1 {
		t.Fatalf("draft version: %v", err)
	}
	terminated, err := contracts.Terminate(ctx, tenantID, created.ID, actor, nil, nil)
	if err != nil || terminated.Status != domain.ContractStatusTerminated {
		t.Fatalf("terminate: %v", err)
	}
	if _, err := contracts.PatchMetadata(ctx, tenantID, created.ID, domain.PatchContractMetadataInput{Description: strPtr("x"), Actor: actor}, nil); err == nil {
		t.Fatal("terminated mutation must deny")
	}

	draft, err := contracts.Create(ctx, domain.CreateContractInput{
		TenantID: tenantID, BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
		ContractNumber: "CTR-002", Name: "Draft", ValidFrom: today, CurrencyCode: "RUB", Actor: actor,
	}, nil)
	if err != nil {
		t.Fatalf("draft create: %v", err)
	}
	cancelled, err := contracts.Cancel(ctx, tenantID, draft.ID, actor, nil)
	if err != nil || cancelled.Status != domain.ContractStatusCancelled {
		t.Fatalf("cancel: %v", err)
	}
}

func TestOneActiveVersionDBConstraint(t *testing.T) {
	pool, tenantID, buyerID, carrierID := setupEnv(t)
	ctx := context.Background()
	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	rateCards := repository.NewRateCardRepository(pool, contracts, audit)
	actor := domain.ActorInput{TenantID: tenantID, ActorUserID: uuid.New(), ActorCompanyID: buyerID, ActorKind: domain.ActorKindBuyer}
	today := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	contract, _ := contracts.Create(ctx, domain.CreateContractInput{
		TenantID: tenantID, BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
		ContractNumber: "CTR-DB", Name: "DB", ValidFrom: today, CurrencyCode: "RUB", Actor: actor,
	}, nil)
	card, err := rateCards.Create(ctx, domain.CreateRateCardInput{TenantID: tenantID, ContractID: contract.ID, Name: "Card", Actor: actor}, nil)
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	v1, err := rateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{TenantID: tenantID, RateCardID: card.ID, ValidFrom: today, Actor: actor}, nil)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE contract_rate.rate_card_version SET status='ACTIVE', activated_at=now() WHERE id=$1`, v1.ID)
	if err != nil {
		t.Fatalf("activate v1: %v", err)
	}
	v2, err := rateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{TenantID: tenantID, RateCardID: card.ID, ValidFrom: today, Actor: actor}, nil)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE contract_rate.rate_card_version SET status='ACTIVE', activated_at=now() WHERE id=$1`, v2.ID)
	if err == nil {
		t.Fatal("second ACTIVE version must be rejected by DB")
	}
}

func TestConcurrentDraftVersionNumbers(t *testing.T) {
	pool, tenantID, buyerID, carrierID := setupEnv(t)
	ctx := context.Background()
	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	rateCards := repository.NewRateCardRepository(pool, contracts, audit)
	actor := domain.ActorInput{TenantID: tenantID, ActorUserID: uuid.New(), ActorCompanyID: buyerID, ActorKind: domain.ActorKindBuyer}
	today := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	contract, _ := contracts.Create(ctx, domain.CreateContractInput{
		TenantID: tenantID, BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
		ContractNumber: "CTR-CONC", Name: "Conc", ValidFrom: today, CurrencyCode: "RUB", Actor: actor,
	}, nil)
	card, _ := rateCards.Create(ctx, domain.CreateRateCardInput{TenantID: tenantID, ContractID: contract.ID, Name: "Card", Actor: actor}, nil)

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := rateCards.CreateDraftVersion(ctx, domain.CreateRateVersionInput{TenantID: tenantID, RateCardID: card.ID, ValidFrom: today, Actor: actor}, nil)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	versions, err := rateCards.ListVersions(ctx, tenantID, card.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 4 {
		t.Fatalf("expected 4 versions, got %d", len(versions))
	}
	nums := map[int]bool{}
	for _, v := range versions {
		if nums[v.VersionNumber] {
			t.Fatalf("duplicate version number %d", v.VersionNumber)
		}
		nums[v.VersionNumber] = true
	}
}

func TestAuditEmitted(t *testing.T) {
	pool, tenantID, buyerID, carrierID := setupEnv(t)
	ctx := context.Background()
	audit := repository.NewAuditRepository()
	contracts := repository.NewContractRepository(pool, audit)
	actor := domain.ActorInput{TenantID: tenantID, ActorUserID: uuid.New(), ActorCompanyID: buyerID, ActorKind: domain.ActorKindBuyer}
	today := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	created, _ := contracts.Create(ctx, domain.CreateContractInput{
		TenantID: tenantID, BuyerCompanyID: buyerID, CarrierCompanyID: carrierID,
		ContractNumber: "CTR-AUD", Name: "Audit", ValidFrom: today, CurrencyCode: "RUB", Actor: actor,
	}, nil)
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM contract_rate.audit_event WHERE tenant_id=$1 AND entity_id=$2`, tenantID, created.ID).Scan(&count); err != nil {
		t.Fatalf("audit count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected audit row, got %d", count)
	}
}

func seedCompanies(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, buyerID, carrierID uuid.UUID) {
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

var _ = pgx.ErrNoRows
