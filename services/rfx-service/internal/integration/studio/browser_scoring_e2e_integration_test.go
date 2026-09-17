//go:build integration

package studio

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type browserScoringLiveStack struct {
	adminURL       string
	procurementURL string
	gatewayURL     string
	fixture        browserScoringFixture
	rfxSrv         *http.Server
	gatewayProc    *browserGatewayProcess
	adminCmd       *webAdminCmd
	procurementCmd *webProcurementCmd
}

type webProcurementCmd struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	port   string
	logs   []*os.File
}

func TestRfxScoringV3_BrowserE2E_Acceptance(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when BROWSER_E2E=1")
	}
	stack := startBrowserScoringLiveStack(t)
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyScoringGatewayProbe(t, stack)
	if err := runScoringV3PlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright scoring v3 suite: %v", err)
	}
}

func TestRfxScoringV3_BrowserE2E_ReadinessDiagnostics(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when BROWSER_E2E=1")
	}
	stack := startBrowserScoringLiveStack(t)
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyScoringGatewayProbe(t, stack)
	if err := runScoringV3PlaywrightReadinessSuite(t, stack); err != nil {
		t.Fatalf("playwright scoring v3 readiness suite: %v", err)
	}
}

func startBrowserScoringLiveStack(t *testing.T) *browserScoringLiveStack {
	t.Helper()
	stack := &browserScoringLiveStack{}
	t.Cleanup(func() { stack.shutdown(t) })

	const adminPort = "3022"
	const procurementPort = "3023"
	env := setupTestEnv(t)
	fix := seedBrowserScoringV3Fixture(t, env)
	rfxURL, rfxSrv := listenHTTPServer(t, newBrowserScoringV3Router(env))
	roles := browserIdentityRolesForBuyer(fix.UserID.String())
	for uid, r := range browserIdentityRolesForCarrier(fix.CarrierAUserID.String()) {
		roles[uid] = r
	}
	for uid, r := range browserIdentityRolesForCarrier(fix.CarrierBUserID.String()) {
		roles[uid] = r
	}
	identity := startBrowserIdentityStub(t, roles)
	adminOrigin := "http://127.0.0.1:" + adminPort
	procOrigin := "http://127.0.0.1:" + procurementPort
	gatewayURL, gatewayProc := startBrowserProductionGatewayWithOrigins(t, rfxURL, adminOrigin+","+procOrigin, identity)
	forkDraftViaProductionGateway(t, gatewayURL, fix)
	ensureDevPortFree(t, adminPort)
	ensureDevPortFree(t, procurementPort)
	adminURL, adminCmd := startBrowserWebAdmin(t, gatewayURL, fix.browserStudioFixture, adminPort)
	waitForHTTP200(t, adminURL+"/login", 120*time.Second)
	procURL, procCmd := startBrowserWebProcurement(t, gatewayURL, fix.browserStudioFixture, procurementPort)
	waitForHTTP200(t, procURL+"/login", 120*time.Second)
	stack.adminURL = adminURL
	stack.procurementURL = procURL
	stack.gatewayURL = gatewayURL
	stack.fixture = fix
	stack.rfxSrv = rfxSrv
	stack.gatewayProc = gatewayProc
	stack.adminCmd = adminCmd
	stack.procurementCmd = procCmd
	return stack
}

func startBrowserProductionGatewayWithOrigins(t *testing.T, rfxServiceURL, origins string, identity *browserIdentityStub) (string, *browserGatewayProcess) {
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
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	return gatewayURL, proc
}

func startBrowserWebProcurement(t *testing.T, gatewayURL string, fix browserStudioFixture, port string) (string, *webProcurementCmd) {
	t.Helper()
	launch := prepareNuxtDevLaunch(t, "web-procurement", port)
	ctx, cancel := context.WithCancel(context.Background())
	env := appendNuxtDevEnv(append(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_E2E_DISABLE_SSR=true",
	), launch)
	cmd, logFile := startNuxtDevCommand(t, ctx, "web-procurement", port, env)
	assertNuxtDevStarted(t, port, logFile.Name())
	return "http://127.0.0.1:" + port, &webProcurementCmd{
		cmd:    cmd,
		cancel: cancel,
		port:   port,
		logs:   []*os.File{logFile},
	}
}

func (s *browserScoringLiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.procurementCmd)
	stopBrowserWebAdmin(t, s.adminCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
	verifyDevPortsReleased(t, "3022", "3023")
}

func stopBrowserWebProcurement(t *testing.T, proc *webProcurementCmd) {
	if proc == nil {
		return
	}
	if proc.cancel != nil {
		proc.cancel()
	}
	stopNuxtDevProcess(t, proc.cmd, proc.port)
	for _, logFile := range proc.logs {
		_ = logFile.Close()
	}
}

func runScoringV3PlaywrightSuite(t *testing.T, stack *browserScoringLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "rfx-scoring-v3")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"BROWSER_E2E_ADMIN_URL="+stack.adminURL,
		"BROWSER_E2E_PROCUREMENT_URL="+stack.procurementURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_JWT="+fix.JWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.CompanyID.String(),
		"BROWSER_E2E_EVENT_ID="+fix.EventID.String(),
		"BROWSER_E2E_RFX_NUMBER="+fix.RfxNumber,
		"BROWSER_E2E_USER_ID="+fix.UserID.String(),
		"BROWSER_E2E_CARRIER_A_JWT="+fix.CarrierAJWT,
		"BROWSER_E2E_CARRIER_A_COMPANY_ID="+fix.CarrierACompanyID.String(),
		"BROWSER_E2E_CARRIER_B_JWT="+fix.CarrierBJWT,
		"BROWSER_E2E_CARRIER_B_COMPANY_ID="+fix.CarrierBCompanyID.String(),
		"BROWSER_E2E_LEGACY_EVENT_ID="+fix.LegacyEventID.String(),
		"BROWSER_E2E_LEGACY_RFX_NUMBER="+fix.LegacyRfxNumber,
	)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		t.Logf("playwright scoring v3 output:\n%s", string(output))
	}
	if err != nil {
		if report, readErr := os.ReadFile(filepath.Join(e2eDir, "test-results", "results.json")); readErr == nil && len(report) > 0 {
			t.Logf("playwright scoring v3 results.json:\n%s", string(report))
		}
	}
	return err
}

func runScoringV3PlaywrightReadinessSuite(t *testing.T, stack *browserScoringLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "rfx-scoring-v3")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"BROWSER_E2E_SCORING_READINESS=1",
		"BROWSER_E2E_ADMIN_URL="+stack.adminURL,
		"BROWSER_E2E_PROCUREMENT_URL="+stack.procurementURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_JWT="+fix.JWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.CompanyID.String(),
		"BROWSER_E2E_EVENT_ID="+fix.EventID.String(),
		"BROWSER_E2E_RFX_NUMBER="+fix.RfxNumber,
		"BROWSER_E2E_USER_ID="+fix.UserID.String(),
		"BROWSER_E2E_CARRIER_A_JWT="+fix.CarrierAJWT,
		"BROWSER_E2E_CARRIER_A_COMPANY_ID="+fix.CarrierACompanyID.String(),
		"BROWSER_E2E_CARRIER_B_JWT="+fix.CarrierBJWT,
		"BROWSER_E2E_CARRIER_B_COMPANY_ID="+fix.CarrierBCompanyID.String(),
		"BROWSER_E2E_LEGACY_EVENT_ID="+fix.LegacyEventID.String(),
		"BROWSER_E2E_LEGACY_RFX_NUMBER="+fix.LegacyRfxNumber,
	)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		t.Logf("playwright scoring v3 readiness output:\n%s", string(output))
	}
	if err != nil {
		if report, readErr := os.ReadFile(filepath.Join(e2eDir, "test-results", "results.json")); readErr == nil && len(report) > 0 {
			t.Logf("playwright scoring v3 readiness results.json:\n%s", string(report))
		}
	}
	return err
}
