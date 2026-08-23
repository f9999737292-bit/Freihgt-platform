//go:build integration

package analytics

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	fixture        browserFixture
	freightCostSrv *http.Server
	gatewaySrv     *http.Server
	webCmd         *exec.Cmd
}

func TestFC22G1_BrowserE2E_LiveBuyerFlow(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	stack := startBrowserLiveStack(t)
	t.Cleanup(stack.shutdown)
	if err := runPlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright suite: %v", err)
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
	webURL, webCmd := startBrowserWebProcurement(t, gwURL, fix)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserLiveStack{
		webURL:         webURL,
		fixture:        fix,
		freightCostSrv: fcServer,
		gatewaySrv:     gwServer,
		webCmd:         webCmd,
	}
}

func seedBrowserE2EFixture(t *testing.T, env *fullProjectionEnv) browserFixture {
	t.Helper()
	base := seedFullProjectionFixture(t, env)
	userID := uuid.New()
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

func startBrowserWebProcurement(t *testing.T, gatewayURL string, fix browserFixture) (string, *exec.Cmd) {
	t.Helper()
	root := webProcurementRoot(t)
	port := "3010"
	cmd := exec.Command("npm", "run", "dev", "--", "--port", port, "--host", "127.0.0.1")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED=true",
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
	)
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
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller failed")
	}
	dir := filepath.Dir(file)
	for {
		candidate := filepath.Join(dir, "apps", "web-procurement")
		if _, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("web-procurement root not found")
		}
		dir = parent
	}
}

func runPlaywrightSuite(t *testing.T, stack *browserLiveStack) error {
	t.Helper()
	root := webProcurementRoot(t)
	e2eDir := filepath.Join(root, "e2e", "freight-cost-intelligence")
	cmd := exec.Command("npx", "playwright", "test")
	cmd.Dir = e2eDir
	cmd.Env = append(os.Environ(),
		"BROWSER_E2E_WEB_URL="+stack.webURL,
		"BROWSER_E2E_JWT="+stack.fixture.JWT,
		"BROWSER_E2E_TENANT_ID="+stack.fixture.TenantID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+stack.fixture.BuyerID.String(),
		"BROWSER_E2E_EXPECTED_PLANNED="+stack.fixture.ExpectedPlanned,
		"BROWSER_E2E_EXPECTED_DELTA="+stack.fixture.ExpectedDelta,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
