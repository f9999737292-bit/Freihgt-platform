//go:build integration

package analytics

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

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	fcconfig "github.com/freight-platform/freight-cost-service/internal/config"
	"github.com/freight-platform/freight-cost-service/internal/http/handlers"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

const (
	browserE2EJWTSecret     = "fc22g1-browser-e2e-jwt-secret"
	browserE2EInternalToken = "fc22g1-browser-e2e-internal-token"
)

type browserFixture struct {
	TenantID        uuid.UUID
	BuyerID         uuid.UUID
	UserID          uuid.UUID
	JWT             string
	ExpectedPlanned string
	ExpectedDelta   string
}

type browserLiveStack struct {
	webURL         string
	gatewayURL     string
	fixture        browserFixture
	freightCostSrv *http.Server
	gatewaySrv     *http.Server
	webCmd         *exec.Cmd
}

func TestFC22G1_BrowserE2E_LiveBuyerFlow(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when BROWSER_E2E=1")
	}
	stack := startBrowserLiveStack(t)
	t.Cleanup(stack.shutdown)
	if err := runPlaywrightSuite(t, stack.webURL, "FC22G1-UI-00[1-7]", stack.fixture); err != nil {
		t.Fatalf("playwright buyer suite: %v", err)
	}
	flagOffURL, flagOffCmd := startBrowserWebProcurement(t, stack.gatewayURL, stack.fixture, "3011", false)
	t.Cleanup(func() {
		if flagOffCmd != nil && flagOffCmd.Process != nil {
			_ = flagOffCmd.Process.Kill()
		}
	})
	waitForHTTP200(t, flagOffURL+"/login", 120*time.Second)
	if err := runPlaywrightSuite(t, flagOffURL, "FC22G1-UI-008", stack.fixture); err != nil {
		t.Fatalf("playwright feature-flag-off suite: %v", err)
	}
}

func (s *browserLiveStack) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if s.webCmd != nil && s.webCmd.Process != nil {
		_ = s.webCmd.Process.Kill()
	}
	if s.gatewaySrv != nil {
		_ = s.gatewaySrv.Shutdown(ctx)
	}
	if s.freightCostSrv != nil {
		_ = s.freightCostSrv.Shutdown(ctx)
	}
}

func startBrowserLiveStack(t *testing.T) *browserLiveStack {
	t.Helper()
	env := setupFullProjectionEnv(t)
	fix := seedBrowserE2EFixture(t, env)
	fcURL, fcServer := startBrowserFreightCostServer(t, env)
	gwURL, gwServer := startBrowserGatewayProxy(t, fcURL, fix)
	webURL, webCmd := startBrowserWebProcurement(t, gwURL, fix, "3010", true)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserLiveStack{
		webURL:         webURL,
		gatewayURL:     gwURL,
		fixture:        fix,
		freightCostSrv: fcServer,
		gatewaySrv:     gwServer,
		webCmd:         webCmd,
	}
}

func seedBrowserE2EFixture(t *testing.T, env *fullProjectionEnv) browserFixture {
	t.Helper()
	base := seedFullProjectionFixture(t, env)
	userID := uuid.MustParse("8541a3a3-bde7-4fed-9501-37b9953bf904")
	return browserFixture{
		TenantID:        base.tenantID,
		BuyerID:         base.buyerID,
		UserID:          userID,
		JWT:             browserJWT(userID, base.tenantID),
		ExpectedPlanned: "190000.00",
		ExpectedDelta:   "7000.00",
	}
}

func browserJWT(userID, tenantID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"email":     "buyer-e2e@freight.test",
		"sub":       userID.String(),
		"exp":       time.Now().Add(2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(browserE2EJWTSecret))
	return signed
}

func startBrowserFreightCostServer(t *testing.T, env *fullProjectionEnv) (string, *http.Server) {
	t.Helper()
	orderFacts := repository.NewAnalyticsOrderFactRepository(env.pool)
	state := repository.NewAnalyticsProjectionStateRepository(env.pool)
	publicSvc := service.NewAnalyticsPublicService(env.analytics, orderFacts, state, true)
	publicHandler := handlers.NewAnalyticsPublicHandler(publicSvc)

	r := chi.NewRouter()
	internalAuth := internalauth.Config{Token: browserE2EInternalToken, Environment: "test"}
	r.Route("/internal/v1/freight-costs", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Get("/analytics/overview", publicHandler.Overview)
		r.Get("/analytics/lanes", publicHandler.ListLanes)
		r.Get("/analytics/carriers", publicHandler.ListCarriers)
		r.Get("/analytics/accessorials", publicHandler.ListAccessorials)
		r.Get("/opportunities", publicHandler.ListOpportunities)
	})
	_ = fcconfig.Config{}
	return listenHTTPServer(t, r)
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

func startBrowserWebProcurement(t *testing.T, gatewayURL string, fix browserFixture, port string, workspaceEnabled bool) (string, *exec.Cmd) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	cmd := exec.Command("pnpm", "--filter", "@freight-platform/web-procurement", "exec", "nuxt", "dev", "--port", port, "--host", "127.0.0.1")
	cmd.Dir = root
	env := []string{
		"NUXT_PUBLIC_API_BASE_URL=http://127.0.0.1:" + port,
		"NUXT_E2E_GATEWAY_URL=" + gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID=" + fix.TenantID.String(),
	}
	if workspaceEnabled {
		env = append(env, "NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=true")
	} else {
		env = append(env, "NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=false")
	}
	env = append(env, "NUXT_E2E_DISABLE_SSR=true")
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start web dev: %v", err)
	}
	return "http://127.0.0.1:" + port, cmd
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

func webProcurementRoot(t *testing.T) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	return filepath.Join(root, "apps", "web-procurement")
}

func runPlaywrightSuite(t *testing.T, webURL, grep string, fix browserFixture) error {
	t.Helper()
	root := webProcurementRoot(t)
	e2eDir := filepath.Join(root, "e2e", "freight-cost-intelligence")
	args := []string{"playwright", "test"}
	if grep != "" {
		args = append(args, "--grep", grep)
	}
	cmd := exec.Command("npx", args...)
	cmd.Dir = e2eDir
	cmd.Env = append(os.Environ(),
		"BROWSER_E2E_WEB_URL="+webURL,
		"BROWSER_E2E_JWT="+fix.JWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.BuyerID.String(),
		"BROWSER_E2E_EXPECTED_PLANNED="+fix.ExpectedPlanned,
		"BROWSER_E2E_EXPECTED_DELTA="+fix.ExpectedDelta,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
