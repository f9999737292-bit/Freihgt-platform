//go:build integration

package latesubmission

import (
	"context"
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
	idemRepo       *repository.IdempotencyRepository
	membershipRepo *repository.MembershipRepository
	qRepo          *repository.QuestionnaireRepository
	answerRepo     *repository.AnswerRepository
	lateRepo       *repository.LateSubmissionRepository
	rfxSvc         *service.RfxService
	qSvc           *service.QuestionnaireService
	lateSvc        *service.LateSubmissionService
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
	BuyerRead     domain.ActorContext
	CarrierAct    domain.ActorContext
	CarrierBAct   domain.ActorContext
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
	t.Logf("isolated database=%s", dbName)
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
	idemRepo := repository.NewIdempotencyRepository(pool)
	lateRepo := repository.NewLateSubmissionRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	lateSvc := service.NewLateSubmissionService(pool, lateRepo, rfxRepo, idemRepo, auditRepo, rfxSvc)
	crSvc := service.NewCarrierResponseServiceWithLateSubmission(pool, rfxRepo, answerRepo, qRepo, auditRepo, membershipRepo, rfxSvc, nil, lateSvc, idemRepo)
	return &testEnv{
		pool: pool, rfxRepo: rfxRepo, auditRepo: auditRepo, membershipRepo: membershipRepo,
		qRepo: qRepo, answerRepo: answerRepo, idemRepo: idemRepo, lateRepo: lateRepo,
		rfxSvc: rfxSvc, qSvc: qSvc, lateSvc: lateSvc, crSvc: crSvc,
	}
}

func seedBuyerFixture(t *testing.T, env *testEnv) buyerFixture {
	t.Helper()
	ctx := context.Background()
	fix := buyerFixture{
		TenantID: uuid.New(), OtherTenantID: uuid.New(), CompanyA: uuid.New(),
		CompanyB: uuid.New(), CarrierID: uuid.New(), CarrierBID: uuid.New(),
	}
	fix.BuyerA = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.BuyerB = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.BuyerRead = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CarrierAct = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CarrierBAct = domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	fix.CrossTenant = domain.ActorContext{TenantID: fix.OtherTenantID, UserID: uuid.New()}
	for _, tenant := range []struct {
		id, code string
	}{
		{fix.TenantID.String(), "t-main"}, {fix.OtherTenantID.String(), "t-other"},
	} {
		id := uuid.MustParse(tenant.id)
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`, id, tenant.code, tenant.code); err != nil {
			t.Fatalf("seed tenant: %v", err)
		}
	}
	for _, c := range []struct {
		id, tenant uuid.UUID
		name, typ  string
	}{
		{fix.CompanyA, fix.TenantID, "Buyer A", "SHIPPER"},
		{fix.CompanyB, fix.TenantID, "Buyer B", "SHIPPER"},
		{fix.CarrierID, fix.TenantID, "Carrier A", "CARRIER"},
		{fix.CarrierBID, fix.TenantID, "Carrier B", "CARRIER"},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`, c.id, c.tenant, c.name, c.typ); err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
	var buyerManageRole, buyerReadRole, carrierRole uuid.UUID
	for _, q := range []struct {
		dest *uuid.UUID
		code string
	}{
		{&buyerManageRole, "PROCUREMENT_MANAGER"},
		{&buyerReadRole, "SHIPPER_LOGIST"},
		{&carrierRole, "CARRIER_DISPATCHER"},
	} {
		if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = $1 LIMIT 1`, q.code).Scan(q.dest); err != nil {
			t.Fatalf("lookup role %s: %v", q.code, err)
		}
	}
	users := []struct {
		act     *domain.ActorContext
		company uuid.UUID
		role    uuid.UUID
		email   string
	}{
		{&fix.BuyerA, fix.CompanyA, buyerManageRole, "buyer-manage@test.local"},
		{&fix.BuyerB, fix.CompanyB, buyerManageRole, "buyer-b@test.local"},
		{&fix.BuyerRead, fix.CompanyA, buyerReadRole, "buyer-read@test.local"},
		{&fix.CarrierAct, fix.CarrierID, carrierRole, "carrier-a@test.local"},
		{&fix.CarrierBAct, fix.CarrierBID, carrierRole, "carrier-b@test.local"},
	}
	for _, u := range users {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`, u.act.UserID, fix.TenantID, u.email, u.email); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`, fix.TenantID, u.company, u.act.UserID); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`, fix.TenantID, u.act.UserID, u.company, u.role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}
	return fix
}

func seedPublishedEventAfterDeadline(t *testing.T, env *testEnv, fix buyerFixture) (*domain.RfxEvent, *domain.Question) {
	t.Helper()
	return seedPublishedEventAfterDeadlineAt(t, env, fix, time.Now().UTC())
}

func seedPublishedEventAfterDeadlineAt(t *testing.T, env *testEnv, fix buyerFixture, anchor time.Time) (*domain.RfxEvent, *domain.Question) {
	t.Helper()
	ctx := context.Background()
	scenarioNow := anchor.UTC()
	expiredDeadline := scenarioNow.Add(-2 * time.Hour)
	creationBase := time.Now().UTC()
	if scenarioNow.After(creationBase) {
		creationBase = scenarioNow
	}
	creationDeadline := creationBase.Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Late Submission Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-LS-" + uuid.NewString()[:8],
		ResponseDeadline: &creationDeadline,
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
	required := true
	q, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes", Required: required,
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET status='PUBLISHED', published_at=now() WHERE id=$1`, version.ID); err != nil {
		t.Fatalf("publish version: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response before late flow: %v", err)
	}
	_, err = env.pool.Exec(ctx, `UPDATE rfx.rfx_events SET response_deadline = $2 WHERE id = $1`, event.ID, expiredDeadline)
	if err != nil {
		t.Fatalf("set past deadline: %v", err)
	}
	_ = ws
	reloaded, err := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	return reloaded, q
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s", code)
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func approveRequest(t *testing.T, env *testEnv, fix buyerFixture, eventID, requestID uuid.UUID, version int, from, until time.Time) *domain.LateSubmissionRequest {
	t.Helper()
	out, err := env.lateSvc.Approve(context.Background(), fix.BuyerA, eventID, requestID, uuid.NewString(), domain.ApproveLateSubmissionInput{
		ExpectedVersion: version, ApprovedValidFrom: from, ApprovedValidUntil: until, DecisionComment: "approved",
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return out
}

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	dbName = "rfx_late_submission_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
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
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("migration %s: %w", base, execErr)
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
