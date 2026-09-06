//go:build integration

package scoringv3

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
	answerRepo     *repository.AnswerRepository
	scoreRepo      *repository.ScoreRepository
	rfxSvc         *service.RfxService
	qSvc           *service.QuestionnaireService
	scoreModelSvc  *service.ScoreModelService
	scoringSvc     *service.ScoringService
	crSvc          *service.CarrierResponseService
}

type buyerFixture struct {
	TenantID      uuid.UUID
	OtherTenantID uuid.UUID
	CompanyA      uuid.UUID
	CompanyB      uuid.UUID
	CarrierID     uuid.UUID
	CarrierBID    uuid.UUID
	BuyerA        domain.ActorContext
	BuyerB        domain.ActorContext
	CarrierAct    domain.ActorContext
	CarrierBAct   domain.ActorContext
	CrossTenant   domain.ActorContext
	NonMember     domain.ActorContext
}

type scoringFixture struct {
	Event         *domain.RfxEvent
	VersionID     uuid.UUID
	ADRQuestion   *domain.Question
	FleetQuestion *domain.Question
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
	answerRepo := repository.NewAnswerRepository(pool)
	scoreRepo := repository.NewScoreRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	scoringSvc := service.NewScoringService(pool, rfxRepo, answerRepo, qRepo, scoreRepo, auditRepo)
	crSvc := service.NewCarrierResponseServiceWithScoring(pool, rfxRepo, answerRepo, qRepo, auditRepo, membershipRepo, rfxSvc, scoringSvc)
	scoreModelSvc := service.NewScoreModelService(rfxRepo, scoreRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	t.Logf("isolated database=%s", dbName)
	return &testEnv{
		pool: pool, rfxRepo: rfxRepo, auditRepo: auditRepo, membershipRepo: membershipRepo,
		qRepo: qRepo, answerRepo: answerRepo, scoreRepo: scoreRepo,
		rfxSvc: rfxSvc, qSvc: qSvc, scoreModelSvc: scoreModelSvc, scoringSvc: scoringSvc, crSvc: crSvc,
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
		CarrierBID:    uuid.New(),
	}
	fix.BuyerA = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.BuyerB = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CarrierAct = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CarrierBAct = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CrossTenant = domain.ActorContext{TenantID: fix.OtherTenantID, UserID: uuid.New()}
	fix.NonMember = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}

	for _, tenant := range []struct {
		id   uuid.UUID
		code string
	}{
		{fix.TenantID, "t-main"},
		{fix.OtherTenantID, "t-other"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`,
			tenant.id, tenant.code, tenant.code); err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}
	for _, company := range []struct {
		id, tenant uuid.UUID
		name, typ  string
	}{
		{fix.CompanyA, fix.TenantID, "Buyer A", "SHIPPER"},
		{fix.CompanyB, fix.TenantID, "Buyer B", "SHIPPER"},
		{fix.CarrierID, fix.TenantID, "Carrier A", "CARRIER"},
		{fix.CarrierBID, fix.TenantID, "Carrier B", "CARRIER"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
			company.id, company.tenant, company.name, company.typ); err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	for _, user := range []struct {
		id, tenant uuid.UUID
		email      string
	}{
		{fix.BuyerA.UserID, fix.TenantID, "buyer-a@test.local"},
		{fix.BuyerB.UserID, fix.TenantID, "buyer-b@test.local"},
		{fix.CarrierAct.UserID, fix.TenantID, "carrier-a@test.local"},
		{fix.CarrierBAct.UserID, fix.TenantID, "carrier-b@test.local"},
		{fix.NonMember.UserID, fix.TenantID, "no-member@test.local"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
			user.id, user.tenant, user.email, user.email); err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	var buyerRoleID, carrierRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&buyerRoleID); err != nil {
		t.Fatalf("lookup buyer role: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'CARRIER_DISPATCHER' LIMIT 1`).Scan(&carrierRoleID); err != nil {
		t.Fatalf("lookup carrier role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, fix.NonMember.UserID, fix.CompanyA, buyerRoleID); err != nil {
		t.Fatalf("seed non-member buyer role without membership: %v", err)
	}
	for _, m := range []struct {
		tenant, company, user, role uuid.UUID
	}{
		{fix.TenantID, fix.CompanyA, fix.BuyerA.UserID, buyerRoleID},
		{fix.TenantID, fix.CompanyB, fix.BuyerB.UserID, buyerRoleID},
		{fix.TenantID, fix.CarrierID, fix.CarrierAct.UserID, carrierRoleID},
		{fix.TenantID, fix.CarrierBID, fix.CarrierBAct.UserID, carrierRoleID},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
			m.tenant, m.company, m.user); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
			m.tenant, m.user, m.company, m.role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}
	return fix
}

func seedScoringFixture(t *testing.T, env *testEnv, fix buyerFixture) scoringFixture {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Scoring Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-SCORE-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	for _, carrier := range []uuid.UUID{fix.CarrierID, fix.CarrierBID} {
		if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
			TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: carrier, ParticipantType: "CARRIER",
		}); err != nil {
			t.Fatalf("add participant: %v", err)
		}
	}
	version, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("draft version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled = TRUE WHERE id = $1`, version.ID); err != nil {
		t.Fatalf("enable questionnaire: %v", err)
	}
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{SectionCode: "MAIN", Title: "Main"})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	adr, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_AVAILABLE", QuestionType: domain.QuestionTypeYesNo, Label: "ADR Available", Required: true,
	})
	if err != nil {
		t.Fatalf("adr question: %v", err)
	}
	fleet, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET_COUNT", QuestionType: domain.QuestionTypeNumber, Label: "Fleet Count", Required: true,
	})
	if err != nil {
		t.Fatalf("fleet question: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return scoringFixture{Event: event, VersionID: version.ID, ADRQuestion: adr, FleetQuestion: fleet}
}

func putDeterministicScoreModel(t *testing.T, env *testEnv, fix buyerFixture, sf scoringFixture) {
	t.Helper()
	ctx := context.Background()
	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, sf.Event.ID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{CriterionCode: "HSE", Name: "HSE", Weight: 40, SortOrder: 1,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`)},
			{CriterionCode: "CAPACITY", Name: "Capacity", Weight: 60, SortOrder: 2,
				NormalizationJSON: json.RawMessage(`{"type":"NUMBER_LINEAR","min":0,"max":100}`)},
		},
		Bindings: []domain.ScoreBindingInput{
			{CriterionCode: "HSE", QuestionCode: "ADR_AVAILABLE",
				KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`)},
			{CriterionCode: "CAPACITY", QuestionCode: "FLEET_COUNT"},
		},
	})
	if err != nil {
		t.Fatalf("put score model: %v", err)
	}
	if _, err := env.scoreModelSvc.PublishScoreModel(ctx, fix.BuyerA, sf.Event.ID); err != nil {
		t.Fatalf("publish score model: %v", err)
	}
}

func publishVersion(t *testing.T, env *testEnv, versionID uuid.UUID) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(), `UPDATE rfx.rfx_versions SET status='PUBLISHED', published_at=now() WHERE id=$1`, versionID); err != nil {
		t.Fatalf("publish version: %v", err)
	}
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s", code)
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected code %s, got %v", code, err)
	}
}

func countQualificationResults(ctx context.Context, pool *pgxpool.Pool, responseID, tenantID uuid.UUID) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_qualification_results WHERE rfx_response_id=$1 AND tenant_id=$2`, responseID, tenantID).Scan(&count)
	return count, err
}

func countAnswerScores(ctx context.Context, pool *pgxpool.Pool, responseID, tenantID uuid.UUID) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_answer_scores WHERE rfx_response_id=$1 AND tenant_id=$2`, responseID, tenantID).Scan(&count)
	return count, err
}

func countAuditByAction(ctx context.Context, pool *pgxpool.Pool, tenantID, entityID uuid.UUID, action string) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id=$1 AND entity_id=$2 AND action=$3`, tenantID, entityID, action).Scan(&count)
	return count, err
}

func hasReadinessCode(result domain.ScoreModelReadinessResult, code string) bool {
	for _, e := range result.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
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
	dbName = "rfx_scoring_v3_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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
	testURL = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		url.QueryEscape(testCfg.ConnConfig.User),
		url.QueryEscape(testCfg.ConnConfig.Password),
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, testCfg.ConnConfig.Database)
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
	excludeVersion := migrationFileVersion(excludeBaseName)
	for _, file := range files {
		base := filepath.Base(file)
		if excludeBaseName != "" {
			if base == excludeBaseName {
				continue
			}
			if excludeVersion > 0 && migrationFileVersion(base) >= excludeVersion {
				continue
			}
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

func migrationFileVersion(baseName string) int {
	if baseName == "" {
		return 0
	}
	var version int
	if _, err := fmt.Sscanf(baseName, "%06d", &version); err != nil {
		return 0
	}
	return version
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
		t.Fatalf("connect: %v", err)
	}
	if err := applyMigrationsExcept(ctx, pool, "000067_rfx_scoring_v3_0d.up.sql"); err != nil {
		pool.Close()
		dropDB(context.Background())
		t.Fatalf("apply pre-067 migrations: %v", err)
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
	answerRepo := repository.NewAnswerRepository(pool)
	scoreRepo := repository.NewScoreRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	scoringSvc := service.NewScoringService(pool, rfxRepo, answerRepo, qRepo, scoreRepo, auditRepo)
	crSvc := service.NewCarrierResponseServiceWithScoring(pool, rfxRepo, answerRepo, qRepo, auditRepo, membershipRepo, rfxSvc, scoringSvc)
	scoreModelSvc := service.NewScoreModelService(rfxRepo, scoreRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	t.Logf("legacy migration database=%s", dbName)
	return &testEnv{
		pool: pool, rfxRepo: rfxRepo, auditRepo: auditRepo, membershipRepo: membershipRepo,
		qRepo: qRepo, answerRepo: answerRepo, scoreRepo: scoreRepo,
		rfxSvc: rfxSvc, qSvc: qSvc, scoreModelSvc: scoreModelSvc, scoringSvc: scoringSvc, crSvc: crSvc,
	}, cleanup
}
