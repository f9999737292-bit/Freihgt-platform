//go:build integration

package excelexchange

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT216CreateTemplateBuyerManage200HeadersAndParserParity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	before := captureTenantWriteSnapshot(t, env, fix.TenantID)

	rec := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, "")
	assertCreateTemplateSuccess(t, rec)
	assertCreateTemplateParserParity(t, rec.Body.Bytes())

	after := captureTenantWriteSnapshot(t, env, fix.TenantID)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("download mutated tenant state: before=%+v after=%+v", before, after)
	}
}

func TestE7P2INT217CreateTemplateAllowedBuyerManageRoles(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	actors := []struct {
		name  string
		actor domain.ActorContext
	}{
		{name: "PROCUREMENT_MANAGER", actor: fix.BuyerA},
		{name: "PLATFORM_ADMIN", actor: seedActorWithRole(t, env, fix, "PLATFORM_ADMIN", fix.CompanyA, "platform-admin@test.local")},
		{name: "SHIPPER_ADMIN", actor: seedActorWithRole(t, env, fix, "SHIPPER_ADMIN", fix.CompanyA, "shipper-admin@test.local")},
		{name: "FORWARDER_MANAGER", actor: seedActorWithRole(t, env, fix, "FORWARDER_MANAGER", fix.CompanyA, "forwarder-manager@test.local")},
	}
	var previous []byte
	for _, tc := range actors {
		t.Run(tc.name, func(t *testing.T) {
			rec := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), tc.actor, "")
			assertCreateTemplateSuccess(t, rec)
			assertCreateTemplateParserParity(t, rec.Body.Bytes())
			if previous != nil {
				assertSemanticallyEquivalentWorkbooks(t, previous, rec.Body.Bytes())
			}
			previous = rec.Body.Bytes()
		})
	}
}

func TestE7P2INT218CreateTemplateDeniedRolesAndUnauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	carrierAdmin := seedActorWithRole(t, env, fix, "CARRIER_ADMIN", fix.CarrierID, "carrier-admin@test.local")

	cases := []struct {
		name   string
		actor  domain.ActorContext
		status int
		code   apperrors.Code
	}{
		{name: "SHIPPER_LOGIST", actor: fix.BuyerRead, status: http.StatusForbidden, code: apperrors.CodeForbidden},
		{name: "CARRIER_DISPATCHER", actor: fix.CarrierAct, status: http.StatusForbidden, code: apperrors.CodeForbidden},
		{name: "CARRIER_ADMIN", actor: carrierAdmin, status: http.StatusForbidden, code: apperrors.CodeForbidden},
		{name: "unauthenticated", actor: domain.ActorContext{}, status: http.StatusUnauthorized, code: apperrors.CodeUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := captureTenantWriteSnapshot(t, env, fix.TenantID)
			rec := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), tc.actor, "")
			assertHTTPErrorCode(t, rec, tc.status, tc.code)
			if strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") {
				t.Fatal("denied response must not return XLSX content-type")
			}
			if body := rec.Body.Bytes(); len(body) >= 2 && body[0] == 'P' && body[1] == 'K' {
				t.Fatal("denied response must not contain XLSX bytes")
			}
			after := captureTenantWriteSnapshot(t, env, fix.TenantID)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("denied download mutated state: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestE7P2INT219CreateTemplateFlagOffTenantQueryAndIsolation(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	otherManage := seedForeignTenantManageActor(t, env, fix)

	t.Run("flag_off_404", func(t *testing.T) {
		before := captureTenantWriteSnapshot(t, env, fix.TenantID)
		rec := getBuyerXlsxCreateTemplateHTTP(t, env, config.Config{RfxExcelExchangeEnabled: false}, fix.BuyerA, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d want 404 body=%s", rec.Code, rec.Body.String())
		}
		after := captureTenantWriteSnapshot(t, env, fix.TenantID)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("flag-off download must not write state")
		}
	})
	t.Run("tenant_id_query_403", func(t *testing.T) {
		before := captureTenantWriteSnapshot(t, env, fix.TenantID)
		rec := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, "tenant_id="+fix.TenantID.String())
		assertHTTPErrorCode(t, rec, http.StatusForbidden, apperrors.CodeForbidden)
		after := captureTenantWriteSnapshot(t, env, fix.TenantID)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("tenant_id query must not write state")
		}
	})
	t.Run("foreign_company_same_blank_file", func(t *testing.T) {
		first := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, "")
		second := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerB, "")
		assertCreateTemplateSuccess(t, first)
		assertCreateTemplateSuccess(t, second)
		assertSemanticallyEquivalentWorkbooks(t, first.Body.Bytes(), second.Body.Bytes())
		assertWorkbookHasNoTenantIdentity(t, second.Body.Bytes(), fix.TenantID, fix.CompanyA, fix.CompanyB)
	})
	t.Run("foreign_tenant_same_blank_file", func(t *testing.T) {
		home := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, "")
		foreign := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), otherManage, "")
		assertCreateTemplateSuccess(t, home)
		assertCreateTemplateSuccess(t, foreign)
		assertSemanticallyEquivalentWorkbooks(t, home.Body.Bytes(), foreign.Body.Bytes())
		assertWorkbookHasNoTenantIdentity(t, foreign.Body.Bytes(), fix.TenantID, fix.CompanyA, fix.OtherTenantID)
	})
}

func TestE7P2INT220CreateTemplateRepeatedGetNoWriteAndRouteParity(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	before := captureTenantWriteSnapshot(t, env, fix.TenantID)

	first := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, "")
	second := getBuyerXlsxCreateTemplateHTTP(t, env, enabledExcelExchangeConfig(), fix.BuyerA, "")
	assertCreateTemplateSuccess(t, first)
	assertCreateTemplateSuccess(t, second)
	assertSemanticallyEquivalentWorkbooks(t, first.Body.Bytes(), second.Body.Bytes())

	after := captureTenantWriteSnapshot(t, env, fix.TenantID)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("repeated GET mutated state: before=%+v after=%+v", before, after)
	}

	var route sharedrfx.ExcelExchangeRoute
	for _, candidate := range sharedrfx.E7ExcelExchangeRoutes() {
		if candidate.Name == "export_buyer_create_xlsx_template" {
			route = candidate
		}
	}
	if route.GatewayPath != "/api/v1/rfx-events/xlsx-create/template" ||
		route.ServicePath != "/v1/rfx-events/xlsx-create/template" ||
		route.OpenAPIOperationID != "get_buyer_new_rfx_event_xlsx_create_template" ||
		route.RBACPolicy != "PolicyBuyerManage" ||
		!route.FeatureFlagProtected {
		t.Fatalf("unexpected template route: %+v", route)
	}
	if sharedrfx.IsIntegrationProtectedRoute(http.MethodGet, route.GatewayPath) {
		t.Fatal("template route must remain human JWT")
	}
}

func assertCreateTemplateSuccess(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, service.BuyerDraftXLSXContentType) {
		t.Fatalf("content-type=%q", ct)
	}
	wantDisposition := `attachment; filename="` + xlsxexchange.BuyerCreateBlankWorkbookFilename + `"`
	if rec.Header().Get("Content-Disposition") != wantDisposition {
		t.Fatalf("content-disposition=%q want %q", rec.Header().Get("Content-Disposition"), wantDisposition)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control=%q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("x-content-type-options=%q", rec.Header().Get("X-Content-Type-Options"))
	}
	if len(rec.Body.Bytes()) < 4 || rec.Body.Bytes()[0] != 'P' || rec.Body.Bytes()[1] != 'K' {
		t.Fatal("expected XLSX zip signature")
	}
}

func assertCreateTemplateParserParity(t *testing.T, data []byte) {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer f.Close()
	if got := f.GetSheetList(); !reflect.DeepEqual(got, []string{"Instructions", "Metadata", "Lots", "Sections", "Questions", "Options", "Rules"}) {
		t.Fatalf("sheets=%v", got)
	}
	preview, err := xlsxexchange.ParseBuyerCreatePreview(context.Background(), data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.Mode != xlsxexchange.BuyerImportModeCreateNewDraft || !preview.ReadyToCommit {
		t.Fatalf("parser parity failed: %+v errors=%v", preview, preview.Errors)
	}
	if preview.SchemaName != domain.SchemaVersionBuyerXLSXV1 {
		t.Fatalf("schema_name=%q", preview.SchemaName)
	}
}

func assertSemanticallyEquivalentWorkbooks(t *testing.T, first, second []byte) {
	t.Helper()
	canonicalFirst, err := xlsxexchange.CanonicalWorkbookSnapshot(first)
	if err != nil {
		t.Fatalf("canonical first: %v", err)
	}
	canonicalSecond, err := xlsxexchange.CanonicalWorkbookSnapshot(second)
	if err != nil {
		t.Fatalf("canonical second: %v", err)
	}
	if !reflect.DeepEqual(canonicalFirst, canonicalSecond) {
		t.Fatal("workbooks are not schema-equivalent")
	}
}

func assertWorkbookHasNoTenantIdentity(t *testing.T, data []byte, ids ...uuid.UUID) {
	t.Helper()
	raw := strings.ToLower(string(data))
	for _, id := range ids {
		if strings.Contains(raw, strings.ToLower(id.String())) {
			t.Fatalf("workbook leaked identity %s", id)
		}
	}
}

func seedActorWithRole(t *testing.T, env *testEnv, fix buyerFixture, roleCode string, companyID uuid.UUID, email string) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	actor := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	var roleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = $1 LIMIT 1`, roleCode).Scan(&roleID); err != nil {
		t.Fatalf("lookup role %s: %v", roleCode, err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`, actor.UserID, fix.TenantID, email, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`, fix.TenantID, companyID, actor.UserID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`, fix.TenantID, actor.UserID, companyID, roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	return actor
}

func seedForeignTenantManageActor(t *testing.T, env *testEnv, fix buyerFixture) domain.ActorContext {
	t.Helper()
	ctx := context.Background()
	companyID := uuid.New()
	actor := domain.ActorContext{TenantID: fix.OtherTenantID, UserID: uuid.New()}
	var roleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'PROCUREMENT_MANAGER' LIMIT 1`).Scan(&roleID); err != nil {
		t.Fatalf("lookup role: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`, companyID, fix.OtherTenantID, "Other Buyer", "SHIPPER"); err != nil {
		t.Fatalf("seed foreign company: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1,$2,$3,$4)`, actor.UserID, fix.OtherTenantID, "other-manage@test.local", "other-manage@test.local"); err != nil {
		t.Fatalf("seed foreign user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1,$2,$3)`, fix.OtherTenantID, companyID, actor.UserID); err != nil {
		t.Fatalf("seed foreign membership: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1,$2,$3,$4)`, fix.OtherTenantID, actor.UserID, companyID, roleID); err != nil {
		t.Fatalf("seed foreign role: %v", err)
	}
	return actor
}

type tenantWriteSnapshot struct {
	events           int
	analyses         int
	lots             int
	versions         int
	sections         int
	questions        int
	options          int
	rules            int
	participants     int
	responses        int
	awards           int
	awardTOs         int
	audits           int
	idempotency      int
	erpCredentials   int
}

func captureTenantWriteSnapshot(t *testing.T, env *testEnv, tenantID uuid.UUID) tenantWriteSnapshot {
	t.Helper()
	ctx := context.Background()
	snap := tenantWriteSnapshot{}
	queries := []struct {
		dest *int
		sql  string
	}{
		{&snap.events, `SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id = $1`},
		{&snap.analyses, `SELECT COUNT(*) FROM rfx.rfx_import_analyses WHERE tenant_id = $1`},
		{&snap.lots, `SELECT COUNT(*) FROM rfx.rfx_lots WHERE tenant_id = $1`},
		{&snap.versions, `SELECT COUNT(*) FROM rfx.rfx_versions WHERE tenant_id = $1`},
		{&snap.sections, `SELECT COUNT(*) FROM rfx.rfx_sections WHERE tenant_id = $1`},
		{&snap.questions, `SELECT COUNT(*) FROM rfx.rfx_questions WHERE tenant_id = $1`},
		{&snap.options, `SELECT COUNT(*) FROM rfx.rfx_question_options WHERE tenant_id = $1`},
		{&snap.rules, `SELECT COUNT(*) FROM rfx.rfx_question_rules WHERE tenant_id = $1`},
		{&snap.participants, `SELECT COUNT(*) FROM rfx.rfx_participants WHERE tenant_id = $1`},
		{&snap.responses, `SELECT COUNT(*) FROM rfx.rfx_responses WHERE tenant_id = $1`},
		{&snap.awards, `SELECT COUNT(*) FROM rfx.rfx_awards WHERE tenant_id = $1`},
		{&snap.awardTOs, `SELECT COUNT(*) FROM rfx.rfx_award_transport_orders WHERE tenant_id = $1`},
		{&snap.audits, `SELECT COUNT(*) FROM rfx.audit_events WHERE tenant_id = $1`},
		{&snap.idempotency, `SELECT COUNT(*) FROM rfx.rfx_idempotency_records WHERE tenant_id = $1`},
		{&snap.erpCredentials, `SELECT COUNT(*) FROM rfx.rfx_integration_credentials WHERE tenant_id = $1`},
	}
	for _, q := range queries {
		if err := env.pool.QueryRow(ctx, q.sql, tenantID).Scan(q.dest); err != nil {
			t.Fatalf("snapshot query %s: %v", q.sql, err)
		}
	}
	return snap
}
