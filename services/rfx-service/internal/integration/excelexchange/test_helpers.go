//go:build integration

package excelexchange

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

type testEnv struct {
	pool               *pgxpool.Pool
	rfxRepo            *repository.RfxRepository
	auditRepo          *repository.AuditRepository
	idemRepo           *repository.IdempotencyRepository
	membershipRepo     *repository.MembershipRepository
	qRepo              *repository.QuestionnaireRepository
	importAnalysisRepo *repository.ImportAnalysisRepository
	rfxSvc             *service.RfxService
	qSvc               *service.QuestionnaireService
	excelExchangeSvc   *service.ExcelExchangeService
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

type richDraftFixture struct {
	Event    *domain.RfxEvent
	Version  *domain.RfxVersion
	Section  *domain.Section
	Question *domain.Question
	Option   *domain.QuestionOption
	Rule     *domain.QuestionRule
	Lot      *domain.RfxLot
}

type graphWriteSnapshot struct {
	eventVersion      int
	versionCount      int
	sectionCount      int
	questionCount     int
	optionCount       int
	ruleCount         int
	lotCount          int
	auditCount        int
	idempotencyCount  int
	importAnalysisCnt int
}

type competitorSentinels struct {
	ParticipantAID  uuid.UUID
	ParticipantBID  uuid.UUID
	ResponseAID     uuid.UUID
	ResponseBID     uuid.UUID
	CarrierACompany uuid.UUID
	CarrierBCompany uuid.UUID
	OfferRateA      string
	OfferRateB      string
	NameTokenA      string
	NameTokenB      string
	EmailTokenA     string
	EmailTokenB     string
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
	idemRepo := repository.NewIdempotencyRepository(pool)
	importAnalysisRepo := repository.NewImportAnalysisRepository(pool)
	rfxSvc := service.NewRfxServiceWithAtomic(pool, rfxRepo, auditRepo, membershipRepo, newAwardConversionStub(pool))
	qSvc := service.NewQuestionnaireService(rfxRepo, qRepo, auditRepo, membershipRepo)
	excelExchangeSvc := service.NewExcelExchangeService(rfxRepo, qRepo, rfxSvc)
	return &testEnv{
		pool: pool, rfxRepo: rfxRepo, auditRepo: auditRepo, membershipRepo: membershipRepo,
		qRepo: qRepo, idemRepo: idemRepo, importAnalysisRepo: importAnalysisRepo,
		rfxSvc: rfxSvc, qSvc: qSvc, excelExchangeSvc: excelExchangeSvc,
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

func seedRichDraftEvent(t *testing.T, env *testEnv, fix buyerFixture) richDraftFixture {
	t.Helper()
	ctx := context.Background()
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Excel Export Event",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-XLSX-" + uuid.NewString()[:8],
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	version, err := env.qRepo.GetOrCreateDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("draft version: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_versions SET questionnaire_enabled = TRUE WHERE id = $1`, version.ID); err != nil {
		t.Fatalf("enable questionnaire: %v", err)
	}
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("section: %v", err)
	}
	required := true
	help := "Help text"
	q, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeSingleSelect, Label: "Notes", HelpText: &help, Required: required,
	})
	if err != nil {
		t.Fatalf("question: %v", err)
	}
	opt, err := env.qSvc.CreateOption(ctx, fix.BuyerA, event.ID, q.ID, domain.CreateQuestionOptionInput{
		OptionCode: "YES", Label: "Yes",
	})
	if err != nil {
		t.Fatalf("option: %v", err)
	}
	detailHelp := "Detail help"
	_, err = env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "DETAIL", QuestionType: domain.QuestionTypeText, Label: "Detail notes", HelpText: &detailHelp,
	})
	if err != nil {
		t.Fatalf("detail question: %v", err)
	}
	targetCode := "DETAIL"
	cond := json.RawMessage(`{"operator":"EQUALS","source_question_code":"NOTES","value":"YES"}`)
	rule, err := env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode: "SHOW_DETAIL", Action: domain.RuleActionShow, TargetQuestionCode: &targetCode, ConditionJSON: cond,
	})
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	desc := "Lot description"
	category := "FREIGHT"
	value := 1000.0
	currency := "RUB"
	lot, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Lane bundle",
		Description: &desc, Category: &category, EstimatedValue: &value, CurrencyCode: &currency,
	})
	if err != nil {
		t.Fatalf("lot: %v", err)
	}
	reloaded, err := env.rfxRepo.GetEventByID(ctx, event.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload event: %v", err)
	}
	version, err = env.qRepo.GetActiveDraftVersion(ctx, fix.TenantID, event.ID)
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	return richDraftFixture{
		Event: reloaded, Version: version, Section: sec, Question: q, Option: opt, Rule: rule, Lot: lot,
	}
}

func seedCompetitorSentinels(t *testing.T, env *testEnv, fix buyerFixture, draft richDraftFixture) competitorSentinels {
	t.Helper()
	ctx := context.Background()
	sent := competitorSentinels{
		ParticipantAID:  uuid.New(),
		ParticipantBID:  uuid.New(),
		ResponseAID:     uuid.New(),
		ResponseBID:     uuid.New(),
		CarrierACompany: fix.CarrierID,
		CarrierBCompany: fix.CarrierBID,
		OfferRateA:      "888888.01",
		OfferRateB:      "777777.02",
		NameTokenA:      "SENTINEL-XLSX-CARRIER-A",
		NameTokenB:      "SENTINEL-XLSX-CARRIER-B",
		EmailTokenA:     "sentinel-xlsx-carrier-a@test.local",
		EmailTokenB:     "sentinel-xlsx-carrier-b@test.local",
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, draft.Event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: draft.Event.ID, CompanyID: fix.CarrierBID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add carrier B participant: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE core.companies SET legal_name = $3 WHERE tenant_id = $1 AND id = $2`,
		fix.TenantID, fix.CarrierID, sent.NameTokenA); err != nil {
		t.Fatalf("tag carrier A company: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE core.companies SET legal_name = $3 WHERE tenant_id = $1 AND id = $2`,
		fix.TenantID, fix.CarrierBID, sent.NameTokenB); err != nil {
		t.Fatalf("tag carrier B company: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE core.users SET email = $3, full_name = $3
		WHERE tenant_id = $1 AND id = $2`,
		fix.TenantID, fix.CarrierAct.UserID, sent.EmailTokenA); err != nil {
		t.Fatalf("tag carrier A email: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE core.users SET email = $3, full_name = $3
		WHERE tenant_id = $1 AND id = $2`,
		fix.TenantID, fix.CarrierBAct.UserID, sent.EmailTokenB); err != nil {
		t.Fatalf("tag carrier B email: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_participants SET id = $4
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND company_id = $3`,
		fix.TenantID, draft.Event.ID, fix.CarrierID, sent.ParticipantAID); err != nil {
		t.Fatalf("set participant A id: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		UPDATE rfx.rfx_participants SET id = $4
		WHERE tenant_id = $1 AND rfx_event_id = $2 AND company_id = $3`,
		fix.TenantID, draft.Event.ID, fix.CarrierBID, sent.ParticipantBID); err != nil {
		t.Fatalf("set participant B id: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_responses (id, tenant_id, rfx_event_id, participant_company_id, status, commercial_score, total_score)
		VALUES ($1,$2,$3,$4,'SUBMITTED',80,80)`,
		sent.ResponseAID, fix.TenantID, draft.Event.ID, fix.CarrierID); err != nil {
		t.Fatalf("insert response A: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_responses (id, tenant_id, rfx_event_id, participant_company_id, status, commercial_score, total_score)
		VALUES ($1,$2,$3,$4,'SUBMITTED',75,75)`,
		sent.ResponseBID, fix.TenantID, draft.Event.ID, fix.CarrierBID); err != nil {
		t.Fatalf("insert response B: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_response_offer_lines (tenant_id, rfx_response_id, rfx_lot_id, amount, currency_code, comment)
		VALUES ($1,$2,$3,$4,'RUB',$5)`,
		fix.TenantID, sent.ResponseAID, draft.Lot.ID, sent.OfferRateA, "SENTINEL-OFFER-A"); err != nil {
		t.Fatalf("insert offer A: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_response_offer_lines (tenant_id, rfx_response_id, rfx_lot_id, amount, currency_code, comment)
		VALUES ($1,$2,$3,$4,'RUB',$5)`,
		fix.TenantID, sent.ResponseBID, draft.Lot.ID, sent.OfferRateB, "SENTINEL-OFFER-B"); err != nil {
		t.Fatalf("insert offer B: %v", err)
	}
	return sent
}

func assertWorkbookExcludesCompetitorSentinels(t *testing.T, data []byte, sent competitorSentinels, draft richDraftFixture) {
	t.Helper()
	needles := []string{
		sent.ParticipantAID.String(),
		sent.ParticipantBID.String(),
		sent.ResponseAID.String(),
		sent.ResponseBID.String(),
		sent.CarrierACompany.String(),
		sent.CarrierBCompany.String(),
		sent.NameTokenA,
		sent.NameTokenB,
		sent.EmailTokenA,
		sent.EmailTokenB,
		sent.OfferRateA,
		sent.OfferRateB,
		"SENTINEL-OFFER-A",
		"SENTINEL-OFFER-B",
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			t.Fatalf("read sheet %s: %v", sheet, err)
		}
		for rowIdx, row := range rows {
			for colIdx, value := range row {
				for _, needle := range needles {
					if strings.Contains(value, needle) {
						cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
						t.Fatalf("competitor sentinel leaked in cell %s!%s (marker=%q)", sheet, cell, needle)
					}
				}
			}
		}
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", file.Name, err)
		}
		body, err := io.ReadAll(io.LimitReader(rc, 4<<20))
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", file.Name, err)
		}
		content := strings.ToLower(string(body))
		for _, needle := range needles {
			if strings.Contains(content, strings.ToLower(needle)) {
				t.Fatalf("competitor sentinel leaked in zip entry %s (marker=%q)", file.Name, needle)
			}
		}
		if strings.Contains(file.Name, "participant") || strings.Contains(file.Name, "response") || strings.Contains(file.Name, "carrier") {
			t.Fatalf("unexpected competitor-oriented zip entry %s", file.Name)
		}
	}

	foundBuyerMarkers := 0
	for _, marker := range []string{
		draft.Section.SectionCode,
		draft.Question.QuestionCode,
		draft.Option.OptionCode,
		draft.Lot.LotNumber,
	} {
		if workbookContainsValue(t, f, marker) {
			foundBuyerMarkers++
		}
	}
	if foundBuyerMarkers < 3 {
		t.Fatalf("buyer draft markers underrepresented in workbook: found %d/4", foundBuyerMarkers)
	}
}

func workbookContainsValue(t *testing.T, f *excelize.File, want string) bool {
	t.Helper()
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			t.Fatalf("read sheet %s: %v", sheet, err)
		}
		for _, row := range rows {
			for _, value := range row {
				if value == want {
					return true
				}
			}
		}
	}
	return false
}

func assertHTTPErrorCode(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode apperrors.Code) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", wantStatus, rec.Code, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v raw=%s", err, rec.Body.String())
	}
	if payload.Error.Code != string(wantCode) {
		t.Fatalf("expected error code=%s got=%s body=%s", wantCode, payload.Error.Code, rec.Body.String())
	}
}

func enabledExcelExchangeConfig() config.Config {
	return config.Config{RfxExcelExchangeEnabled: true}
}

func getBuyerXlsxExportHTTP(t *testing.T, env *testEnv, cfg config.Config, actor domain.ActorContext, eventID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpserver.NewRouter(log, env.pool, cfg, env.rfxSvc, env.qSvc, nil, nil, nil, nil, nil, nil, env.excelExchangeSvc, nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/rfx-events/"+eventID.String()+"/xlsx-export", nil)
	if actor.TenantID != uuid.Nil {
		req.Header.Set("X-Tenant-ID", actor.TenantID.String())
	}
	if actor.UserID != uuid.Nil {
		req.Header.Set("X-User-ID", actor.UserID.String())
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func captureGraphWriteSnapshot(t *testing.T, env *testEnv, tenantID, eventID uuid.UUID) graphWriteSnapshot {
	t.Helper()
	ctx := context.Background()
	snap := graphWriteSnapshot{}
	if err := env.pool.QueryRow(ctx, `SELECT version FROM rfx.rfx_events WHERE id = $1 AND tenant_id = $2`, eventID, tenantID).Scan(&snap.eventVersion); err != nil {
		t.Fatalf("event version: %v", err)
	}
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
		SELECT COUNT(*) FROM rfx.rfx_lots WHERE tenant_id = $1 AND rfx_event_id = $2 AND deleted_at IS NULL`,
		tenantID, eventID).Scan(&snap.lotCount); err != nil {
		t.Fatalf("count lots: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`, tenantID).Scan(&snap.auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_idempotency_records WHERE tenant_id = $1 AND aggregate_scope = $2`,
		tenantID, eventID).Scan(&snap.idempotencyCount); err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_import_analyses WHERE tenant_id = $1`, tenantID).Scan(&snap.importAnalysisCnt); err != nil {
		t.Fatalf("count import analyses: %v", err)
	}
	return snap
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

func createTempDatabase(ctx context.Context, adminURL string) (dbName string, testURL string, cleanup func(context.Context), err error) {
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return "", "", nil, fmt.Errorf("parse database url: %w", err)
	}
	dbName = "rfx_excel_exchange_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
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

func connectPool(ctx context.Context, testURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, testURL)
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
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			return fmt.Errorf("migration %s: %w", filepath.Base(file), execErr)
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
