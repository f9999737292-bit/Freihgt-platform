//go:build integration

package studio

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/freight-platform/rfx-service/internal/config"
	"github.com/freight-platform/rfx-service/internal/domain"
	httpserver "github.com/freight-platform/rfx-service/internal/http"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

const buyerXlsxBrowserPort = "3031"

type browserBuyerXlsxLiveStack struct {
	webURL      string
	gatewayURL  string
	fixture     browserStudioFixture
	rfxSrv      *http.Server
	gatewayProc *browserGatewayProcess
	webCmd      *webProcurementCmd
}

func TestRfxBuyerXlsx_BrowserE2E_LiveUpdateDraft(t *testing.T) {
	requireBuyerXlsxBrowserGate(t)
	t.Cleanup(func() { verifyBrowserHarnessCleanup(t) })
	stack := startBuyerXlsxLiveStack(t)
	t.Cleanup(func() { stack.shutdown(t) })
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyBuyerXlsxGatewayProbe(t, stack)
	if err := runBuyerXlsxPlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright buyer XLSX suite: %v", err)
	}
}

func requireBuyerXlsxBrowserGate(t *testing.T) {
	t.Helper()
	if os.Getenv("BROWSER_E2E") != "1" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("BROWSER_E2E=1 is required in CI for the buyer XLSX browser gate")
		}
		t.Skip("set BROWSER_E2E=1 to run the buyer XLSX browser gate")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required for the buyer XLSX browser gate")
	}
}

func startBuyerXlsxLiveStack(t *testing.T) *browserBuyerXlsxLiveStack {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedBuyerXlsxBrowserFixture(t, env)
	rfxURL, rfxSrv := startBuyerXlsxRfxService(t, env)
	identity := startBrowserIdentityStub(t, browserIdentityRolesForBuyer(fix.UserID.String()))
	webOrigin := browserGatewayEnvForStack(t, buyerXlsxBrowserPort)
	gatewayURL, gatewayProc := startBuyerXlsxProductionGateway(t, rfxURL, webOrigin, identity)
	webURL, webCmd := startBuyerXlsxWebProcurement(t, gatewayURL, fix, buyerXlsxBrowserPort)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserBuyerXlsxLiveStack{
		webURL:      webURL,
		gatewayURL:  gatewayURL,
		fixture:     fix,
		rfxSrv:      rfxSrv,
		gatewayProc: gatewayProc,
		webCmd:      webCmd,
	}
}

func seedBuyerXlsxBrowserFixture(t *testing.T, env *testEnv) browserStudioFixture {
	t.Helper()
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-XLSX-BRW-1")
	enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(context.Background(), fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(context.Background(), fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes",
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	desc := "Lane bundle"
	category := "FREIGHT"
	if _, err := env.rfxSvc.CreateLot(context.Background(), fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Lane bundle",
		Description: &desc, Category: &category,
	}); err != nil {
		t.Fatalf("create lot: %v", err)
	}
	return browserStudioFixture{
		TenantID:  fix.TenantID,
		CompanyID: fix.CompanyA,
		UserID:    fix.BuyerA.UserID,
		EventID:   event.ID,
		JWT:       browserStudioJWT(fix.BuyerA.UserID, fix.TenantID),
		RfxNumber: event.RfxNumber,
	}
}

func startBuyerXlsxRfxService(t *testing.T, env *testEnv) (string, *http.Server) {
	t.Helper()
	importRepo := repository.NewImportAnalysisRepository(env.pool)
	txRunner := repository.NewTransactionRunner(env.pool)
	excelSvc := service.NewExcelExchangeService(
		env.rfxRepo, env.qRepo, env.rfxSvc, importRepo, env.idemRepo, env.auditRepo, txRunner, env.answerRepo,
	)
	cfg := config.Config{RfxExcelExchangeEnabled: true, Environment: "test"}
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

func startBuyerXlsxProductionGateway(t *testing.T, rfxServiceURL, origins string, identity *browserIdentityStub) (string, *browserGatewayProcess) {
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
		"CORS_ALLOWED_ORIGINS=" + origins,
		"RATE_LIMIT_ENABLED=false",
		"OPENAPI_DIR=" + openAPIDir,
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
		"RFX_EXCEL_EXCHANGE_ENABLED=true",
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	verifyBrowserGatewayRfxRoute(t, gatewayURL, rfxServiceURL)
	return gatewayURL, proc
}

func startBuyerXlsxWebProcurement(t *testing.T, gatewayURL string, fix browserStudioFixture, port string) (string, *webProcurementCmd) {
	t.Helper()
	env := append(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED=true",
		"NUXT_E2E_DISABLE_SSR=true",
	)
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

func (s *browserBuyerXlsxLiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.webCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
	verifyDevPortsReleased(t, buyerXlsxBrowserPort)
}

func verifyBuyerXlsxGatewayProbe(t *testing.T, stack *browserBuyerXlsxLiveStack) {
	t.Helper()
	eventURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.EventID.String()
	req, err := http.NewRequest(http.MethodGet, eventURL, nil)
	if err != nil {
		t.Fatalf("probe request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.JWT)
	req.Header.Set("X-Company-ID", stack.fixture.CompanyID.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe buyer XLSX event: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe buyer XLSX event: status=%d url=%s", resp.StatusCode, eventURL)
	}

	exportURL := eventURL + "/xlsx-export"
	exportReq, err := http.NewRequest(http.MethodGet, exportURL, nil)
	if err != nil {
		t.Fatalf("export probe request: %v", err)
	}
	exportReq.Header.Set("Authorization", "Bearer "+stack.fixture.JWT)
	exportReq.Header.Set("X-Company-ID", stack.fixture.CompanyID.String())
	exportResp, err := http.DefaultClient.Do(exportReq)
	if err != nil {
		t.Fatalf("probe buyer XLSX export: %v", err)
	}
	exportResp.Body.Close()
	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("probe buyer XLSX export: status=%d url=%s (excel flag or route missing)", exportResp.StatusCode, exportURL)
	}
}

func runBuyerXlsxPlaywrightSuite(t *testing.T, stack *browserBuyerXlsxLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "buyer-xlsx")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"CI=true",
		"BROWSER_E2E=1",
		"BROWSER_E2E_REQUIRE=1",
		"BROWSER_E2E_EXCEL_FLAG=1",
		"BROWSER_E2E_WEB_URL="+stack.webURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_JWT="+fix.JWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.CompanyID.String(),
		"BROWSER_E2E_EVENT_ID="+fix.EventID.String(),
		"BROWSER_E2E_RFX_NUMBER="+fix.RfxNumber,
		"BROWSER_E2E_USER_ID="+fix.UserID.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
