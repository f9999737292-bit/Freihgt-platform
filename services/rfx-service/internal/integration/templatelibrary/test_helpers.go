//go:build integration

package templatelibrary

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
	qRepo          *repository.QuestionnaireRepository
	scoreRepo      *repository.ScoreRepository
	answerRepo     *repository.AnswerRepository
	idemRepo       *repository.IdempotencyRepository
	auditRepo      *repository.AuditRepository
	membershipRepo *repository.MembershipRepository
	tmplRepo       *repository.TemplateLibraryRepository
	tmplQRepo      *repository.TemplateQuestionnaireRepository
	rfxSvc         *service.RfxService
	qSvc           *service.QuestionnaireService
	scoreModelSvc  *service.ScoreModelService
	crSvc          *service.CarrierResponseService
	versionSvc     *service.VersionLifecycleService
	templateSvc    *service.TemplateLibraryService
	templateQSvc   *service.TemplateQuestionnaireService
	cloneSvc       *service.TemplateCloneService
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
	qRepo := repository.NewQuestionnaireRepository(pool)
	answerRepo := repository.NewAnswerRepository(pool)
	idemRepo := repository.NewIdempotencyRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	crSvc := service.NewCarrierResponseService(pool, rfxRepo, answerRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	scoreRepo := repository.NewScoreRepository(pool)
	scoreModelSvc := service.NewScoreModelService(rfxRepo, scoreRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	changeImpactRepo := repository.NewChangeImpactRepository(pool)
	versionSvc := service.NewVersionLifecycleService(pool, rfxRepo, qRepo, scoreRepo, idemRepo, auditRepo, changeImpactRepo, rfxSvc)
	tmplRepo := repository.NewTemplateLibraryRepository(pool)
	tmplQRepo := repository.NewTemplateQuestionnaireRepository(pool)
	templateSvc := service.NewTemplateLibraryService(pool, tmplRepo, tmplQRepo, idemRepo, auditRepo, rfxSvc)
	templateQSvc := service.NewTemplateQuestionnaireService(pool, tmplRepo, tmplQRepo, auditRepo, templateSvc)
	cloneSvc := service.NewTemplateCloneService(pool, rfxRepo, qRepo, tmplRepo, tmplQRepo, idemRepo, auditRepo, rfxSvc, templateSvc)
	t.Logf("isolated database=%s", dbName)

	return &testEnv{
		pool:           pool,
		rfxRepo:        rfxRepo,
		qRepo:          qRepo,
		scoreRepo:      scoreRepo,
		answerRepo:     answerRepo,
		idemRepo:       idemRepo,
		auditRepo:      auditRepo,
		membershipRepo: membershipRepo,
		tmplRepo:       tmplRepo,
		tmplQRepo:      tmplQRepo,
		rfxSvc:         rfxSvc,
		qSvc:           qSvc,
		scoreModelSvc:  scoreModelSvc,
		crSvc:          crSvc,
		versionSvc:     versionSvc,
		templateSvc:    templateSvc,
		templateQSvc:   templateQSvc,
		cloneSvc:       cloneSvc,
	}
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

	if err := applyMigrationsExcept(ctx, pool, "000068_rfx_version_lifecycle_v3_0e1.up.sql"); err != nil {
		pool.Close()
		dropDB(context.Background())
		t.Fatalf("apply pre-068 migrations: %v", err)
	}

	cleanup := func() {
		pool.Close()
		dropDB(context.Background())
	}
	t.Cleanup(cleanup)

	rfxRepo := repository.NewRfxRepository(pool)
	qRepo := repository.NewQuestionnaireRepository(pool)
	answerRepo := repository.NewAnswerRepository(pool)
	idemRepo := repository.NewIdempotencyRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	membershipRepo := repository.NewMembershipRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	crSvc := service.NewCarrierResponseService(pool, rfxRepo, answerRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	scoreRepo := repository.NewScoreRepository(pool)
	scoreModelSvc := service.NewScoreModelService(rfxRepo, scoreRepo, qRepo, auditRepo, membershipRepo, rfxSvc)
	changeImpactRepo := repository.NewChangeImpactRepository(pool)
	versionSvc := service.NewVersionLifecycleService(pool, rfxRepo, qRepo, scoreRepo, idemRepo, auditRepo, changeImpactRepo, rfxSvc)
	tmplRepo := repository.NewTemplateLibraryRepository(pool)
	tmplQRepo := repository.NewTemplateQuestionnaireRepository(pool)
	templateSvc := service.NewTemplateLibraryService(pool, tmplRepo, tmplQRepo, idemRepo, auditRepo, rfxSvc)
	templateQSvc := service.NewTemplateQuestionnaireService(pool, tmplRepo, tmplQRepo, auditRepo, templateSvc)
	cloneSvc := service.NewTemplateCloneService(pool, rfxRepo, qRepo, tmplRepo, tmplQRepo, idemRepo, auditRepo, rfxSvc, templateSvc)
	t.Logf("legacy migration database=%s", dbName)

	return &testEnv{
		pool:           pool,
		rfxRepo:        rfxRepo,
		qRepo:          qRepo,
		scoreRepo:      scoreRepo,
		answerRepo:     answerRepo,
		idemRepo:       idemRepo,
		auditRepo:      auditRepo,
		membershipRepo: membershipRepo,
		tmplRepo:       tmplRepo,
		tmplQRepo:      tmplQRepo,
		rfxSvc:         rfxSvc,
		qSvc:           qSvc,
		scoreModelSvc:  scoreModelSvc,
		crSvc:          crSvc,
		versionSvc:     versionSvc,
		templateSvc:    templateSvc,
		templateQSvc:   templateQSvc,
		cloneSvc:       cloneSvc,
	}, cleanup
}

func defaultCloneEventInput(rfxNumber string, owner uuid.UUID) domain.CreateRfxEventInput {
	return domain.CreateRfxEventInput{
		RfxNumber:      rfxNumber,
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		Title:          "Cloned RFx Event",
		OwnerCompanyID: owner,
	}
}

func cloneFromTemplate(t *testing.T, env *testEnv, actor domain.ActorContext, templateVersionID uuid.UUID, in domain.CreateRfxEventInput, key string) *domain.CloneEventFromTemplateResult {
	t.Helper()
	result, err := env.cloneSvc.CloneEventFromTemplate(context.Background(), actor, templateVersionID, in, key)
	if err != nil {
		t.Fatalf("clone from template: %v", err)
	}
	return result
}

type cloneArtifactCounts struct {
	events      int
	versions    int
	sections    int
	questions   int
	options     int
	rules       int
	auditClone  int
	idempotency int
}

func countCloneArtifacts(ctx context.Context, env *testEnv, fix buyerFixture, rfxNumber, idempotencyKey string, templateVersionID uuid.UUID) (cloneArtifactCounts, error) {
	var out cloneArtifactCounts
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id=$1 AND rfx_number=$2`, fix.TenantID, rfxNumber).Scan(&out.events); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_versions v
		JOIN rfx.rfx_events e ON e.id = v.rfx_event_id AND e.tenant_id = v.tenant_id
		WHERE e.tenant_id=$1 AND e.rfx_number=$2`, fix.TenantID, rfxNumber).Scan(&out.versions); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_sections s
		JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id AND v.tenant_id = s.tenant_id
		JOIN rfx.rfx_events e ON e.id = v.rfx_event_id AND e.tenant_id = v.tenant_id
		WHERE e.tenant_id=$1 AND e.rfx_number=$2`, fix.TenantID, rfxNumber).Scan(&out.sections); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_questions q
		JOIN rfx.rfx_sections s ON s.id = q.section_id AND s.tenant_id = q.tenant_id
		JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id AND v.tenant_id = s.tenant_id
		JOIN rfx.rfx_events e ON e.id = v.rfx_event_id AND e.tenant_id = v.tenant_id
		WHERE e.tenant_id=$1 AND e.rfx_number=$2`, fix.TenantID, rfxNumber).Scan(&out.questions); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_question_options o
		JOIN rfx.rfx_questions q ON q.id = o.question_id AND q.tenant_id = o.tenant_id
		JOIN rfx.rfx_sections s ON s.id = q.section_id AND s.tenant_id = q.tenant_id
		JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id AND v.tenant_id = s.tenant_id
		JOIN rfx.rfx_events e ON e.id = v.rfx_event_id AND e.tenant_id = v.tenant_id
		WHERE e.tenant_id=$1 AND e.rfx_number=$2`, fix.TenantID, rfxNumber).Scan(&out.options); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_question_rules r
		JOIN rfx.rfx_versions v ON v.id = r.rfx_version_id AND v.tenant_id = r.tenant_id
		JOIN rfx.rfx_events e ON e.id = v.rfx_event_id AND e.tenant_id = v.tenant_id
		WHERE e.tenant_id=$1 AND e.rfx_number=$2`, fix.TenantID, rfxNumber).Scan(&out.rules); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.audit_events
		WHERE tenant_id=$1 AND action='rfx.event.created_from_template.v1'
			AND metadata->>'event_id' IN (
				SELECT id::text FROM rfx.rfx_events WHERE tenant_id=$1 AND rfx_number=$2
			)`, fix.TenantID, rfxNumber).Scan(&out.auditClone); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records
		WHERE tenant_id=$1 AND aggregate_scope=$2 AND idempotency_key=$3 AND operation=$4 AND expires_at > now()`,
		fix.TenantID, templateVersionID, idempotencyKey, domain.TemplateCloneOperationCloneEventFromTemplate).Scan(&out.idempotency); err != nil {
		return out, err
	}
	return out, nil
}

func assertZeroCloneArtifacts(t *testing.T, env *testEnv, fix buyerFixture, rfxNumber, idempotencyKey string, templateVersionID uuid.UUID) {
	t.Helper()
	counts, err := countCloneArtifacts(context.Background(), env, fix, rfxNumber, idempotencyKey, templateVersionID)
	if err != nil {
		t.Fatalf("count clone artifacts: %v", err)
	}
	if counts.events != 0 || counts.versions != 0 || counts.sections != 0 || counts.questions != 0 ||
		counts.options != 0 || counts.rules != 0 || counts.auditClone != 0 || counts.idempotency != 0 {
		t.Fatalf("expected zero clone artifacts, got events=%d versions=%d sections=%d questions=%d options=%d rules=%d audit=%d idempotency=%d",
			counts.events, counts.versions, counts.sections, counts.questions, counts.options, counts.rules, counts.auditClone, counts.idempotency)
	}
}

func regclassExists(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var regclass *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, name).Scan(&regclass); err != nil {
		return false, err
	}
	return regclass != nil && *regclass != "", nil
}

func assertMigration000071Absent(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, idx := range []string{
		"rfx.idx_rfx_events_source_template_version_id",
		"rfx.uq_rfx_template_versions_tenant_version_id",
	} {
		exists, err := regclassExists(ctx, pool, idx)
		if err != nil {
			t.Fatalf("regclass %s: %v", idx, err)
		}
		if exists {
			t.Fatalf("expected index %s absent after down migration", idx)
		}
	}
	var triggerCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_trigger
		WHERE tgname = 'trg_rfx_events_provenance_immutable'`).Scan(&triggerCount); err != nil {
		t.Fatalf("trigger check: %v", err)
	}
	if triggerCount != 0 {
		t.Fatal("expected provenance trigger absent after down migration")
	}
	var functionCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'rfx' AND p.proname = 'prevent_rfx_event_provenance_mutation'`).Scan(&functionCount); err != nil {
		t.Fatalf("function check: %v", err)
	}
	if functionCount != 0 {
		t.Fatal("expected provenance function absent after down migration")
	}
	var fkCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_constraint
		WHERE conname = 'fk_rfx_events_source_template_version_composite'`).Scan(&fkCount); err != nil {
		t.Fatalf("fk check: %v", err)
	}
	if fkCount != 0 {
		t.Fatal("expected composite provenance FK absent after down migration")
	}
	var columnCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'rfx' AND table_name = 'rfx_events' AND column_name = 'source_template_version_id'`).Scan(&columnCount); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if columnCount != 0 {
		t.Fatal("expected source_template_version_id column absent after down migration")
	}
}

func assertMigration000071Present(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, idx := range []string{
		"rfx.idx_rfx_events_source_template_version_id",
		"rfx.uq_rfx_template_versions_tenant_version_id",
	} {
		exists, err := regclassExists(ctx, pool, idx)
		if err != nil {
			t.Fatalf("regclass %s: %v", idx, err)
		}
		if !exists {
			t.Fatalf("expected index %s present after up migration", idx)
		}
	}
	var columnCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'rfx' AND table_name = 'rfx_events' AND column_name = 'source_template_version_id'`).Scan(&columnCount); err != nil {
		t.Fatalf("column check: %v", err)
	}
	if columnCount != 1 {
		t.Fatal("expected source_template_version_id column present after up migration")
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
	fix.CrossTenant = domain.ActorContext{TenantID: fix.OtherTenantID, UserID: uuid.New()}

	for _, tenant := range []struct {
		id   uuid.UUID
		code string
	}{
		{fix.TenantID, "t-" + fix.TenantID.String()[:8]},
		{fix.OtherTenantID, "t-" + fix.OtherTenantID.String()[:8]},
	} {
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1, $2, $3)`, tenant.id, tenant.code, tenant.code); err != nil {
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
		{fix.CarrierAct.UserID, fix.TenantID, "carrier@test.local"},
		{fix.CrossTenant.UserID, fix.OtherTenantID, "cross@test.local"},
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

	for _, membership := range []struct {
		tenantID  uuid.UUID
		companyID uuid.UUID
		userID    uuid.UUID
		roleID    uuid.UUID
	}{
		{fix.TenantID, fix.CompanyA, fix.BuyerA.UserID, buyerRoleID},
		{fix.TenantID, fix.CompanyB, fix.BuyerB.UserID, buyerRoleID},
		{fix.TenantID, fix.CarrierID, fix.CarrierAct.UserID, carrierRoleID},
		{fix.OtherTenantID, fix.CompanyA, fix.CrossTenant.UserID, buyerRoleID},
	} {
		companyID := membership.companyID
		if membership.tenantID == fix.OtherTenantID {
			companyID = uuid.New()
			if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1, $2, $3, $4)`,
				companyID, membership.tenantID, "Foreign Buyer", "SHIPPER"); err != nil {
				t.Fatalf("seed foreign company: %v", err)
			}
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
			membership.tenantID, companyID, membership.userID); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
			membership.tenantID, membership.userID, companyID, membership.roleID); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}

	return fix
}

func createDraftEvent(t *testing.T, env *testEnv, fix buyerFixture, rfxNumber string) *domain.RfxEvent {
	t.Helper()
	event, err := env.rfxSvc.CreateEvent(context.Background(), fix.BuyerA, domain.CreateRfxEventInput{
		TenantID:       fix.TenantID,
		OwnerCompanyID: fix.CompanyA,
		Title:          "Version Lifecycle Test",
		RfxType:        "SPOT_RFQ",
		Category:       "FREIGHT",
		RfxNumber:      rfxNumber,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	return event
}

func ensurePublishedVersion(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, key string) *domain.RfxVersion {
	t.Helper()
	draft := makeQuestionnaireDraft(t, env, fix, eventID)
	event, err := env.rfxRepo.GetEventByID(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event before publish: %v", err)
	}
	published, err := env.versionSvc.PublishQuestionnaire(context.Background(), fix.BuyerA, eventID, key, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Initial publish",
	})
	if err != nil {
		t.Fatalf("publish questionnaire: %v", err)
	}
	return published
}

func makeQuestionnaireDraft(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) *domain.RfxVersion {
	t.Helper()
	ctx := context.Background()
	version, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, eventID)
	if err != nil {
		t.Fatalf("get draft version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled = TRUE WHERE id = $1 AND tenant_id = $2`, version.ID, fix.TenantID); err != nil {
		t.Fatalf("enable questionnaire: %v", err)
	}
	version, err = env.qRepo.GetVersionByID(ctx, version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft version: %v", err)
	}
	section, err := env.qSvc.CreateSection(ctx, fix.BuyerA, eventID, domain.CreateSectionInput{
		SectionCode: "GENERAL",
		Title:       "General",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, eventID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET_SIZE",
		QuestionType: domain.QuestionTypeText,
		Label:        "Fleet size",
		Required:     true,
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, eventID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "HSE_OK",
		QuestionType: domain.QuestionTypeYesNo,
		Label:        "HSE compliant",
		Required:     false,
	}); err != nil {
		t.Fatalf("create HSE question: %v", err)
	}
	version, err = env.qRepo.GetVersionByID(ctx, version.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload updated draft version: %v", err)
	}
	return version
}

func addParticipantAndOpenResponses(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, eventID, domain.AddRfxParticipantInput{
		TenantID:        fix.TenantID,
		RfxEventID:      eventID,
		CompanyID:       fix.CarrierID,
		ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, eventID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	if _, err := env.rfxSvc.TransitionEvent(ctx, fix.BuyerA, eventID, domain.RfxCommandOpenResponses); err != nil {
		t.Fatalf("open responses: %v", err)
	}
}

func insertQualificationForResponse(t *testing.T, env *testEnv, tenantID uuid.UUID, response *domain.RfxResponse) {
	t.Helper()
	if response.RfxVersionID == nil {
		t.Fatal("response version must be pinned before scoring fixture insert")
	}
	ctx := context.Background()
	modelID := uuid.Nil
	model, err := env.scoreRepo.GetPublishedModelForVersion(ctx, tenantID, *response.RfxVersionID)
	if err != nil {
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			t.Fatalf("load published score model: %v", err)
		}
		modelID = uuid.New()
		if _, err := env.pool.Exec(ctx, `
			INSERT INTO rfx.rfx_score_models (
				id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json
			) VALUES ($1, $2, $3, 1, 'PUBLISHED', 'AUTOMATIC', '{}'::jsonb)
		`, modelID, tenantID, *response.RfxVersionID); err != nil {
			t.Fatalf("insert score model: %v", err)
		}
	} else {
		modelID = model.ID
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_qualification_results (
			id, tenant_id, rfx_response_id, score_model_id, score_model_version,
			status, calculation_status, total_score, knockout_triggered, created_at, updated_at
		) VALUES ($1, $2, $3, $4, 1, 'QUALIFIED', 'CALCULATED', 88.5, FALSE, now(), now())
	`, uuid.New(), tenantID, response.ID, modelID); err != nil {
		t.Fatalf("insert qualification result: %v", err)
	}
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
	dbName = "rfx_version_lifecycle_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func assertAppErrorCode(t *testing.T, err error, code apperrors.Code) *apperrors.AppError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s", code)
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected code %s, got %v", code, err)
	}
	return appErr
}

type compareWriteSnapshot struct {
	versionCount      int
	sectionCount      int
	questionCount     int
	optionCount       int
	ruleCount         int
	scoreModelCount   int
	auditCount        int
	idempotencyCount  int
	versionUpdatedMax time.Time
}

func captureCompareWriteSnapshot(t *testing.T, env *testEnv, tenantID, eventID uuid.UUID) compareWriteSnapshot {
	t.Helper()
	ctx := context.Background()
	snap := compareWriteSnapshot{}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_versions WHERE tenant_id = $1 AND rfx_event_id = $2 AND deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.versionCount); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_sections s
		INNER JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id AND v.tenant_id = s.tenant_id
		WHERE v.tenant_id = $1 AND v.rfx_event_id = $2 AND v.deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.sectionCount); err != nil {
		t.Fatalf("count sections: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_questions q
		INNER JOIN rfx.rfx_sections s ON s.id = q.section_id
		INNER JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id AND v.tenant_id = s.tenant_id
		WHERE v.tenant_id = $1 AND v.rfx_event_id = $2 AND v.deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.questionCount); err != nil {
		t.Fatalf("count questions: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_question_options o
		INNER JOIN rfx.rfx_questions q ON q.id = o.question_id
		INNER JOIN rfx.rfx_sections s ON s.id = q.section_id
		INNER JOIN rfx.rfx_versions v ON v.id = s.rfx_version_id AND v.tenant_id = s.tenant_id
		WHERE v.tenant_id = $1 AND v.rfx_event_id = $2 AND v.deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.optionCount); err != nil {
		t.Fatalf("count options: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_question_rules r
		INNER JOIN rfx.rfx_versions v ON v.id = r.rfx_version_id AND v.tenant_id = r.tenant_id
		WHERE v.tenant_id = $1 AND v.rfx_event_id = $2 AND v.deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.ruleCount); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_score_models sm
		INNER JOIN rfx.rfx_versions v ON v.id = sm.rfx_version_id AND v.tenant_id = sm.tenant_id
		WHERE v.tenant_id = $1 AND v.rfx_event_id = $2 AND v.deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.scoreModelCount); err != nil {
		t.Fatalf("count score models: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`,
		tenantID).Scan(&snap.auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records WHERE tenant_id = $1 AND aggregate_scope = $2`,
		tenantID, eventID).Scan(&snap.idempotencyCount); err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(updated_at), 'epoch'::timestamptz) FROM rfx.rfx_versions
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.versionUpdatedMax); err != nil {
		t.Fatalf("max version updated_at: %v", err)
	}
	return snap
}

func attachPublishedScoringToEvent(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	_, err := env.scoreModelSvc.PutScoreModel(ctx, fix.BuyerA, eventID, domain.PutScoreModelInput{
		Criteria: []domain.ScoreCriterionInput{
			{
				CriterionCode:     "HSE",
				Name:              "HSE",
				Weight:            100,
				SortOrder:         1,
				NormalizationJSON: json.RawMessage(`{"type":"BOOLEAN_MAP","true_score":100,"false_score":0}`),
			},
		},
		Bindings: []domain.ScoreBindingInput{
			{
				CriterionCode:    "HSE",
				QuestionCode:     "HSE_OK",
				ScoringRuleJSON:  json.RawMessage(`{"type":"BOOLEAN_MAP"}`),
				KnockoutRuleJSON: json.RawMessage(`{"type":"BOOLEAN_EQUALS","value":false}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("put score model: %v", err)
	}
	if _, err := env.scoreModelSvc.PublishScoreModel(ctx, fix.BuyerA, eventID); err != nil {
		t.Fatalf("publish score model: %v", err)
	}
}

func ensureRichPublishedVersion(t *testing.T, env *testEnv, fix buyerFixture, eventID uuid.UUID, key string) *domain.RfxVersion {
	t.Helper()
	ctx := context.Background()
	draft := makeQuestionnaireDraft(t, env, fix, eventID)
	section, err := env.qSvc.CreateSection(ctx, fix.BuyerA, eventID, domain.CreateSectionInput{
		SectionCode: "DETAILS",
		Title:       "Details",
	})
	if err != nil {
		t.Fatalf("create details section: %v", err)
	}
	choiceQ, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, eventID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "COVERAGE",
		QuestionType: domain.QuestionTypeSingleSelect,
		Label:        "Coverage",
		Required:     true,
	})
	if err != nil {
		t.Fatalf("create choice question: %v", err)
	}
	if _, err := env.qSvc.CreateOption(ctx, fix.BuyerA, eventID, choiceQ.ID, domain.CreateQuestionOptionInput{
		OptionCode: "FULL",
		Label:      "Full",
	}); err != nil {
		t.Fatalf("create option full: %v", err)
	}
	if _, err := env.qSvc.CreateOption(ctx, fix.BuyerA, eventID, choiceQ.ID, domain.CreateQuestionOptionInput{
		OptionCode: "PARTIAL",
		Label:      "Partial",
	}); err != nil {
		t.Fatalf("create option partial: %v", err)
	}
	targetCoverage := "COVERAGE"
	if _, err := env.qSvc.CreateRule(ctx, fix.BuyerA, eventID, domain.CreateQuestionRuleInput{
		RuleCode:           "REQ_COVERAGE",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &targetCoverage,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":"1"}`),
	}); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	draft, err = env.qRepo.GetVersionByID(ctx, draft.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	event, err := env.rfxRepo.GetEventByID(ctx, eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	published, err := env.versionSvc.PublishQuestionnaire(ctx, fix.BuyerA, eventID, key, domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        "Rich publish",
	})
	if err != nil {
		t.Fatalf("publish rich version: %v", err)
	}
	return published
}

func assertFullGraphEqual(t *testing.T, env *testEnv, fix buyerFixture, leftVersionID, rightVersionID uuid.UUID, expectScoring bool) {
	t.Helper()
	ctx := context.Background()
	leftGraph, err := env.qRepo.LoadQuestionnaire(ctx, leftVersionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load left graph: %v", err)
	}
	rightGraph, err := env.qRepo.LoadQuestionnaire(ctx, rightVersionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load right graph: %v", err)
	}
	assertQuestionnaireGraphEqual(t, leftGraph, rightGraph)
	if !expectScoring {
		if _, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, rightVersionID); err == nil {
			t.Fatal("expected no draft score model on restored version")
		}
		return
	}
	leftModel, err := env.scoreRepo.GetPublishedModelForVersion(ctx, fix.TenantID, leftVersionID)
	if err != nil {
		leftModel, err = env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, leftVersionID)
	}
	if err != nil {
		t.Fatalf("load left score model: %v", err)
	}
	rightModel, err := env.scoreRepo.GetDraftModelForVersion(ctx, fix.TenantID, rightVersionID)
	if err != nil {
		t.Fatalf("load restored draft score model: %v", err)
	}
	if rightModel.Status != domain.ScoreModelStatusDraft {
		t.Fatalf("restored score model status=%s", rightModel.Status)
	}
	if leftModel.ID == rightModel.ID {
		t.Fatal("restored score model reused source ID")
	}
	leftCriteria, err := env.scoreRepo.ListCriteriaByModel(ctx, leftModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("left criteria: %v", err)
	}
	rightCriteria, err := env.scoreRepo.ListCriteriaByModel(ctx, rightModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("right criteria: %v", err)
	}
	if len(leftCriteria) != len(rightCriteria) {
		t.Fatalf("criteria count mismatch")
	}
	leftCriterionByCode := map[string]domain.ScoreCriterion{}
	for _, c := range leftCriteria {
		leftCriterionByCode[c.CriterionCode] = c
	}
	for _, c := range rightCriteria {
		source, ok := leftCriterionByCode[c.CriterionCode]
		if !ok {
			t.Fatalf("missing criterion code %s", c.CriterionCode)
		}
		if source.ID == c.ID {
			t.Fatalf("criterion ID reused for %s", c.CriterionCode)
		}
		if source.Weight != c.Weight || string(source.NormalizationJSON) != string(c.NormalizationJSON) {
			t.Fatalf("criterion config mismatch for %s", c.CriterionCode)
		}
	}
	leftBindings, err := env.scoreRepo.ListBindingsByModel(ctx, leftModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("left bindings: %v", err)
	}
	rightBindings, err := env.scoreRepo.ListBindingsByModel(ctx, rightModel.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("right bindings: %v", err)
	}
	if len(leftBindings) != len(rightBindings) {
		t.Fatalf("binding count mismatch")
	}
	leftQuestions := map[uuid.UUID]string{}
	for _, section := range leftGraph.Sections {
		for _, q := range section.Questions {
			leftQuestions[q.ID] = q.QuestionCode
		}
	}
	rightQuestions := map[uuid.UUID]string{}
	for _, section := range rightGraph.Sections {
		for _, q := range section.Questions {
			rightQuestions[q.ID] = q.QuestionCode
		}
	}
	for i, leftBinding := range leftBindings {
		rightBinding := rightBindings[i]
		if leftBinding.BindingType != rightBinding.BindingType {
			t.Fatalf("binding type mismatch")
		}
		if string(leftBinding.ScoringRuleJSON) != string(rightBinding.ScoringRuleJSON) {
			t.Fatalf("scoring_rule_json mismatch")
		}
		if string(leftBinding.KnockoutRuleJSON) != string(rightBinding.KnockoutRuleJSON) {
			t.Fatalf("knockout_rule_json mismatch")
		}
		if leftQuestions[leftBinding.QuestionID] != rightQuestions[rightBinding.QuestionID] {
			t.Fatalf("binding question remap mismatch")
		}
		if leftBinding.QuestionID == rightBinding.QuestionID {
			t.Fatal("binding reused source question ID")
		}
	}
}

func buildRepublishPublishInput(
	t *testing.T,
	env *testEnv,
	fix buyerFixture,
	actor domain.ActorContext,
	eventID uuid.UUID,
	draft *domain.RfxVersion,
	summary string,
) domain.PublishQuestionnaireInput {
	t.Helper()
	analysis, err := env.versionSvc.PreviewChangeImpact(context.Background(), actor, eventID, domain.PreviewChangeImpactInput{
		CandidateVersionID: draft.ID,
	})
	if err != nil {
		t.Fatalf("preview for republish: %v", err)
	}
	event, err := env.rfxRepo.GetEventByID(context.Background(), eventID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	analysisID := analysis.ID
	return domain.PublishQuestionnaireInput{
		ExpectedEventVersion: event.Version,
		ExpectedDraftVersion: draft.Version,
		ChangeSummary:        summary,
		ImpactAnalysisID:     &analysisID,
		CanonicalDiffHash:    analysis.CanonicalDiffHash,
	}
}

func republishWithImpactConfirmation(
	t *testing.T,
	env *testEnv,
	fix buyerFixture,
	eventID uuid.UUID,
	key string,
	actor domain.ActorContext,
	draft *domain.RfxVersion,
	summary string,
) *domain.RfxVersion {
	t.Helper()
	in := buildRepublishPublishInput(t, env, fix, actor, eventID, draft, summary)
	published, err := env.versionSvc.PublishQuestionnaire(context.Background(), actor, eventID, key, in)
	if err != nil {
		t.Fatalf("republish with confirmation: %v", err)
	}
	return published
}

func createTemplateInput(code string, owner *uuid.UUID) domain.CreateTemplateInput {
	in := domain.CreateTemplateInput{
		TemplateCode: code,
		NameI18n:     json.RawMessage(`{"en-US":"Template ` + code + `"}`),
		RfxType:      strPtr("RFQ"),
	}
	if owner != nil {
		in.OwnerCompanyID = owner
	}
	return in
}

func strPtr(v string) *string { return &v }

func createTemplate(t *testing.T, env *testEnv, fix buyerFixture, code string, owner *uuid.UUID) *domain.TemplateDetail {
	t.Helper()
	detail, err := env.templateSvc.CreateTemplate(context.Background(), fix.BuyerA, createTemplateInput(code, owner))
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return detail
}

func populateTemplateGraph(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) {
	t.Helper()
	populateTemplateGraphWithRule(t, env, fix, templateID, false)
}

func populateTemplateGraphWithOptions(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	section, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "GENERAL",
		Title:       "General",
	})
	if err != nil {
		t.Fatalf("create template section: %v", err)
	}
	question, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "COVERAGE",
		QuestionType: domain.QuestionTypeSingleSelect,
		Label:        "Coverage",
		Required:     true,
	})
	if err != nil {
		t.Fatalf("create template question: %v", err)
	}
	if _, err := env.templateQSvc.CreateOption(ctx, fix.BuyerA, templateID, question.ID, domain.CreateQuestionOptionInput{
		OptionCode: "FULL",
		Label:      "Full",
	}); err != nil {
		t.Fatalf("create template option: %v", err)
	}
}

func populateRichTemplateGraphForCloneTests(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	generalDesc := "General requirements"
	fleetHelp := "Enter fleet count"
	section, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "GENERAL",
		Title:       "General",
		Description: &generalDesc,
	})
	if err != nil {
		t.Fatalf("create template general section: %v", err)
	}
	if _, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, section.ID, domain.CreateQuestionInput{
		QuestionCode:       "FLEET_SIZE",
		QuestionType:       domain.QuestionTypeText,
		Label:              "Fleet size",
		HelpText:           &fleetHelp,
		Required:           true,
		ValidationRuleJSON: json.RawMessage(`{"min":1}`),
	}); err != nil {
		t.Fatalf("create template fleet question: %v", err)
	}
	detailsSection, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "DETAILS",
		Title:       "Details",
	})
	if err != nil {
		t.Fatalf("create template details section: %v", err)
	}
	coverageQ, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, detailsSection.ID, domain.CreateQuestionInput{
		QuestionCode: "COVERAGE",
		QuestionType: domain.QuestionTypeSingleSelect,
		Label:        "Coverage",
		Required:     true,
	})
	if err != nil {
		t.Fatalf("create template coverage question: %v", err)
	}
	for _, opt := range []struct {
		code, label string
	}{
		{"FULL", "Full"},
		{"PARTIAL", "Partial"},
	} {
		if _, err := env.templateQSvc.CreateOption(ctx, fix.BuyerA, templateID, coverageQ.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.code,
			Label:      opt.label,
		}); err != nil {
			t.Fatalf("create template option %s: %v", opt.code, err)
		}
	}
	target := "COVERAGE"
	if _, err := env.templateQSvc.CreateRule(ctx, fix.BuyerA, templateID, domain.CreateQuestionRuleInput{
		RuleCode:           "REQ_COVERAGE",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &target,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":"10"}`),
	}); err != nil {
		t.Fatalf("create template rule: %v", err)
	}
}

func setupPublishedRichTemplate(t *testing.T, env *testEnv, fix buyerFixture, code string, owner *uuid.UUID) (*domain.TemplateDetail, *domain.RfxTemplateVersion) {
	t.Helper()
	detail := createTemplate(t, env, fix, code, owner)
	populateRichTemplateGraphForCloneTests(t, env, fix, detail.Template.ID)
	published := publishTemplate(t, env, fix, detail.Template.ID, uuid.NewString())
	return detail, published
}

func populateTemplateGraphWithRule(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, withRule bool) {
	t.Helper()
	ctx := context.Background()
	section, err := env.templateQSvc.CreateSection(ctx, fix.BuyerA, templateID, domain.CreateSectionInput{
		SectionCode: "GENERAL",
		Title:       "General",
	})
	if err != nil {
		t.Fatalf("create template section: %v", err)
	}
	fleetQ, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET_SIZE",
		QuestionType: domain.QuestionTypeText,
		Label:        "Fleet size",
		Required:     true,
	})
	if err != nil {
		t.Fatalf("create template question: %v", err)
	}
	if !withRule {
		return
	}
	if _, err := env.templateQSvc.CreateQuestion(ctx, fix.BuyerA, templateID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES",
		QuestionType: domain.QuestionTypeText,
		Label:        "Notes",
		Required:     false,
	}); err != nil {
		t.Fatalf("create template target question: %v", err)
	}
	target := "NOTES"
	_ = fleetQ
	if _, err := env.templateQSvc.CreateRule(ctx, fix.BuyerA, templateID, domain.CreateQuestionRuleInput{
		RuleCode:           "REQ_NOTES",
		Action:             domain.RuleActionRequire,
		TargetQuestionCode: &target,
		ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"FLEET_SIZE","value":"10"}`),
	}); err != nil {
		t.Fatalf("create template rule: %v", err)
	}
}

func publishTemplate(t *testing.T, env *testEnv, fix buyerFixture, templateID uuid.UUID, key string) *domain.RfxTemplateVersion {
	t.Helper()
	detail, err := env.templateSvc.GetTemplate(context.Background(), fix.BuyerA, templateID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if detail.DraftVersion == nil {
		t.Fatal("expected draft version")
	}
	published, err := env.templateSvc.PublishTemplateVersion(context.Background(), fix.BuyerA, templateID, key, domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: detail.Template.Version,
		ExpectedDraftVersion:    detail.DraftVersion.Version,
		ChangeSummary:           "Initial publish",
	})
	if err != nil {
		t.Fatalf("publish template: %v", err)
	}
	return published
}

func expectAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %v", err)
	}
	if appErr.Code != code {
		t.Fatalf("expected code=%s got=%s message=%s", code, appErr.Code, appErr.Message)
	}
}

func assertQuestionnaireGraphEqual(t *testing.T, left, right *domain.QuestionnaireDefinition) {
	t.Helper()
	if len(left.Sections) != len(right.Sections) {
		t.Fatalf("section count mismatch: %d vs %d", len(left.Sections), len(right.Sections))
	}
}
