//go:build integration

package questionnaire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

type testEnv struct {
	pool           *pgxpool.Pool
	rfxRepo        *repository.RfxRepository
	auditRepo      *repository.AuditRepository
	membershipRepo *repository.MembershipRepository
	qRepo          *repository.QuestionnaireRepository
	rfxSvc         *service.RfxService
	qSvc           *service.QuestionnaireService
}

type buyerFixture struct {
	TenantID      uuid.UUID
	OtherTenantID uuid.UUID
	CompanyA      uuid.UUID
	CompanyB      uuid.UUID
	CarrierID     uuid.UUID
	BuyerA        domain.ActorContext
	BuyerB        domain.ActorContext
	CarrierAct    domain.ActorContext
	NoMembership  domain.ActorContext
	CrossTenant   domain.ActorContext
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres unavailable: %v", err)
	}

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		dropDB(context.Background())
	})

	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	rfxRepo := repository.NewRfxRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	qRepo := repository.NewQuestionnaireRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	t.Logf("isolated database=%s", dbName)
	return &testEnv{
		pool:           pool,
		rfxRepo:        rfxRepo,
		auditRepo:      auditRepo,
		membershipRepo: membershipRepo,
		qRepo:          qRepo,
		rfxSvc:         rfxSvc,
		qSvc:           qSvc,
	}
}

func seedBuyerFixture(t *testing.T, env *testEnv) buyerFixture {
	t.Helper()
	ctx := context.Background()
	fix := buyerFixture{
		TenantID:      uuid.New(),
		OtherTenantID: uuid.New(),
		CompanyA:      uuid.New(),
		CompanyB:      uuid.New(),
		CarrierID:     uuid.New(),
	}
	fix.BuyerA = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.BuyerB = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CarrierAct = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.NoMembership = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CrossTenant = domain.ActorContext{TenantID: fix.OtherTenantID, UserID: uuid.New()}

	for _, tenant := range []struct {
		id   uuid.UUID
		code string
	}{
		{fix.TenantID, "t-" + fix.TenantID.String()[:8]},
		{fix.OtherTenantID, "t-" + fix.OtherTenantID.String()[:8]},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
			tenant.id, tenant.code, "Questionnaire Tenant"); err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}
	for _, company := range []struct {
		tenantID uuid.UUID
		id       uuid.UUID
		name     string
		typ      string
	}{
		{fix.TenantID, fix.CompanyA, "Company A", "SHIPPER"},
		{fix.TenantID, fix.CompanyB, "Company B", "SHIPPER"},
		{fix.TenantID, fix.CarrierID, "Carrier C", "CARRIER"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
			company.id, company.tenantID, company.name, company.typ); err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, user := range []struct {
		tenantID uuid.UUID
		id       uuid.UUID
		email    string
	}{
		{fix.TenantID, fix.BuyerA.UserID, "buyer-a@test.local"},
		{fix.TenantID, fix.BuyerB.UserID, "buyer-b@test.local"},
		{fix.TenantID, fix.CarrierAct.UserID, "carrier@test.local"},
		{fix.TenantID, fix.NoMembership.UserID, "no-member@test.local"},
		{fix.OtherTenantID, fix.CrossTenant.UserID, "cross-tenant@test.local"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
			user.id, user.tenantID, user.email, user.email); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3), ($1, $4, $5), ($1, $6, $7)`,
		fix.TenantID, fix.CompanyA, fix.BuyerA.UserID, fix.CompanyB, fix.BuyerB.UserID, fix.CarrierID, fix.CarrierAct.UserID); err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	var buyerRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&buyerRoleID); err != nil {
		t.Fatalf("lookup buyer role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4), ($1, $5, $6, $4), ($1, $7, $8, $4)`,
		fix.TenantID, fix.BuyerA.UserID, fix.CompanyA, buyerRoleID, fix.BuyerB.UserID, fix.CompanyB, fix.NoMembership.UserID, fix.CompanyA); err != nil {
		t.Fatalf("seed buyer roles: %v", err)
	}

	var carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, fix.CarrierAct.UserID, fix.CarrierID, carrierRoleID); err != nil {
		t.Fatalf("seed carrier role: %v", err)
	}
	return fix
}

func createTestEvent(t *testing.T, env *testEnv, actor domain.ActorContext, tenantID, ownerCompanyID uuid.UUID, rfxNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	event, err := env.rfxSvc.CreateEvent(ctx, actor, domain.CreateRfxEventInput{
		TenantID: tenantID, OwnerCompanyID: ownerCompanyID, Title: "Questionnaire Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: rfxNumber,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	return event.ID
}

func createDraftEvent(t *testing.T, env *testEnv, fix buyerFixture, rfxNumber string) *domain.RfxEvent {
	t.Helper()
	ctx := context.Background()
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID:       fix.TenantID,
		OwnerCompanyID: fix.CompanyA,
		Title:          "Questionnaire Test Event",
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		RfxNumber:      rfxNumber,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	return event
}

func seedForeignTenantBuyer(t *testing.T, env *testEnv) (uuid.UUID, domain.ActorContext) {
	t.Helper()
	ctx := context.Background()
	tenantID := uuid.New()
	companyID := uuid.New()
	userID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
		tenantID, "t-foreign", "Foreign Tenant"); err != nil {
		t.Fatalf("seed foreign tenant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
		companyID, tenantID, "Foreign Buyer", "SHIPPER"); err != nil {
		t.Fatalf("seed foreign company: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		userID, tenantID, "foreign@test.local", "foreign@test.local"); err != nil {
		t.Fatalf("seed foreign user: %v", err)
	}
	var roleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&roleID); err != nil {
		t.Fatalf("lookup role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		tenantID, companyID, userID); err != nil {
		t.Fatalf("seed foreign membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		tenantID, userID, companyID, roleID); err != nil {
		t.Fatalf("seed foreign role: %v", err)
	}
	return tenantID, domain.ActorContext{TenantID: tenantID, UserID: userID}
}

func publishVersion(t *testing.T, env *testEnv, versionID uuid.UUID) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_versions SET status='PUBLISHED', published_at=now() WHERE id=$1`, versionID); err != nil {
		t.Fatalf("publish version: %v", err)
	}
}

func enableQuestionnaireByVersionID(t *testing.T, env *testEnv, tenantID, versionID uuid.UUID) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_versions SET questionnaire_enabled=true WHERE id=$1 AND tenant_id=$2`, versionID, tenantID); err != nil {
		t.Fatalf("enable questionnaire: %v", err)
	}
}

func countRules(ctx context.Context, pool *pgxpool.Pool, tenantID, versionID uuid.UUID) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_question_rules WHERE tenant_id=$1 AND rfx_version_id=$2 AND deleted_at IS NULL`, tenantID, versionID).Scan(&count)
	return count, err
}

func enableQuestionnaire(t *testing.T, env *testEnv, actor domain.ActorContext, eventID uuid.UUID) *domain.RfxVersion {
	t.Helper()
	ctx := context.Background()
	version, err := env.qRepo.GetOrCreateDraftVersion(ctx, actor.TenantID, eventID)
	if err != nil {
		t.Fatalf("get draft version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled = TRUE WHERE id = $1 AND tenant_id = $2`,
		version.ID, actor.TenantID); err != nil {
		t.Fatalf("enable questionnaire: %v", err)
	}
	version.QuestionnaireEnabled = true
	return version
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	adminDB := cfg.ConnConfig.Database
	if adminDB == "" {
		adminDB = "postgres"
	}
	dbName = "rfx_questionnaire_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = adminDB
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return "", "", nil, err
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", "", nil, err
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
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(cfg.ConnConfig.User),
		url.QueryEscape(cfg.ConnConfig.Password),
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return applyMigrationsExcept(ctx, pool, "")
}

func applyMigrationsExcept(ctx context.Context, pool *pgxpool.Pool, excludeBaseName string) error {
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
		if excludeBaseName != "" && base == excludeBaseName {
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

func applyMigrationFile(ctx context.Context, pool *pgxpool.Pool, baseName string) error {
	migrationsDir, err := locateMigrationsDir()
	if err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(migrationsDir, baseName))
	if err != nil {
		return err
	}
	if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
		return fmt.Errorf("apply %s: %w", baseName, execErr)
	}
	return nil
}

func setupLegacyMigrationTestEnv(t *testing.T) (*testEnv, func()) {
	t.Helper()
	adminURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if adminURL == "" {
		if os.Getenv("REQUIRE_TEST_DATABASE") == "1" || strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI")
		}
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	dbName, testURL, dropDB, err := createTempDatabase(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated postgres unavailable: %v", err)
	}

	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		dropDB(context.Background())
		t.Fatalf("connect test database: %v", err)
	}

	if err := applyMigrationsExcept(ctx, pool, "000065_rfx_questionnaire_v3_0b.up.sql"); err != nil {
		pool.Close()
		dropDB(context.Background())
		t.Fatalf("apply pre-065 migrations: %v", err)
	}

	cleanup := func() {
		pool.Close()
		dropDB(context.Background())
	}
	t.Cleanup(cleanup)

	rfxRepo := repository.NewRfxRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	qRepo := repository.NewQuestionnaireRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	t.Logf("legacy migration database=%s", dbName)
	return &testEnv{
		pool:           pool,
		rfxRepo:        rfxRepo,
		auditRepo:      auditRepo,
		membershipRepo: membershipRepo,
		qRepo:          qRepo,
		rfxSvc:         rfxSvc,
		qSvc:           qSvc,
	}, cleanup
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

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected code %s, got %v", code, err)
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func conditionEquals(sourceCode string, value any) json.RawMessage {
	return mustJSON(map[string]any{
		"operator":             domain.ConditionOperatorEquals,
		"source_question_code": sourceCode,
		"value":                value,
	})
}

func conditionIsNotEmpty(sourceCode string) json.RawMessage {
	return mustJSON(map[string]any{
		"operator":             domain.ConditionOperatorIsNotEmpty,
		"source_question_code": sourceCode,
	})
}

func conditionGreaterThan(sourceCode string, value float64) json.RawMessage {
	return mustJSON(map[string]any{
		"operator":             domain.ConditionOperatorGreaterThan,
		"source_question_code": sourceCode,
		"value":                value,
	})
}

func strPtr(v string) *string { return &v }
