//go:build integration

package studio

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const browserE2EJWTSecret = "rfx-studio-browser-e2e-jwt-secret"

type browserStudioFixture struct {
	TenantID  uuid.UUID
	CompanyID uuid.UUID
	UserID    uuid.UUID
	EventID   uuid.UUID
	JWT       string
	RfxNumber string
}

type browserLiveStack struct {
	webURL     string
	gatewayURL string
	fixture    browserStudioFixture
	rfxSrv     *http.Server
	gatewaySrv *http.Server
	webCmd     *webAdminCmd
}

type webAdminCmd struct {
	cmd  *exec.Cmd
	logs []*os.File
}

func TestRfxStudio_BrowserE2E_LiveBuyerFlow(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when BROWSER_E2E=1")
	}
	stack := startBrowserLiveStack(t)
	t.Cleanup(stack.shutdown)
	verifyStudioGatewayProbe(t, stack)
	if err := runPlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright rfx studio suite: %v", err)
	}
}

func verifyStudioGatewayProbe(t *testing.T, stack *browserLiveStack) {
	t.Helper()
	probeURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.EventID.String() + "/studio"
	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		t.Fatalf("probe request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.JWT)
	req.Header.Set("X-Tenant-ID", stack.fixture.TenantID.String())
	req.Header.Set("X-User-ID", stack.fixture.UserID.String())
	req.Header.Set("X-Company-ID", stack.fixture.CompanyID.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe studio gateway: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe studio gateway: status=%d url=%s", resp.StatusCode, probeURL)
	}
}

func (s *browserLiveStack) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebAdmin(s.webCmd)
	if s.gatewaySrv != nil {
		_ = s.gatewaySrv.Shutdown(ctx)
	}
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
}

func startBrowserLiveStack(t *testing.T) *browserLiveStack {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedBrowserStudioFixture(t, env)
	rfxURL, rfxSrv := startBrowserRfxService(t, env)
	gwURL, gwSrv := startBrowserGatewayProxy(t, rfxURL, fix)
	webURL, webCmd := startBrowserWebAdmin(t, gwURL, fix, "3020")
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserLiveStack{
		webURL:     webURL,
		gatewayURL: gwURL,
		fixture:    fix,
		rfxSrv:     rfxSrv,
		gatewaySrv: gwSrv,
		webCmd:     webCmd,
	}
}

func seedBrowserStudioFixture(t *testing.T, env *testEnv) browserStudioFixture {
	t.Helper()
	fix := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, fix, "RFX-BROWSER-E2E-1")
	enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	return browserStudioFixture{
		TenantID:  fix.TenantID,
		CompanyID: fix.CompanyA,
		UserID:    fix.BuyerA.UserID,
		EventID:   event.ID,
		JWT:       browserStudioJWT(fix.BuyerA.UserID, fix.TenantID),
		RfxNumber: event.RfxNumber,
	}
}

func browserStudioJWT(userID, tenantID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"email":     "buyer-studio-e2e@freight.test",
		"sub":       userID.String(),
		"exp":       time.Now().Add(2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(browserE2EJWTSecret))
	return signed
}

func startBrowserRfxService(t *testing.T, env *testEnv) (string, *http.Server) {
	t.Helper()
	return listenHTTPServer(t, newBrowserQuestionnaireRouter(env))
}

func listenHTTPServer(t *testing.T, handler http.Handler) (string, *http.Server) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return "http://" + ln.Addr().String(), srv
}

func startBrowserWebAdmin(t *testing.T, gatewayURL string, fix browserStudioFixture, port string) (string, *webAdminCmd) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	cmd := exec.Command("pnpm", "--filter", "@freight-platform/web-admin", "exec", "nuxt", "dev", "--port", port, "--host", "127.0.0.1")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL=http://127.0.0.1:"+port,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_E2E_DISABLE_SSR=true",
	)
	logFile, err := os.CreateTemp("", "rfx-studio-nuxt-"+port+"-*.log")
	if err != nil {
		t.Fatalf("create nuxt log file: %v", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.WaitDelay = 0
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start web-admin dev: %v", err)
	}
	return "http://127.0.0.1:" + port, &webAdminCmd{
		cmd:  cmd,
		logs: []*os.File{logFile},
	}
}

func stopBrowserWebAdmin(proc *webAdminCmd) {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return
	}
	proc.cmd.WaitDelay = 0
	_ = proc.cmd.Process.Kill()
	for _, logFile := range proc.logs {
		_ = logFile.Close()
	}
	_ = proc.cmd.Wait()
}

func waitForHTTP200(t *testing.T, targetURL string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(targetURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", targetURL)
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "infrastructure", "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func runPlaywrightSuite(t *testing.T, stack *browserLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "rfx-studio")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
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
