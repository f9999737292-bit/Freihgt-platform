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
	httpserver "github.com/freight-platform/rfx-service/internal/http"
)

const humanProvenanceBrowserPort = "3034"

type browserHumanProvenanceFixture struct {
	TenantID            string
	BuyerCompanyID      string
	OtherBuyerCompanyID string
	CarrierCompanyID    string
	BuyerUserID         string
	OtherBuyerUserID    string
	CarrierUserID       string
	EventID             string
	RfxNumber           string
	BuyerJWT            string
	OtherBuyerJWT       string
	CarrierJWT          string
}

type browserHumanProvenanceLiveStack struct {
	webURL      string
	gatewayURL  string
	fixture     browserHumanProvenanceFixture
	rfxSrv      *http.Server
	gatewayProc *browserGatewayProcess
	webCmd      *webProcurementCmd
}

func TestRfxHumanProvenance_BrowserE2E_LiveCreationChannel(t *testing.T) {
	requireHumanProvenanceBrowserGate(t)
	t.Cleanup(func() { verifyBrowserHarnessCleanup(t) })
	stack := startHumanProvenanceLiveStack(t)
	t.Cleanup(func() { stack.shutdown(t) })
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyHumanProvenanceGatewayProbe(t, stack)
	if err := runHumanProvenancePlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright human provenance suite: %v", err)
	}
}

func requireHumanProvenanceBrowserGate(t *testing.T) {
	t.Helper()
	if os.Getenv("BROWSER_E2E") != "1" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("BROWSER_E2E=1 is required in CI for the human provenance browser gate")
		}
		t.Fatal("BROWSER_E2E=1 is required for the human provenance browser gate")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required for the human provenance browser gate")
	}
}

func startHumanProvenanceLiveStack(t *testing.T) *browserHumanProvenanceLiveStack {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedHumanProvenanceBrowserFixture(t, env)
	rfxURL, rfxSrv := startHumanProvenanceRfxService(t, env)
	identity := startBrowserIdentityStub(t, map[string][]string{
		fix.BuyerUserID:         {"PROCUREMENT_MANAGER"},
		fix.OtherBuyerUserID:    {"PROCUREMENT_MANAGER"},
		fix.CarrierUserID:       {"CARRIER_DISPATCHER"},
	})
	webOrigin := browserGatewayEnvForStack(t, humanProvenanceBrowserPort)
	corsOrigins := webOrigin + ",http://localhost:" + humanProvenanceBrowserPort
	gatewayURL, gatewayProc := startBuyerXlsxProductionGateway(t, rfxURL, corsOrigins, identity)
	webURL, webCmd := startHumanProvenanceWebProcurement(t, gatewayURL, fix, humanProvenanceBrowserPort)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserHumanProvenanceLiveStack{
		webURL:      webURL,
		gatewayURL:  gatewayURL,
		fixture:     fix,
		rfxSrv:      rfxSrv,
		gatewayProc: gatewayProc,
		webCmd:      webCmd,
	}
}

func seedHumanProvenanceBrowserFixture(t *testing.T, env *testEnv) browserHumanProvenanceFixture {
	t.Helper()
	seed := seedBuyerFixture(t, env)
	event := createDraftEvent(t, env, seed, "RFX-F4-BRW-1")
	return browserHumanProvenanceFixture{
		TenantID:            seed.TenantID.String(),
		BuyerCompanyID:      seed.CompanyA.String(),
		OtherBuyerCompanyID: seed.CompanyB.String(),
		CarrierCompanyID:    seed.CarrierID.String(),
		BuyerUserID:         seed.BuyerA.UserID.String(),
		OtherBuyerUserID:    seed.BuyerB.UserID.String(),
		CarrierUserID:       seed.CarrierAct.UserID.String(),
		EventID:             event.ID.String(),
		RfxNumber:           event.RfxNumber,
		BuyerJWT:            browserStudioJWT(seed.BuyerA.UserID, seed.TenantID),
		OtherBuyerJWT:       browserStudioJWT(seed.BuyerB.UserID, seed.TenantID),
		CarrierJWT:          browserStudioJWT(seed.CarrierAct.UserID, seed.TenantID),
	}
}

func startHumanProvenanceRfxService(t *testing.T, env *testEnv) (string, *http.Server) {
	t.Helper()
	cfg := config.Config{Environment: "test"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpserver.NewRouter(
		log, env.pool, cfg,
		env.rfxSvc, env.qSvc, env.versionSvc,
		nil, nil, nil,
		env.crSvc, nil, nil, nil,
		env.scoreModelSvc, env.scoringSvc,
		nil, nil, nil,
	)
	return listenHTTPServer(t, handler)
}

func startHumanProvenanceWebProcurement(t *testing.T, gatewayURL string, fix browserHumanProvenanceFixture, port string) (string, *webProcurementCmd) {
	t.Helper()
	webURL := "http://127.0.0.1:" + port
	env := overrideProcessEnv(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID,
		"NUXT_E2E_DISABLE_SSR=true",
		"NUXT_E2E_DISABLE_DEVTOOLS=true",
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
	return webURL, proc
}

func (s *browserHumanProvenanceLiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.webCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
	verifyDevPortsReleased(t, humanProvenanceBrowserPort)
}

func verifyHumanProvenanceGatewayProbe(t *testing.T, stack *browserHumanProvenanceLiveStack) {
	t.Helper()
	eventURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.EventID
	req, err := http.NewRequest(http.MethodGet, eventURL, nil)
	if err != nil {
		t.Fatalf("probe event request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.BuyerJWT)
	req.Header.Set("X-Company-ID", stack.fixture.BuyerCompanyID)
	req.Header.Set("Origin", stack.webURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe human GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe human GET: status=%d url=%s", resp.StatusCode, eventURL)
	}
}

func runHumanProvenancePlaywrightSuite(t *testing.T, stack *browserHumanProvenanceLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "human-provenance")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"CI=true",
		"BROWSER_E2E=1",
		"BROWSER_E2E_REQUIRE=1",
		"BROWSER_E2E_WEB_URL="+stack.webURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_JWT="+fix.BuyerJWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID,
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.BuyerCompanyID,
		"BROWSER_E2E_EVENT_ID="+fix.EventID,
		"BROWSER_E2E_RFX_NUMBER="+fix.RfxNumber,
		"BROWSER_E2E_USER_ID="+fix.BuyerUserID,
		"BROWSER_E2E_CARRIER_JWT="+fix.CarrierJWT,
		"BROWSER_E2E_CARRIER_USER_ID="+fix.CarrierUserID,
		"BROWSER_E2E_CARRIER_COMPANY_ID="+fix.CarrierCompanyID,
		"BROWSER_E2E_OTHER_BUYER_JWT="+fix.OtherBuyerJWT,
		"BROWSER_E2E_OTHER_BUYER_USER_ID="+fix.OtherBuyerUserID,
		"BROWSER_E2E_OTHER_BUYER_COMPANY_ID="+fix.OtherBuyerCompanyID,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
