//go:build integration

package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

const (
	e7ProcurementPort = "3040"
	e7AdminPort       = "3041"
	e7FlagOffPort     = "3042"
)

type browserE7Fixture struct {
	TenantID            uuid.UUID
	BuyerCompanyID      uuid.UUID
	OtherBuyerCompanyID uuid.UUID
	CarrierCompanyID    uuid.UUID
	CompetitorCompanyID uuid.UUID
	BuyerUserID         uuid.UUID
	OtherBuyerUserID    uuid.UUID
	CarrierUserID       uuid.UUID
	LogistUserID        uuid.UUID
	BuyerJWT            string
	OtherBuyerJWT       string
	CarrierJWT          string
	LogistJWT           string
	IsolationEventID    uuid.UUID
	IsolationRfxNumber  string
	BuyerXlsxEventID    uuid.UUID
	BuyerXlsxRfxNumber  string
	CarrierXlsxEventID  uuid.UUID
	CarrierXlsxResponse uuid.UUID
	CarrierXlsxRfxNumber string
	CarrierXlsxLotID    uuid.UUID
	CompetitorOffer     string
	CompetitorName      string
	CompetitorAnswer    string
	TemplateID          uuid.UUID
	TemplateCode        string
	Late                browserLateSubmissionFixture
	FlagOffXlsxEventID  uuid.UUID
	FlagOffLateEventID  uuid.UUID
}

type browserE7LiveStack struct {
	procurementURL string
	adminURL       string
	flagOffURL     string
	gatewayURL     string
	flagOffGateway string
	fixture        browserE7Fixture
	rfxSrv         *http.Server
	flagOffRfx     *http.Server
	gatewayProc    *browserGatewayProcess
	flagOffGw      *browserGatewayProcess
	adminCmd       *webAdminCmd
	procurementCmd *webProcurementCmd
	flagOffCmd     *webProcurementCmd
	env            *testEnv
}

func TestRfxE7_BrowserE2E_LiveAcceptance(t *testing.T) {
	requireE7BrowserGate(t)
	t.Cleanup(func() { verifyBrowserHarnessCleanup(t) })
	stack := startE7LiveStack(t)
	t.Cleanup(func() { stack.shutdown(t) })
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyE7GatewayProbe(t, stack)
	verifyE7GatewayTemplateDetail(t, stack)
	if err := runE7PlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright E7 browser acceptance suite: %v", err)
	}
	assertE7FlagOffSQLClean(t, stack)
}

func requireE7BrowserGate(t *testing.T) {
	t.Helper()
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Fatal("BROWSER_E2E=1 is required for the E7 browser acceptance gate")
	}
	if os.Getenv("BROWSER_E2E_REQUIRE") != "1" {
		t.Fatal("BROWSER_E2E_REQUIRE=1 is required for the E7 browser acceptance gate")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required for the E7 browser acceptance gate")
	}
}

func startE7LiveStack(t *testing.T) *browserE7LiveStack {
	t.Helper()
	env := setupTestEnv(t)
	lateRepo := repository.NewLateSubmissionRepository(env.pool)
	lateSvc := service.NewLateSubmissionService(env.pool, lateRepo, env.rfxRepo, env.idemRepo, env.auditRepo, env.rfxSvc)
	crSvc := service.NewCarrierResponseServiceWithLateSubmission(
		env.pool, env.rfxRepo, env.answerRepo, env.qRepo, env.auditRepo, env.membershipRepo, env.rfxSvc, env.scoringSvc, lateSvc, env.idemRepo,
	)
	env.crSvc = crSvc

	tmplRepo := repository.NewTemplateLibraryRepository(env.pool)
	tmplQRepo := repository.NewTemplateQuestionnaireRepository(env.pool)
	templateSvc := service.NewTemplateLibraryService(env.pool, tmplRepo, tmplQRepo, env.idemRepo, env.auditRepo, env.rfxSvc)
	templateQSvc := service.NewTemplateQuestionnaireService(env.pool, tmplRepo, tmplQRepo, env.auditRepo, templateSvc)
	templateCloneSvc := service.NewTemplateCloneService(
		env.pool, env.rfxRepo, env.qRepo, tmplRepo, tmplQRepo, env.idemRepo, env.auditRepo, env.rfxSvc, templateSvc,
	)
	importRepo := repository.NewImportAnalysisRepository(env.pool)
	txRunner := repository.NewTransactionRunner(env.pool)
	excelSvc := service.NewExcelExchangeService(
		env.rfxRepo, env.qRepo, env.rfxSvc, importRepo, env.idemRepo, env.auditRepo, txRunner, env.answerRepo,
	)

	fix := seedE7BrowserFixture(t, env, lateSvc, templateSvc, templateQSvc)

	rfxURL, rfxSrv := startE7RfxService(t, env, lateSvc, crSvc, excelSvc, templateSvc, templateQSvc, templateCloneSvc, true)
	flagOffURL, flagOffSrv := startE7RfxService(t, env, lateSvc, crSvc, excelSvc, templateSvc, templateQSvc, templateCloneSvc, false)

	identity := startBrowserIdentityStubWithMemberships(t, map[string][]string{
		fix.BuyerUserID.String():      {"PROCUREMENT_MANAGER"},
		fix.OtherBuyerUserID.String(): {"PROCUREMENT_MANAGER"},
		fix.CarrierUserID.String():    {"CARRIER_DISPATCHER"},
		fix.LogistUserID.String():     {"SHIPPER_LOGIST"},
	}, e7Memberships(fix))
	companyStub := startE7CompanyStub(t, fix)

	procOrigin := "http://127.0.0.1:" + e7ProcurementPort
	adminOrigin := "http://127.0.0.1:" + e7AdminPort
	flagOffOrigin := "http://127.0.0.1:" + e7FlagOffPort
	corsOn := strings.Join([]string{
		procOrigin, adminOrigin, flagOffOrigin,
		"http://localhost:" + e7ProcurementPort,
		"http://localhost:" + e7AdminPort,
		"http://localhost:" + e7FlagOffPort,
	}, ",")

	gatewayURL, gatewayProc := startE7ProductionGateway(t, rfxURL, corsOn, identity, companyStub.URL(), true)
	flagOffGateway, flagOffGw := startE7ProductionGateway(t, flagOffURL, corsOn, identity, companyStub.URL(), false)

	ensureDevPortFree(t, e7AdminPort)
	ensureDevPortFree(t, e7ProcurementPort)
	ensureDevPortFree(t, e7FlagOffPort)

	adminURL, adminCmd := startE7WebAdmin(t, gatewayURL, fix, e7AdminPort)
	waitForHTTP200(t, adminURL+"/login", 120*time.Second)
	procURL, procCmd := startE7WebProcurement(t, gatewayURL, fix, e7ProcurementPort, true)
	waitForHTTP200(t, procURL+"/login", 120*time.Second)
	flagOffWeb, flagOffCmd := startE7WebProcurement(t, flagOffGateway, fix, e7FlagOffPort, false)
	waitForHTTP200(t, flagOffWeb+"/login", 120*time.Second)

	return &browserE7LiveStack{
		procurementURL: procURL,
		adminURL:       adminURL,
		flagOffURL:     flagOffWeb,
		gatewayURL:     gatewayURL,
		flagOffGateway: flagOffGateway,
		fixture:        fix,
		rfxSrv:         rfxSrv,
		flagOffRfx:     flagOffSrv,
		gatewayProc:    gatewayProc,
		flagOffGw:      flagOffGw,
		adminCmd:       adminCmd,
		procurementCmd: procCmd,
		flagOffCmd:     flagOffCmd,
		env:            env,
	}
}

func startE7RfxService(
	t *testing.T,
	env *testEnv,
	lateSvc *service.LateSubmissionService,
	crSvc *service.CarrierResponseService,
	excelSvc *service.ExcelExchangeService,
	templateSvc *service.TemplateLibraryService,
	templateQSvc *service.TemplateQuestionnaireService,
	templateCloneSvc *service.TemplateCloneService,
	flagsOn bool,
) (string, *http.Server) {
	t.Helper()
	cfg := config.Config{
		Environment:              "test",
		RfxVersioningV3Enabled:   true,
		RfxLateSubmissionEnabled: flagsOn,
		RfxExcelExchangeEnabled:  flagsOn,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpserver.NewRouter(
		log, env.pool, cfg,
		env.rfxSvc, env.qSvc, env.versionSvc,
		templateSvc, templateQSvc, templateCloneSvc,
		crSvc, lateSvc, excelSvc, nil,
		env.scoreModelSvc, env.scoringSvc,
		nil, nil, nil,
	)
	return listenHTTPServer(t, handler)
}

func startE7ProductionGateway(
	t *testing.T,
	rfxServiceURL, origins string,
	identity *browserIdentityStub,
	companyURL string,
	flagsOn bool,
) (string, *browserGatewayProcess) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	openAPIDir := filepath.Join(root, "packages", "openapi")
	env := []string{
		"AUTH_ENABLED=true",
		"JWT_SECRET=" + browserE2EJWTSecret,
		"RFX_SERVICE_URL=" + strings.TrimRight(rfxServiceURL, "/"),
		"IDENTITY_SERVICE_URL=" + identity.URL(),
		"COMPANY_SERVICE_URL=" + strings.TrimRight(companyURL, "/"),
		"CORS_ALLOWED_ORIGINS=" + origins,
		"RATE_LIMIT_ENABLED=false",
		"OPENAPI_DIR=" + openAPIDir,
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
		"RFX_VERSIONING_V3_ENABLED=true",
	}
	if flagsOn {
		env = append(env, "RFX_EXCEL_EXCHANGE_ENABLED=true", "RFX_LATE_SUBMISSION_ENABLED=true")
	} else {
		env = append(env, "RFX_EXCEL_EXCHANGE_ENABLED=false", "RFX_LATE_SUBMISSION_ENABLED=false")
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	verifyBrowserGatewayRfxRoute(t, gatewayURL, rfxServiceURL)
	verifyE7GatewayTemplatesRoute(t, gatewayURL, rfxServiceURL)
	return gatewayURL, proc
}

func verifyE7GatewayTemplatesRoute(t *testing.T, gatewayURL, rfxServiceURL string) {
	t.Helper()
	routes := readGatewayRoutes(t, gatewayURL)
	wantTarget := strings.TrimRight(rfxServiceURL, "/") + "/v1/rfx-templates"
	for _, route := range routes {
		if route["prefix"] == "/api/v1/rfx-templates" && route["target"] == wantTarget {
			return
		}
	}
	t.Fatalf("gateway /routes missing rfx-templates target=%q routes=%v", wantTarget, routes)
}

func startE7WebAdmin(t *testing.T, gatewayURL string, fix browserE7Fixture, port string) (string, *webAdminCmd) {
	t.Helper()
	env := append(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_PUBLIC_RFX_VERSIONING_V3_ENABLED=true",
		"NUXT_E2E_DISABLE_SSR=true",
		"NUXT_E2E_DISABLE_DEVTOOLS=true",
	)
	cmd, cancel, logFile, launch := bootNuxtDevApp(t, "web-admin", port, env)
	proc := &webAdminCmd{
		cmd:    cmd,
		cancel: cancel,
		port:   port,
		logs:   []*os.File{logFile},
		launch: launch,
	}
	t.Cleanup(func() { stopBrowserWebAdmin(t, proc) })
	return "http://127.0.0.1:" + port, proc
}

func startE7WebProcurement(t *testing.T, gatewayURL string, fix browserE7Fixture, port string, flagsOn bool) (string, *webProcurementCmd) {
	t.Helper()
	env := overrideProcessEnv(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_PUBLIC_RFX_VERSIONING_V3_ENABLED=true",
		"NUXT_E2E_DISABLE_SSR=true",
		"NUXT_E2E_DISABLE_DEVTOOLS=true",
	)
	if flagsOn {
		env = overrideProcessEnv(env,
			"NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED=true",
			"RFX_EXCEL_EXCHANGE_UI=1",
			"NUXT_PUBLIC_RFX_LATE_SUBMISSION_ENABLED=true",
			"RFX_LATE_SUBMISSION_UI=1",
		)
	} else {
		env = overrideProcessEnv(env,
			"NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED=false",
			"RFX_EXCEL_EXCHANGE_UI=0",
			"NUXT_PUBLIC_RFX_LATE_SUBMISSION_ENABLED=false",
			"RFX_LATE_SUBMISSION_UI=0",
		)
	}
	cmd, cancel, logFile, launch := bootNuxtDevApp(t, "web-procurement", port, env)
	proc := &webProcurementCmd{
		cmd:    cmd,
		cancel: cancel,
		port:   port,
		logs:   []*os.File{logFile},
		launch: launch,
	}
	t.Cleanup(func() { stopBrowserWebProcurement(t, proc) })
	return "http://127.0.0.1:" + port, proc
}

func (s *browserE7LiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.flagOffCmd)
	stopBrowserWebProcurement(t, s.procurementCmd)
	stopBrowserWebAdmin(t, s.adminCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.flagOffRfx != nil {
		_ = s.flagOffRfx.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
	if s.flagOffGw != nil {
		shutdownBrowserGatewayProcess(s.flagOffGw)
	}
	verifyDevPortsReleased(t, e7ProcurementPort, e7AdminPort, e7FlagOffPort)
}

func verifyE7GatewayTemplateDetail(t *testing.T, stack *browserE7LiveStack) {
	t.Helper()
	detailURL := strings.TrimRight(stack.gatewayURL, "/") + "/api/v1/rfx-templates/" + stack.fixture.TemplateID.String()
	req, err := http.NewRequest(http.MethodGet, detailURL, nil)
	if err != nil {
		t.Fatalf("template detail request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.BuyerJWT)
	req.Header.Set("X-Company-ID", stack.fixture.BuyerCompanyID.String())
	req.Header.Set("Origin", stack.adminURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", detailURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s -> %d body=%s", detailURL, resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), `"versions"`) {
		t.Fatalf("GET %s missing versions payload: %s", detailURL, string(body))
	}
	t.Logf("E7-TEMPLATE-DETAIL-PROBE GET %s -> %d", detailURL, resp.StatusCode)
}

func verifyE7GatewayProbe(t *testing.T, stack *browserE7LiveStack) {
	t.Helper()
	eventURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.IsolationEventID.String()
	req, err := http.NewRequest(http.MethodGet, eventURL, nil)
	if err != nil {
		t.Fatalf("probe event request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.BuyerJWT)
	req.Header.Set("X-Company-ID", stack.fixture.BuyerCompanyID.String())
	req.Header.Set("Origin", stack.procurementURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe human GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe human GET: status=%d url=%s", resp.StatusCode, eventURL)
	}
}

func runE7PlaywrightSuite(t *testing.T, stack *browserE7LiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "e7-browser-acceptance")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"CI=true",
		"BROWSER_E2E=1",
		"BROWSER_E2E_REQUIRE=1",
		"BROWSER_E2E_PROCUREMENT_URL="+stack.procurementURL,
		"BROWSER_E2E_ADMIN_URL="+stack.adminURL,
		"BROWSER_E2E_FLAG_OFF_WEB_URL="+stack.flagOffURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_FLAG_OFF_GATEWAY_URL="+stack.flagOffGateway,
		"BROWSER_E2E_JWT="+fix.BuyerJWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.BuyerCompanyID.String(),
		"BROWSER_E2E_USER_ID="+fix.BuyerUserID.String(),
		"BROWSER_E2E_CARRIER_JWT="+fix.CarrierJWT,
		"BROWSER_E2E_CARRIER_USER_ID="+fix.CarrierUserID.String(),
		"BROWSER_E2E_CARRIER_COMPANY_ID="+fix.CarrierCompanyID.String(),
		"BROWSER_E2E_OTHER_BUYER_JWT="+fix.OtherBuyerJWT,
		"BROWSER_E2E_OTHER_BUYER_USER_ID="+fix.OtherBuyerUserID.String(),
		"BROWSER_E2E_OTHER_BUYER_COMPANY_ID="+fix.OtherBuyerCompanyID.String(),
		"BROWSER_E2E_LOGIST_JWT="+fix.LogistJWT,
		"BROWSER_E2E_LOGIST_USER_ID="+fix.LogistUserID.String(),
		"BROWSER_E2E_ISOLATION_EVENT_ID="+fix.IsolationEventID.String(),
		"BROWSER_E2E_ISOLATION_RFX_NUMBER="+fix.IsolationRfxNumber,
		"BROWSER_E2E_BUYER_XLSX_EVENT_ID="+fix.BuyerXlsxEventID.String(),
		"BROWSER_E2E_BUYER_XLSX_RFX_NUMBER="+fix.BuyerXlsxRfxNumber,
		"BROWSER_E2E_CARRIER_XLSX_EVENT_ID="+fix.CarrierXlsxEventID.String(),
		"BROWSER_E2E_CARRIER_XLSX_RESPONSE_ID="+fix.CarrierXlsxResponse.String(),
		"BROWSER_E2E_CARRIER_XLSX_RFX_NUMBER="+fix.CarrierXlsxRfxNumber,
		"BROWSER_E2E_CARRIER_XLSX_LOT_ID="+fix.CarrierXlsxLotID.String(),
		"BROWSER_E2E_COMPETITOR_OFFER="+fix.CompetitorOffer,
		"BROWSER_E2E_COMPETITOR_NAME="+fix.CompetitorName,
		"BROWSER_E2E_COMPETITOR_ANSWER="+fix.CompetitorAnswer,
		"BROWSER_E2E_TEMPLATE_ID="+fix.TemplateID.String(),
		"BROWSER_E2E_TEMPLATE_CODE="+fix.TemplateCode,
		"BROWSER_E2E_LATE_CREATE_EVENT_ID="+fix.Late.Create.EventID.String(),
		"BROWSER_E2E_LATE_CREATE_RFX_NUMBER="+fix.Late.Create.RfxNumber,
		"BROWSER_E2E_LATE_CREATE_RESPONSE_ID="+fix.Late.Create.ResponseID.String(),
		"BROWSER_E2E_LATE_SUBMIT_EVENT_ID="+fix.Late.Submit.EventID.String(),
		"BROWSER_E2E_LATE_SUBMIT_RFX_NUMBER="+fix.Late.Submit.RfxNumber,
		"BROWSER_E2E_LATE_REJECT_EVENT_ID="+fix.Late.Reject.EventID.String(),
		"BROWSER_E2E_LATE_REJECT_RFX_NUMBER="+fix.Late.Reject.RfxNumber,
		"BROWSER_E2E_LATE_NOT_STARTED_EVENT_ID="+fix.Late.NotStarted.EventID.String(),
		"BROWSER_E2E_LATE_NOT_STARTED_RFX_NUMBER="+fix.Late.NotStarted.RfxNumber,
		"BROWSER_E2E_LATE_EXPIRED_EVENT_ID="+fix.Late.Expired.EventID.String(),
		"BROWSER_E2E_LATE_EXPIRED_RFX_NUMBER="+fix.Late.Expired.RfxNumber,
		"BROWSER_E2E_FLAG_OFF_XLSX_EVENT_ID="+fix.FlagOffXlsxEventID.String(),
		"BROWSER_E2E_FLAG_OFF_LATE_EVENT_ID="+fix.FlagOffLateEventID.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return assertE7PlaywrightNoSkipped(filepath.Join(e2eDir, "test-results", "results.json"))
}

func assertE7PlaywrightNoSkipped(resultsPath string) error {
	raw, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("E7 playwright results.json is required: %w", err)
	}
	var payload struct {
		Suites []json.RawMessage `json:"suites"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse E7 playwright results: %w", err)
	}
	skipped := countPlaywrightSkipped(raw)
	if skipped > 0 {
		return fmt.Errorf("E7 browser acceptance forbids skipped tests: skipped=%d", skipped)
	}
	return nil
}

func countPlaywrightSkipped(raw []byte) int {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return -1
	}
	return countSkippedNode(node)
}

func countSkippedNode(node any) int {
	switch typed := node.(type) {
	case map[string]any:
		count := 0
		if results, ok := typed["results"].([]any); ok {
			for _, result := range results {
				resultMap, ok := result.(map[string]any)
				if !ok {
					continue
				}
				if status, _ := resultMap["status"].(string); status == "skipped" {
					count++
				}
			}
		}
		for _, value := range typed {
			count += countSkippedNode(value)
		}
		return count
	case []any:
		count := 0
		for _, value := range typed {
			count += countSkippedNode(value)
		}
		return count
	default:
		return 0
	}
}

func assertE7FlagOffSQLClean(t *testing.T, stack *browserE7LiveStack) {
	t.Helper()
	ctx := context.Background()
	var lateCount, importCount int
	if err := stack.env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rfx.rfx_late_submission_requests WHERE rfx_event_id = $1`,
		stack.fixture.FlagOffLateEventID,
	).Scan(&lateCount); err != nil {
		t.Fatalf("flag-off late SQL: %v", err)
	}
	if lateCount != 0 {
		t.Fatalf("flag-off late writes present: count=%d event=%s", lateCount, stack.fixture.FlagOffLateEventID)
	}
	if err := stack.env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM rfx.rfx_import_analyses WHERE target_id = $1`,
		stack.fixture.FlagOffXlsxEventID,
	).Scan(&importCount); err != nil {
		t.Fatalf("flag-off import SQL: %v", err)
	}
	if importCount != 0 {
		t.Fatalf("flag-off xlsx writes present: count=%d event=%s", importCount, stack.fixture.FlagOffXlsxEventID)
	}
}

func seedE7BrowserFixture(
	t *testing.T,
	env *testEnv,
	lateSvc *service.LateSubmissionService,
	templateSvc *service.TemplateLibraryService,
	templateQSvc *service.TemplateQuestionnaireService,
) browserE7Fixture {
	t.Helper()
	ctx := context.Background()
	base := seedBuyerFixture(t, env)
	logistID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		logistID, base.TenantID, "e7-logist@test.local", "E7 Logist"); err != nil {
		t.Fatalf("seed logist user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		base.TenantID, base.CompanyA, logistID); err != nil {
		t.Fatalf("seed logist membership: %v", err)
	}
	var logistRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_LOGIST' LIMIT 1`).Scan(&logistRoleID); err != nil {
		t.Fatalf("lookup SHIPPER_LOGIST: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		base.TenantID, logistID, base.CompanyA, logistRoleID); err != nil {
		t.Fatalf("seed logist role: %v", err)
	}

	isolation := createDraftEvent(t, env, base, "RFX-E7-ISO-1")
	buyerXlsx := seedE7BuyerXlsxDraft(t, env, base, "RFX-E7-XLSX-1")
	carrierXlsx := seedE7CarrierXlsxOnFixture(t, env, base)
	late := seedE7LateOnFixture(t, env, lateSvc, base, logistID)
	templateID, templateCode := seedE7PublishedTemplate(t, env, base, templateSvc, templateQSvc)
	flagOffXlsx := createDraftEvent(t, env, base, "RFX-E7-FLAG-XLSX")
	flagOffLate := seedLateDraftEvent(t, env, lateSvc, base, "flag-off", false, "")

	return browserE7Fixture{
		TenantID:             base.TenantID,
		BuyerCompanyID:       base.CompanyA,
		OtherBuyerCompanyID:  base.CompanyB,
		CarrierCompanyID:     base.CarrierID,
		CompetitorCompanyID:  carrierXlsx.CompetitorCompanyID,
		BuyerUserID:          base.BuyerA.UserID,
		OtherBuyerUserID:     base.BuyerB.UserID,
		CarrierUserID:        base.CarrierAct.UserID,
		LogistUserID:         logistID,
		BuyerJWT:             browserStudioJWT(base.BuyerA.UserID, base.TenantID),
		OtherBuyerJWT:        browserStudioJWT(base.BuyerB.UserID, base.TenantID),
		CarrierJWT:           browserStudioJWT(base.CarrierAct.UserID, base.TenantID),
		LogistJWT:            browserStudioJWT(logistID, base.TenantID),
		IsolationEventID:     isolation.ID,
		IsolationRfxNumber:   isolation.RfxNumber,
		BuyerXlsxEventID:     buyerXlsx.ID,
		BuyerXlsxRfxNumber:   buyerXlsx.RfxNumber,
		CarrierXlsxEventID:   carrierXlsx.EventID,
		CarrierXlsxResponse:  carrierXlsx.ResponseID,
		CarrierXlsxRfxNumber: carrierXlsx.RfxNumber,
		CarrierXlsxLotID:     carrierXlsx.LotID,
		CompetitorOffer:      carrierXlsx.CompetitorOffer,
		CompetitorName:       carrierXlsx.CompetitorName,
		CompetitorAnswer:     carrierXlsx.CompetitorAnswer,
		TemplateID:           templateID,
		TemplateCode:         templateCode,
		Late:                 late,
		FlagOffXlsxEventID:   flagOffXlsx.ID,
		FlagOffLateEventID:   flagOffLate.EventID,
	}
}

func seedE7BuyerXlsxDraft(t *testing.T, env *testEnv, fix buyerFixture, rfxNumber string) *domain.RfxEvent {
	t.Helper()
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, rfxNumber)
	enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("xlsx section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes",
	}); err != nil {
		t.Fatalf("xlsx question: %v", err)
	}
	desc := "Lane bundle"
	category := "FREIGHT"
	if _, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Lane bundle",
		Description: &desc, Category: &category,
	}); err != nil {
		t.Fatalf("xlsx lot: %v", err)
	}
	return event
}

func seedE7CarrierXlsxOnFixture(t *testing.T, env *testEnv, fix buyerFixture) browserCarrierXlsxFixture {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(48 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID:         fix.TenantID,
		OwnerCompanyID:   fix.CompanyA,
		Title:            "E7 Carrier XLSX",
		RfxType:          "SPOT_RFQ",
		Category:         "FREIGHT",
		RfxNumber:        "RFX-E7-CR-XLSX-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("carrier xlsx event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("carrier xlsx participant: %v", err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("carrier xlsx section: %v", err)
	}
	question, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes",
	})
	if err != nil {
		t.Fatalf("carrier xlsx question: %v", err)
	}
	desc := "Lane bundle"
	category := "FREIGHT"
	lot, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Lane bundle",
		Description: &desc, Category: &category,
	})
	if err != nil {
		t.Fatalf("carrier xlsx lot: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("carrier xlsx publish: %v", err)
	}
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("carrier xlsx start: %v", err)
	}
	if _, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: question.ID, Value: json.RawMessage(`"YES"`)}},
	}); err != nil {
		t.Fatalf("carrier xlsx answers: %v", err)
	}
	comment := "own-offer"
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, ws.Response.ID, []domain.UpsertOfferLineInput{{
		RfxLotID: lot.ID, Amount: 12345.67, CurrencyCode: "RUB", Comment: &comment,
	}}); err != nil {
		t.Fatalf("carrier xlsx offer: %v", err)
	}
	response, err := env.rfxRepo.GetResponseByID(ctx, ws.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("carrier xlsx reload: %v", err)
	}
	competitor := seedCarrierXlsxCompetitor(t, env, fix, event.ID, lot.ID, question.ID)
	return browserCarrierXlsxFixture{
		TenantID:             fix.TenantID,
		BuyerCompanyID:       fix.CompanyA,
		CarrierCompanyID:     fix.CarrierID,
		UserID:               fix.CarrierAct.UserID,
		EventID:              event.ID,
		ResponseID:           response.ID,
		LotID:                lot.ID,
		JWT:                  browserStudioJWT(fix.CarrierAct.UserID, fix.TenantID),
		RfxNumber:            event.RfxNumber,
		CompetitorOffer:      competitor.offer,
		CompetitorName:       competitor.name,
		CompetitorAnswer:     competitor.answer,
		CompetitorCompanyID:  competitor.companyID,
		CompetitorResponseID: competitor.responseID,
	}
}

func seedE7LateOnFixture(
	t *testing.T,
	env *testEnv,
	lateSvc *service.LateSubmissionService,
	fix buyerFixture,
	logistID uuid.UUID,
) browserLateSubmissionFixture {
	t.Helper()
	return browserLateSubmissionFixture{
		TenantID:         fix.TenantID,
		BuyerCompanyID:   fix.CompanyA,
		CarrierCompanyID: fix.CarrierID,
		CarrierUserID:    fix.CarrierAct.UserID,
		BuyerUserID:      fix.BuyerA.UserID,
		LogistUserID:     logistID,
		CarrierJWT:       browserStudioJWT(fix.CarrierAct.UserID, fix.TenantID),
		BuyerJWT:         browserStudioJWT(fix.BuyerA.UserID, fix.TenantID),
		LogistJWT:        browserStudioJWT(logistID, fix.TenantID),
		Create:           seedLateDraftEvent(t, env, lateSvc, fix, "create", false, ""),
		Submit:           seedLateDraftEvent(t, env, lateSvc, fix, "submit", true, "requested"),
		Reject:           seedLateDraftEvent(t, env, lateSvc, fix, "reject", true, "requested"),
		NotStarted:       seedLateDraftEvent(t, env, lateSvc, fix, "window-open", true, "not_started"),
		Expired:          seedLateDraftEvent(t, env, lateSvc, fix, "window-closed", true, "expired"),
	}
}

func seedE7PublishedTemplate(
	t *testing.T,
	env *testEnv,
	fix buyerFixture,
	templateSvc *service.TemplateLibraryService,
	templateQSvc *service.TemplateQuestionnaireService,
) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	code := "E7-TPL-" + uuid.NewString()[:8]
	owner := fix.CompanyA
	detail, err := templateSvc.CreateTemplate(ctx, fix.BuyerA, domain.CreateTemplateInput{
		TemplateCode:   code,
		NameI18n:       json.RawMessage(`{"ru-RU":"E7 Template","en-US":"E7 Template","zh-CN":"E7 Template"}`),
		RfxType:        e7StringPtr("SPOT_RFQ"),
		OwnerCompanyID: &owner,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	section, err := templateQSvc.CreateSection(ctx, fix.BuyerA, detail.Template.ID, domain.CreateSectionInput{
		SectionCode: "GENERAL", Title: "General",
	})
	if err != nil {
		t.Fatalf("template section: %v", err)
	}
	if _, err := templateQSvc.CreateQuestion(ctx, fix.BuyerA, detail.Template.ID, section.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET_SIZE", QuestionType: domain.QuestionTypeText, Label: "Fleet size", Required: true,
	}); err != nil {
		t.Fatalf("template question: %v", err)
	}
	loaded, err := templateSvc.GetTemplate(ctx, fix.BuyerA, detail.Template.ID)
	if err != nil || loaded.DraftVersion == nil {
		t.Fatalf("reload template: %v", err)
	}
	if _, err := templateSvc.PublishTemplateVersion(ctx, fix.BuyerA, detail.Template.ID, "e7-tpl-pub", domain.PublishTemplateVersionInput{
		ExpectedTemplateVersion: loaded.Template.Version,
		ExpectedDraftVersion:    loaded.DraftVersion.Version,
		ChangeSummary:           "E7 browser template",
	}); err != nil {
		t.Fatalf("publish template: %v", err)
	}
	return detail.Template.ID, code
}

type e7CompanyStub struct {
	server *httptest.Server
}

func e7Memberships(fix browserE7Fixture) map[string][]map[string]any {
	return map[string][]map[string]any{
		fix.BuyerUserID.String():      {membershipItem(fix.BuyerCompanyID, "Buyer A", "SHIPPER", "PROCUREMENT_MANAGER")},
		fix.OtherBuyerUserID.String(): {membershipItem(fix.OtherBuyerCompanyID, "Buyer B", "SHIPPER", "PROCUREMENT_MANAGER")},
		fix.CarrierUserID.String():    {membershipItem(fix.CarrierCompanyID, "Carrier A", "CARRIER", "CARRIER_DISPATCHER")},
		fix.LogistUserID.String():     {membershipItem(fix.BuyerCompanyID, "Buyer A", "SHIPPER", "SHIPPER_LOGIST")},
	}
}

func startE7CompanyStub(t *testing.T, fix browserE7Fixture) *e7CompanyStub {
	t.Helper()
	companies := []map[string]any{
		companyItem(fix.BuyerCompanyID, fix.TenantID, "Buyer A", "SHIPPER"),
		companyItem(fix.OtherBuyerCompanyID, fix.TenantID, "Buyer B", "SHIPPER"),
		companyItem(fix.CarrierCompanyID, fix.TenantID, "Carrier A", "CARRIER"),
		companyItem(fix.CompetitorCompanyID, fix.TenantID, fix.CompetitorName, "CARRIER"),
	}
	memberships := e7Memberships(fix)
	stub := &e7CompanyStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/users/") && strings.HasSuffix(r.URL.Path, "/companies") {
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			userID := ""
			for i, part := range parts {
				if part == "users" && i+1 < len(parts) {
					userID = parts[i+1]
					break
				}
			}
			items := memberships[userID]
			if items == nil {
				items = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": len(items), "limit": 200, "offset": 0})
			return
		}
		if r.URL.Path == "/v1/companies" || strings.HasSuffix(r.URL.Path, "/companies") {
			wanted := r.URL.Query().Get("company_type")
			items := make([]map[string]any, 0, len(companies))
			for _, company := range companies {
				if wanted == "" || company["company_type"] == wanted {
					items = append(items, company)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": len(items), "limit": 200, "offset": 0})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *e7CompanyStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func companyItem(id, tenant uuid.UUID, name, typ string) map[string]any {
	return map[string]any{
		"id": id.String(), "tenant_id": tenant.String(), "legal_name": name,
		"company_type": typ, "country_code": "RU", "preferred_locale": "en-US", "status": "ACTIVE",
	}
}

func e7StringPtr(v string) *string { return &v }

func membershipItem(companyID uuid.UUID, name, typ, role string) map[string]any {
	return map[string]any{
		"membership_id":     companyID.String() + "-" + role,
		"company_id":        companyID.String(),
		"legal_name":        name,
		"company_type":      typ,
		"membership_status": "ACTIVE",
		"roles": []map[string]any{{
			"role_id": companyID.String() + "-" + role,
			"code":    role,
			"name":    role,
		}},
	}
}
