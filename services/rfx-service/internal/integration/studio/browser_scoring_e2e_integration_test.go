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
	cmd  *exec.Cmd
	logs []*os.File
}

func TestRfxScoringV3_BrowserE2E_Acceptance(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when BROWSER_E2E=1")
	}
	stack := startBrowserScoringLiveStack(t)
	t.Cleanup(stack.shutdown)
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	if err := runScoringV3PlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright scoring v3 suite: %v", err)
	}
}

func startBrowserScoringLiveStack(t *testing.T) *browserScoringLiveStack {
	t.Helper()
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
	adminURL, adminCmd := startBrowserWebAdmin(t, gatewayURL, fix.browserStudioFixture, adminPort)
	procURL, procCmd := startBrowserWebProcurement(t, gatewayURL, fix.browserStudioFixture, procurementPort)
	waitForHTTP200(t, adminURL+"/login", 120*time.Second)
	waitForHTTP200(t, procURL+"/login", 120*time.Second)
	return &browserScoringLiveStack{
		adminURL:       adminURL,
		procurementURL: procURL,
		gatewayURL:     gatewayURL,
		fixture:        fix,
		rfxSrv:         rfxSrv,
		gatewayProc:    gatewayProc,
		adminCmd:       adminCmd,
		procurementCmd: procCmd,
	}
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
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	cmd := exec.Command("pnpm", "--filter", "@freight-platform/web-procurement", "exec", "nuxt", "dev", "--port", port, "--host", "127.0.0.1")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_E2E_DISABLE_SSR=true",
	)
	logFile, err := os.CreateTemp("", "rfx-scoring-proc-"+port+"-*.log")
	if err != nil {
		t.Fatalf("create procurement log: %v", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start web-procurement dev: %v", err)
	}
	return "http://127.0.0.1:" + port, &webProcurementCmd{cmd: cmd, logs: []*os.File{logFile}}
}

func (s *browserScoringLiveStack) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(s.procurementCmd)
	stopBrowserWebAdmin(s.adminCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
}

func stopBrowserWebProcurement(proc *webProcurementCmd) {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return
	}
	_ = proc.cmd.Process.Kill()
	for _, logFile := range proc.logs {
		_ = logFile.Close()
	}
	_ = proc.cmd.Wait()
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
