//go:build integration

package studio

import (
	"context"
	"encoding/json"
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
	buyerXlsxCreateBrowserPort        = "3035"
	buyerXlsxCreateFlagOffBrowserPort = "3036"
)

type browserBuyerXlsxCreateFixture struct {
	TenantID       uuid.UUID
	OtherTenantID  uuid.UUID
	CompanyID      uuid.UUID
	OtherCompanyID uuid.UUID
	CarrierID      uuid.UUID
	BuyerUserID    uuid.UUID
	LogistUserID   uuid.UUID
	CarrierUserID  uuid.UUID
	ForeignUserID  uuid.UUID
	BuyerJWT       string
	LogistJWT      string
	CarrierJWT     string
	ForeignJWT     string
	WorkbookPath   string
	FlagOffNumber  string
	SourceEventID  uuid.UUID
}

type browserBuyerXlsxCreateLiveStack struct {
	webURL         string
	flagOffWebURL  string
	gatewayURL     string
	flagOffGateway string
	fixture        browserBuyerXlsxCreateFixture
	rfxSrv         *http.Server
	flagOffRfx     *http.Server
	gatewayProc    *browserGatewayProcess
	flagOffGw      *browserGatewayProcess
	webCmd         *webProcurementCmd
	flagOffCmd     *webProcurementCmd
	env            *testEnv
}

func TestRfxBuyerXlsxCreate_BrowserE2E_LiveCreateDraft(t *testing.T) {
	requireBuyerXlsxCreateBrowserGate(t)
	t.Cleanup(func() { verifyBrowserHarnessCleanup(t) })
	stack := startBuyerXlsxCreateLiveStack(t)
	t.Cleanup(func() { stack.shutdown(t) })
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	beforeDownload := snapshotBuyerXlsxCreateWrites(t, stack)
	if err := runBuyerXlsxCreatePlaywrightSuite(t, stack, "--grep", "blank template download writes nothing"); err != nil {
		t.Fatalf("playwright blank template no-write proof: %v", err)
	}
	assertBuyerXlsxCreateWritesUnchanged(t, stack, beforeDownload)
	beforeSuite := snapshotBuyerXlsxCreateWrites(t, stack)
	if err := runBuyerXlsxCreatePlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright buyer XLSX create suite: %v", err)
	}
	afterSuite := snapshotBuyerXlsxCreateWrites(t, stack)
	if afterSuite.analyses <= beforeSuite.analyses {
		t.Fatalf("preview after template download did not persist an analysis: before=%d after=%d", beforeSuite.analyses, afterSuite.analyses)
	}
	assertBuyerXlsxCreateFlagOffSQLClean(t, stack)
}

func requireBuyerXlsxCreateBrowserGate(t *testing.T) {
	t.Helper()
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Fatal("BROWSER_E2E=1 is required for the buyer XLSX create browser gate")
	}
	if os.Getenv("BROWSER_E2E_REQUIRE") != "1" {
		t.Fatal("BROWSER_E2E_REQUIRE=1 is required for the buyer XLSX create browser gate")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required for the buyer XLSX create browser gate")
	}
}

func startBuyerXlsxCreateLiveStack(t *testing.T) *browserBuyerXlsxCreateLiveStack {
	t.Helper()
	env := setupTestEnv(t)
	importRepo := repository.NewImportAnalysisRepository(env.pool)
	txRunner := repository.NewTransactionRunner(env.pool)
	excelSvc := service.NewExcelExchangeService(
		env.rfxRepo, env.qRepo, env.rfxSvc, importRepo, env.idemRepo, env.auditRepo, txRunner, env.answerRepo,
	)
	fix := seedBuyerXlsxCreateBrowserFixture(t, env, excelSvc)

	rfxURL, rfxSrv := startBuyerXlsxCreateRfxService(t, env, excelSvc, true)
	flagOffURL, flagOffSrv := startBuyerXlsxCreateRfxService(t, env, excelSvc, false)

	identity := startBrowserIdentityStubWithMemberships(t, map[string][]string{
		fix.BuyerUserID.String():   {"PROCUREMENT_MANAGER"},
		fix.LogistUserID.String():  {"SHIPPER_LOGIST"},
		fix.CarrierUserID.String(): {"CARRIER_DISPATCHER"},
		fix.ForeignUserID.String(): {"PROCUREMENT_MANAGER"},
	}, map[string][]map[string]any{
		fix.BuyerUserID.String():   {membershipItem(fix.CompanyID, "Buyer A", "SHIPPER", "PROCUREMENT_MANAGER")},
		fix.LogistUserID.String():  {membershipItem(fix.CompanyID, "Buyer A", "SHIPPER", "SHIPPER_LOGIST")},
		fix.CarrierUserID.String(): {membershipItem(fix.CarrierID, "Carrier C", "CARRIER", "CARRIER_DISPATCHER")},
	})
	companyStub := startBuyerXlsxCreateCompanyStub(t, fix)

	onOrigin := "http://127.0.0.1:" + buyerXlsxCreateBrowserPort
	offOrigin := "http://127.0.0.1:" + buyerXlsxCreateFlagOffBrowserPort
	corsOrigins := strings.Join([]string{
		onOrigin, offOrigin,
		"http://localhost:" + buyerXlsxCreateBrowserPort,
		"http://localhost:" + buyerXlsxCreateFlagOffBrowserPort,
	}, ",")

	gatewayURL, gatewayProc := startBuyerXlsxCreateProductionGateway(t, rfxURL, corsOrigins, identity, companyStub.URL(), true)
	flagOffGateway, flagOffGw := startBuyerXlsxCreateProductionGateway(t, flagOffURL, corsOrigins, identity, companyStub.URL(), false)

	ensureDevPortFree(t, buyerXlsxCreateBrowserPort)
	ensureDevPortFree(t, buyerXlsxCreateFlagOffBrowserPort)
	webURL, webCmd := startBuyerXlsxCreateWebProcurement(t, gatewayURL, fix, buyerXlsxCreateBrowserPort, true)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	flagOffWeb, flagOffCmd := startBuyerXlsxCreateWebProcurement(t, flagOffGateway, fix, buyerXlsxCreateFlagOffBrowserPort, false)
	waitForHTTP200(t, flagOffWeb+"/login", 120*time.Second)

	return &browserBuyerXlsxCreateLiveStack{
		webURL:         webURL,
		flagOffWebURL:  flagOffWeb,
		gatewayURL:     gatewayURL,
		flagOffGateway: flagOffGateway,
		fixture:        fix,
		rfxSrv:         rfxSrv,
		flagOffRfx:     flagOffSrv,
		gatewayProc:    gatewayProc,
		flagOffGw:      flagOffGw,
		webCmd:         webCmd,
		flagOffCmd:     flagOffCmd,
		env:            env,
	}
}

func seedBuyerXlsxCreateBrowserFixture(
	t *testing.T,
	env *testEnv,
	excelSvc *service.ExcelExchangeService,
) browserBuyerXlsxCreateFixture {
	t.Helper()
	ctx := context.Background()
	base := seedBuyerFixture(t, env)
	logistID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		logistID, base.TenantID, "f5-logist@test.local", "F5 Logist"); err != nil {
		t.Fatalf("seed logist user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		base.TenantID, base.CompanyA, logistID); err != nil {
		t.Fatalf("seed logist membership: %v", err)
	}

	event := createDraftEvent(t, env, base, "RFX-F5-WB-SRC")
	enableQuestionnaire(t, env, base.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, base.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, base.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes",
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	desc := "Lane bundle"
	category := "FREIGHT"
	if _, err := env.rfxSvc.CreateLot(ctx, base.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: base.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Lane bundle",
		Description: &desc, Category: &category,
	}); err != nil {
		t.Fatalf("create lot: %v", err)
	}

	data, _, err := excelSvc.ExportBuyerDraftWorkbook(ctx, base.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("export create workbook: %v", err)
	}
	if len(data) < 4 || string(data[:2]) != "PK" {
		t.Fatalf("exported workbook is not xlsx: %d bytes", len(data))
	}
	workbookPath := filepath.Join(t.TempDir(), "f5-create-source.xlsx")
	if err := os.WriteFile(workbookPath, data, 0o600); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	return browserBuyerXlsxCreateFixture{
		TenantID:       base.TenantID,
		OtherTenantID:  base.OtherTenantID,
		CompanyID:      base.CompanyA,
		OtherCompanyID: base.CompanyB,
		CarrierID:      base.CarrierID,
		BuyerUserID:    base.BuyerA.UserID,
		LogistUserID:   logistID,
		CarrierUserID:  base.CarrierAct.UserID,
		ForeignUserID:  base.CrossTenant.UserID,
		BuyerJWT:       browserStudioJWT(base.BuyerA.UserID, base.TenantID),
		LogistJWT:      browserStudioJWT(logistID, base.TenantID),
		CarrierJWT:     browserStudioJWT(base.CarrierAct.UserID, base.TenantID),
		ForeignJWT:     browserStudioJWT(base.CrossTenant.UserID, base.OtherTenantID),
		WorkbookPath:   workbookPath,
		FlagOffNumber:  "RFX-F5-FLAG-OFF-1",
		SourceEventID:  event.ID,
	}
}

func startBuyerXlsxCreateRfxService(
	t *testing.T,
	env *testEnv,
	excelSvc *service.ExcelExchangeService,
	flagOn bool,
) (string, *http.Server) {
	t.Helper()
	cfg := config.Config{RfxExcelExchangeEnabled: flagOn, Environment: "test"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpserver.NewRouter(
		log, env.pool, cfg,
		env.rfxSvc, env.qSvc, env.versionSvc,
		nil, nil, nil,
		env.crSvc, nil, excelSvc, nil,
		env.scoreModelSvc, env.scoringSvc,
		nil, nil, nil,
	)
	return listenHTTPServer(t, handler)
}

func startBuyerXlsxCreateProductionGateway(
	t *testing.T,
	rfxServiceURL, origins string,
	identity *browserIdentityStub,
	companyURL string,
	flagOn bool,
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
	}
	if flagOn {
		env = append(env, "RFX_EXCEL_EXCHANGE_ENABLED=true")
	} else {
		env = append(env, "RFX_EXCEL_EXCHANGE_ENABLED=false")
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	verifyBrowserGatewayRfxRoute(t, gatewayURL, rfxServiceURL)
	return gatewayURL, proc
}

func startBuyerXlsxCreateWebProcurement(
	t *testing.T,
	gatewayURL string,
	fix browserBuyerXlsxCreateFixture,
	port string,
	flagOn bool,
) (string, *webProcurementCmd) {
	t.Helper()
	env := overrideProcessEnv(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_E2E_DISABLE_SSR=true",
		"NUXT_E2E_DISABLE_DEVTOOLS=true",
	)
	if flagOn {
		env = overrideProcessEnv(env,
			"NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED=true",
			"RFX_EXCEL_EXCHANGE_UI=1",
		)
	} else {
		env = overrideProcessEnv(env,
			"NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED=false",
			"RFX_EXCEL_EXCHANGE_UI=0",
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

func (s *browserBuyerXlsxCreateLiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.flagOffCmd)
	stopBrowserWebProcurement(t, s.webCmd)
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
	verifyDevPortsReleased(t, buyerXlsxCreateBrowserPort, buyerXlsxCreateFlagOffBrowserPort)
}

func runBuyerXlsxCreatePlaywrightSuite(t *testing.T, stack *browserBuyerXlsxCreateLiveStack, extra ...string) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "buyer-xlsx-create")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	args := []string{"playwright", "test", "--config", configPath}
	args = append(args, extra...)
	cmd := exec.Command("npx", args...)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"CI=true",
		"BROWSER_E2E=1",
		"BROWSER_E2E_REQUIRE=1",
		"BROWSER_E2E_EXCEL_FLAG=1",
		"BROWSER_E2E_WEB_URL="+stack.webURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_FLAG_OFF_WEB_URL="+stack.flagOffWebURL,
		"BROWSER_E2E_FLAG_OFF_GATEWAY_URL="+stack.flagOffGateway,
		"BROWSER_E2E_JWT="+fix.BuyerJWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.CompanyID.String(),
		"BROWSER_E2E_USER_ID="+fix.BuyerUserID.String(),
		"BROWSER_E2E_WORKBOOK_PATH="+fix.WorkbookPath,
		"BROWSER_E2E_LOGIST_JWT="+fix.LogistJWT,
		"BROWSER_E2E_LOGIST_USER_ID="+fix.LogistUserID.String(),
		"BROWSER_E2E_CARRIER_JWT="+fix.CarrierJWT,
		"BROWSER_E2E_CARRIER_USER_ID="+fix.CarrierUserID.String(),
		"BROWSER_E2E_CARRIER_COMPANY_ID="+fix.CarrierID.String(),
		"BROWSER_E2E_FOREIGN_JWT="+fix.ForeignJWT,
		"BROWSER_E2E_FOREIGN_TENANT_ID="+fix.OtherTenantID.String(),
		"BROWSER_E2E_FOREIGN_USER_ID="+fix.ForeignUserID.String(),
		"BROWSER_E2E_OTHER_COMPANY_ID="+fix.OtherCompanyID.String(),
		"BROWSER_E2E_FLAG_OFF_RFX_NUMBER="+fix.FlagOffNumber,
		"BROWSER_E2E_SOURCE_EVENT_ID="+fix.SourceEventID.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type buyerXlsxCreateWriteCounts struct {
	events       int
	analyses     int
	participants int
	responses    int
	awards       int
	orders       int
	idempotency  int
}

func snapshotBuyerXlsxCreateWrites(t *testing.T, stack *browserBuyerXlsxCreateLiveStack) buyerXlsxCreateWriteCounts {
	t.Helper()
	count := func(query string) int {
		t.Helper()
		var n int
		if err := stack.env.pool.QueryRow(context.Background(), query).Scan(&n); err != nil {
			t.Fatalf("write snapshot %s: %v", query, err)
		}
		return n
	}
	return buyerXlsxCreateWriteCounts{
		events:       count(`SELECT COUNT(*) FROM rfx.rfx_events`),
		analyses:     count(`SELECT COUNT(*) FROM rfx.rfx_import_analyses`),
		participants: count(`SELECT COUNT(*) FROM rfx.rfx_participants`),
		responses:    count(`SELECT COUNT(*) FROM rfx.rfx_responses`),
		awards:       count(`SELECT COUNT(*) FROM rfx.rfx_awards`),
		orders:       count(`SELECT COUNT(*) FROM transport.transport_orders`),
		idempotency:  count(`SELECT COUNT(*) FROM rfx.rfx_idempotency_records`),
	}
}

func assertBuyerXlsxCreateWritesUnchanged(t *testing.T, stack *browserBuyerXlsxCreateLiveStack, before buyerXlsxCreateWriteCounts) {
	t.Helper()
	after := snapshotBuyerXlsxCreateWrites(t, stack)
	if after != before {
		t.Fatalf("blank template download wrote rows: before=%+v after=%+v", before, after)
	}
}

func assertBuyerXlsxCreateFlagOffSQLClean(t *testing.T, stack *browserBuyerXlsxCreateLiveStack) {
	t.Helper()
	var eventCount int
	if err := stack.env.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM rfx.rfx_events WHERE tenant_id = $1 AND rfx_number = $2`,
		stack.fixture.TenantID, stack.fixture.FlagOffNumber,
	).Scan(&eventCount); err != nil {
		t.Fatalf("flag-off event SQL: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("flag-off preview/commit wrote an event: count=%d number=%s", eventCount, stack.fixture.FlagOffNumber)
	}
}

type buyerXlsxCreateCompanyStub struct {
	server *httptest.Server
}

func startBuyerXlsxCreateCompanyStub(t *testing.T, fix browserBuyerXlsxCreateFixture) *buyerXlsxCreateCompanyStub {
	t.Helper()
	companies := []map[string]any{
		companyItem(fix.CompanyID, fix.TenantID, "Buyer A", "SHIPPER"),
		companyItem(fix.OtherCompanyID, fix.TenantID, "Buyer B", "SHIPPER"),
		companyItem(fix.CarrierID, fix.TenantID, "Carrier C", "CARRIER"),
	}
	stub := &buyerXlsxCreateCompanyStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/companies" || strings.HasSuffix(r.URL.Path, "/companies") {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": companies, "total": len(companies), "limit": 200, "offset": 0})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *buyerXlsxCreateCompanyStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}
