//go:build integration

package carrierresponse

import (
	"context"
	"encoding/json"
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

	"github.com/freight-platform/rfx-service/internal/domain"
)

const browserCarrierE2EJWTSecret = "rfx-carrier-browser-e2e-jwt-secret"

type browserCarrierFixture struct {
	TenantID       uuid.UUID
	BuyerCompanyID uuid.UUID
	CarrierID      uuid.UUID
	CarrierUserID  uuid.UUID
	EventID        uuid.UUID
	JWT            string
	RfxNumber      string
}

type browserLiveStack struct {
	webURL      string
	gatewayURL  string
	fixture     browserCarrierFixture
	rfxSrv      *http.Server
	gatewayProc *browserGatewayProcess
	webCmd      *webProcurementCmd
}

type webProcurementCmd struct {
	cmd  *exec.Cmd
	logs []*os.File
}

func TestRfxCarrierResponse_BrowserE2E_LiveCarrierFlow(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required when BROWSER_E2E=1")
	}
	stack := startCarrierBrowserLiveStack(t)
	t.Cleanup(stack.shutdown)
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyCarrierGatewayProbe(t, stack)
	if err := runCarrierPlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright carrier response suite: %v", err)
	}
}

func verifyCarrierGatewayProbe(t *testing.T, stack *browserLiveStack) {
	t.Helper()
	probeURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.EventID.String() + "/carrier-response"
	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		t.Fatalf("probe request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.JWT)
	req.Header.Set("X-Company-ID", stack.fixture.CarrierID.String())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe carrier gateway: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("probe carrier gateway: status=%d url=%s", resp.StatusCode, probeURL)
	}
}

func (s *browserLiveStack) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(s.webCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
}

func startCarrierBrowserLiveStack(t *testing.T) *browserLiveStack {
	t.Helper()
	const webPort = "3005"
	env := setupTestEnv(t)
	fix := seedBrowserHSEFixture(t, env)
	rfxURL, rfxSrv := startBrowserCarrierRfxService(t, env)
	identity := startBrowserIdentityStub(t, browserIdentityRolesForCarrier(fix.CarrierUserID.String()))
	webOrigin := browserGatewayEnvForStack(t, webPort)
	gatewayURL, gatewayProc := startBrowserProductionGateway(t, rfxURL, webOrigin, identity)
	webURL, webCmd := startBrowserWebProcurement(t, gatewayURL, fix, webPort)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserLiveStack{
		webURL:      webURL,
		gatewayURL:  gatewayURL,
		fixture:     fix,
		rfxSrv:      rfxSrv,
		gatewayProc: gatewayProc,
		webCmd:      webCmd,
	}
}

func seedBrowserHSEFixture(t *testing.T, env *testEnv) browserCarrierFixture {
	t.Helper()
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Carrier Browser HSE",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-CR-BRW-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "HSE", Title: "HSE",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_AVAILABLE", QuestionType: domain.QuestionTypeYesNo, Label: "ADR available?",
	}); err != nil {
		t.Fatalf("create ADR_AVAILABLE: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_NUMBER", QuestionType: domain.QuestionTypeText, Label: "ADR number",
	}); err != nil {
		t.Fatalf("create ADR_NUMBER: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "ADR_EXPIRY", QuestionType: domain.QuestionTypeDate, Label: "ADR expiry",
	}); err != nil {
		t.Fatalf("create ADR_EXPIRY: %v", err)
	}
	if _, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "FLEET_COUNT", QuestionType: domain.QuestionTypeNumber, Label: "Fleet count",
		ValidationRuleJSON: json.RawMessage(`{"min_value":0}`),
	}); err != nil {
		t.Fatalf("create FLEET_COUNT: %v", err)
	}
	targetNumber := "ADR_NUMBER"
	targetExpiry := "ADR_EXPIRY"
	if _, err := env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode: "REQ_ADR_NUMBER", Action: domain.RuleActionRequire, TargetQuestionCode: &targetNumber,
		ConditionJSON: conditionEquals("ADR_AVAILABLE", true),
	}); err != nil {
		t.Fatalf("require ADR_NUMBER: %v", err)
	}
	if _, err := env.qSvc.CreateRule(ctx, fix.BuyerA, event.ID, domain.CreateQuestionRuleInput{
		RuleCode: "REQ_ADR_EXPIRY", Action: domain.RuleActionRequire, TargetQuestionCode: &targetExpiry,
		ConditionJSON: conditionEquals("ADR_AVAILABLE", true),
	}); err != nil {
		t.Fatalf("require ADR_EXPIRY: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	return browserCarrierFixture{
		TenantID:       fix.TenantID,
		BuyerCompanyID: fix.CompanyA,
		CarrierID:      fix.CarrierID,
		CarrierUserID:  fix.CarrierAct.UserID,
		EventID:        event.ID,
		JWT:            browserCarrierJWT(fix.CarrierAct.UserID, fix.TenantID),
		RfxNumber:      event.RfxNumber,
	}
}

func browserCarrierJWT(userID, tenantID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"email":     "carrier-response-e2e@freight.test",
		"sub":       userID.String(),
		"exp":       time.Now().Add(2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(browserCarrierE2EJWTSecret))
	return signed
}

func startBrowserCarrierRfxService(t *testing.T, env *testEnv) (string, *http.Server) {
	t.Helper()
	return listenHTTPServer(t, newBrowserCarrierRouter(env))
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

func startBrowserWebProcurement(t *testing.T, gatewayURL string, fix browserCarrierFixture, port string) (string, *webProcurementCmd) {
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
	logFile, err := os.CreateTemp("", "rfx-carrier-nuxt-"+port+"-*.log")
	if err != nil {
		t.Fatalf("create nuxt log file: %v", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		t.Fatalf("start web-procurement dev: %v", err)
	}
	return "http://127.0.0.1:" + port, &webProcurementCmd{cmd: cmd, logs: []*os.File{logFile}}
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

func runCarrierPlaywrightSuite(t *testing.T, stack *browserLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "carrier-response")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"BROWSER_E2E_WEB_URL="+stack.webURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_JWT="+fix.JWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_CARRIER_COMPANY_ID="+fix.CarrierID.String(),
		"BROWSER_E2E_EVENT_ID="+fix.EventID.String(),
		"BROWSER_E2E_RFX_NUMBER="+fix.RfxNumber,
		"BROWSER_E2E_USER_ID="+fix.CarrierUserID.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
