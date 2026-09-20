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

const lateSubmissionBrowserPort = "3033"

type browserLateDraft struct {
	EventID    uuid.UUID
	ResponseID uuid.UUID
	RfxNumber  string
}

type browserLateSubmissionFixture struct {
	TenantID         uuid.UUID
	BuyerCompanyID   uuid.UUID
	CarrierCompanyID uuid.UUID
	CarrierUserID    uuid.UUID
	BuyerUserID      uuid.UUID
	LogistUserID     uuid.UUID
	CarrierJWT       string
	BuyerJWT         string
	LogistJWT        string
	Create           browserLateDraft
	Submit           browserLateDraft
	Reject           browserLateDraft
	NotStarted       browserLateDraft
	Expired          browserLateDraft
}

type browserLateSubmissionLiveStack struct {
	webURL      string
	gatewayURL  string
	fixture     browserLateSubmissionFixture
	rfxSrv      *http.Server
	gatewayProc *browserGatewayProcess
	webCmd      *webProcurementCmd
}

func TestRfxLateSubmission_BrowserE2E_LiveDraftWindow(t *testing.T) {
	requireLateSubmissionBrowserGate(t)
	t.Cleanup(func() { verifyBrowserHarnessCleanup(t) })
	stack := startLateSubmissionLiveStack(t)
	t.Cleanup(func() { stack.shutdown(t) })
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyLateSubmissionGatewayProbe(t, stack)
	if err := runLateSubmissionPlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright late submission suite: %v", err)
	}
}

func requireLateSubmissionBrowserGate(t *testing.T) {
	t.Helper()
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Fatal("BROWSER_E2E=1 is required for the late submission browser gate")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required for the late submission browser gate")
	}
	if os.Getenv("BROWSER_E2E_LATE_FLAG") != "1" {
		t.Fatal("BROWSER_E2E_LATE_FLAG=1 is required for the late submission browser gate")
	}
}

func startLateSubmissionLiveStack(t *testing.T) *browserLateSubmissionLiveStack {
	t.Helper()
	env := setupTestEnv(t)
	lateRepo := repository.NewLateSubmissionRepository(env.pool)
	lateSvc := service.NewLateSubmissionService(env.pool, lateRepo, env.rfxRepo, env.idemRepo, env.auditRepo, env.rfxSvc)
	crSvc := service.NewCarrierResponseServiceWithLateSubmission(
		env.pool, env.rfxRepo, env.answerRepo, env.qRepo, env.auditRepo, env.membershipRepo, env.rfxSvc, env.scoringSvc, lateSvc, env.idemRepo,
	)
	env.crSvc = crSvc
	fix := seedLateSubmissionBrowserFixture(t, env, lateSvc)
	rfxURL, rfxSrv := startLateSubmissionRfxService(t, env, lateSvc, crSvc)
	identity := startBrowserIdentityStub(t, map[string][]string{
		fix.CarrierUserID.String(): {"CARRIER_DISPATCHER"},
		fix.BuyerUserID.String():   {"PROCUREMENT_MANAGER"},
		fix.LogistUserID.String():  {"SHIPPER_LOGIST"},
	})
	companyStub := startLateSubmissionCompanyStub(t, fix)
	webOrigin := browserGatewayEnvForStack(t, lateSubmissionBrowserPort)
	corsOrigins := webOrigin + ",http://localhost:" + lateSubmissionBrowserPort
	gatewayURL, gatewayProc := startLateSubmissionProductionGateway(t, rfxURL, corsOrigins, identity, companyStub.URL())
	webURL, webCmd := startLateSubmissionWebProcurement(t, gatewayURL, fix, lateSubmissionBrowserPort)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserLateSubmissionLiveStack{
		webURL:      webURL,
		gatewayURL:  gatewayURL,
		fixture:     fix,
		rfxSrv:      rfxSrv,
		gatewayProc: gatewayProc,
		webCmd:      webCmd,
	}
}

func seedLateSubmissionBrowserFixture(t *testing.T, env *testEnv, lateSvc *service.LateSubmissionService) browserLateSubmissionFixture {
	t.Helper()
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	logistID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.users (id, tenant_id, email, full_name) VALUES ($1, $2, $3, $4)`,
		logistID, fix.TenantID, "buyer-logist@test.local", "Buyer Logist"); err != nil {
		t.Fatalf("seed logist user: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id) VALUES ($1, $2, $3)`,
		fix.TenantID, fix.CompanyA, logistID); err != nil {
		t.Fatalf("seed logist membership: %v", err)
	}
	var logistRoleID uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_LOGIST' LIMIT 1`).Scan(&logistRoleID); err != nil {
		t.Fatalf("lookup SHIPPER_LOGIST: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id) VALUES ($1, $2, $3, $4)`,
		fix.TenantID, logistID, fix.CompanyA, logistRoleID); err != nil {
		t.Fatalf("seed logist role: %v", err)
	}

	create := seedLateDraftEvent(t, env, lateSvc, fix, "create", false, "")
	submit := seedLateDraftEvent(t, env, lateSvc, fix, "submit", true, "requested")
	reject := seedLateDraftEvent(t, env, lateSvc, fix, "reject", true, "requested")
	notStarted := seedLateDraftEvent(t, env, lateSvc, fix, "window-open", true, "not_started")
	expired := seedLateDraftEvent(t, env, lateSvc, fix, "window-closed", true, "expired")
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
		Create:           create,
		Submit:           submit,
		Reject:           reject,
		NotStarted:       notStarted,
		Expired:          expired,
	}
}

func seedLateDraftEvent(
	t *testing.T,
	env *testEnv,
	lateSvc *service.LateSubmissionService,
	fix buyerFixture,
	suffix string,
	createRequest bool,
	windowKind string,
) browserLateDraft {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(48 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID:         fix.TenantID,
		OwnerCompanyID:   fix.CompanyA,
		Title:            "Late submission " + suffix,
		RfxType:          "SPOT_RFQ",
		Category:         "FREIGHT",
		RfxNumber:        "RFX-LATE-" + suffix + "-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline,
	})
	if err != nil {
		t.Fatalf("create event %s: %v", suffix, err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant %s: %v", suffix, err)
	}
	version := enableQuestionnaire(t, env, fix.BuyerA, event.ID)
	sec, err := env.qSvc.CreateSection(ctx, fix.BuyerA, event.ID, domain.CreateSectionInput{
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("create section %s: %v", suffix, err)
	}
	question, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes",
	})
	if err != nil {
		t.Fatalf("create question %s: %v", suffix, err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event %s: %v", suffix, err)
	}
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start response %s: %v", suffix, err)
	}
	if _, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: question.ID, Value: json.RawMessage(`"YES"`)}},
	}); err != nil {
		t.Fatalf("save answers %s: %v", suffix, err)
	}
	past := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := env.pool.Exec(ctx, `UPDATE rfx.rfx_events SET response_deadline = $1 WHERE id = $2`, past, event.ID); err != nil {
		t.Fatalf("expire deadline %s: %v", suffix, err)
	}
	if createRequest {
		created, err := lateSvc.CreateRequest(ctx, fix.CarrierAct, event.ID, fix.CarrierID, "late-seed-create:"+event.ID.String(), domain.CreateLateSubmissionRequestInput{
			ReasonCode:     domain.LateSubmissionReasonTechnicalFailure,
			ReasonText:     "seeded late request for " + suffix,
			RequestedUntil: time.Now().UTC().Add(24 * time.Hour),
		})
		if err != nil {
			t.Fatalf("seed late request %s: %v", suffix, err)
		}
		switch windowKind {
		case "not_started":
			if _, err := lateSvc.Approve(ctx, fix.BuyerA, event.ID, created.ID, "late-seed-approve:"+event.ID.String(), domain.ApproveLateSubmissionInput{
				ExpectedVersion:    created.Version,
				ApprovedValidFrom:  time.Now().UTC().Add(2 * time.Hour),
				ApprovedValidUntil: time.Now().UTC().Add(26 * time.Hour),
			}); err != nil {
				t.Fatalf("approve future window %s: %v", suffix, err)
			}
		case "expired":
			approved, err := lateSvc.Approve(ctx, fix.BuyerA, event.ID, created.ID, "late-seed-approve:"+event.ID.String(), domain.ApproveLateSubmissionInput{
				ExpectedVersion:    created.Version,
				ApprovedValidFrom:  time.Now().UTC().Add(-2 * time.Hour),
				ApprovedValidUntil: time.Now().UTC().Add(2 * time.Hour),
			})
			if err != nil {
				t.Fatalf("approve then expire %s: %v", suffix, err)
			}
			from := time.Now().UTC().Add(-3 * time.Hour)
			until := time.Now().UTC().Add(-1 * time.Hour)
			if _, err := env.pool.Exec(ctx, `
				UPDATE rfx.rfx_late_submission_requests
				SET approved_valid_from = $1, approved_valid_until = $2, updated_at = now()
				WHERE id = $3`, from, until, approved.ID); err != nil {
				t.Fatalf("expire approved window %s: %v", suffix, err)
			}
		}
	}
	return browserLateDraft{EventID: event.ID, ResponseID: ws.Response.ID, RfxNumber: event.RfxNumber}
}

func startLateSubmissionRfxService(t *testing.T, env *testEnv, lateSvc *service.LateSubmissionService, crSvc *service.CarrierResponseService) (string, *http.Server) {
	t.Helper()
	cfg := config.Config{RfxLateSubmissionEnabled: true, Environment: "test"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpserver.NewRouter(
		log, env.pool, cfg,
		env.rfxSvc, env.qSvc, env.versionSvc,
		nil, nil, nil,
		crSvc, lateSvc, nil, nil,
		env.scoreModelSvc, env.scoringSvc,
		nil, nil, nil,
	)
	return listenHTTPServer(t, handler)
}

type browserLateCompanyStub struct {
	server *httptest.Server
}

func startLateSubmissionCompanyStub(t *testing.T, fix browserLateSubmissionFixture) *browserLateCompanyStub {
	t.Helper()
	stub := &browserLateCompanyStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/v1/companies" || strings.HasSuffix(r.URL.Path, "/companies")):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id": fix.BuyerCompanyID.String(), "tenant_id": fix.TenantID.String(),
						"legal_name": "Buyer A", "company_type": "SHIPPER", "country_code": "RU",
						"preferred_locale": "en-US", "status": "ACTIVE",
					},
					{
						"id": fix.CarrierCompanyID.String(), "tenant_id": fix.TenantID.String(),
						"legal_name": "Carrier A", "company_type": "CARRIER", "country_code": "RU",
						"preferred_locale": "en-US", "status": "ACTIVE",
					},
				},
				"total": 2, "limit": 200, "offset": 0,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *browserLateCompanyStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func startLateSubmissionProductionGateway(t *testing.T, rfxServiceURL, origins string, identity *browserIdentityStub, companyURL string) (string, *browserGatewayProcess) {
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
		"RFX_LATE_SUBMISSION_ENABLED=true",
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	verifyBrowserGatewayRfxRoute(t, gatewayURL, rfxServiceURL)
	return gatewayURL, proc
}

func startLateSubmissionWebProcurement(t *testing.T, gatewayURL string, fix browserLateSubmissionFixture, port string) (string, *webProcurementCmd) {
	t.Helper()
	webURL := "http://127.0.0.1:" + port
	env := overrideProcessEnv(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_PUBLIC_RFX_LATE_SUBMISSION_ENABLED=true",
		"RFX_LATE_SUBMISSION_UI=1",
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

func (s *browserLateSubmissionLiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.webCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
	verifyDevPortsReleased(t, lateSubmissionBrowserPort)
}

func verifyLateSubmissionGatewayProbe(t *testing.T, stack *browserLateSubmissionLiveStack) {
	t.Helper()
	eventURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.Create.EventID.String()
	req, err := http.NewRequest(http.MethodGet, eventURL+"/late-submission-requests/mine?carrier_company_id="+stack.fixture.CarrierCompanyID.String(), nil)
	if err != nil {
		t.Fatalf("probe mine request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.CarrierJWT)
	req.Header.Set("X-Company-ID", stack.fixture.CarrierCompanyID.String())
	req.Header.Set("Origin", stack.webURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe late mine: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe late mine: status=%d url=%s", resp.StatusCode, req.URL.String())
	}
}

func runLateSubmissionPlaywrightSuite(t *testing.T, stack *browserLateSubmissionLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "late-submission")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath)
	cmd.Dir = e2eDir
	fix := stack.fixture
	cmd.Env = append(os.Environ(),
		"CI=true",
		"BROWSER_E2E=1",
		"BROWSER_E2E_REQUIRE=1",
		"BROWSER_E2E_LATE_FLAG=1",
		"BROWSER_E2E_WEB_URL="+stack.webURL,
		"BROWSER_E2E_GATEWAY_URL="+stack.gatewayURL,
		"BROWSER_E2E_JWT="+fix.CarrierJWT,
		"BROWSER_E2E_BUYER_JWT="+fix.BuyerJWT,
		"BROWSER_E2E_LOGIST_JWT="+fix.LogistJWT,
		"BROWSER_E2E_TENANT_ID="+fix.TenantID.String(),
		"BROWSER_E2E_CARRIER_COMPANY_ID="+fix.CarrierCompanyID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.BuyerCompanyID.String(),
		"BROWSER_E2E_USER_ID="+fix.CarrierUserID.String(),
		"BROWSER_E2E_BUYER_USER_ID="+fix.BuyerUserID.String(),
		"BROWSER_E2E_LOGIST_USER_ID="+fix.LogistUserID.String(),
		"BROWSER_E2E_EVENT_ID="+fix.Create.EventID.String(),
		"BROWSER_E2E_RESPONSE_ID="+fix.Create.ResponseID.String(),
		"BROWSER_E2E_RFX_NUMBER="+fix.Create.RfxNumber,
		"BROWSER_E2E_SUBMIT_EVENT_ID="+fix.Submit.EventID.String(),
		"BROWSER_E2E_SUBMIT_RFX_NUMBER="+fix.Submit.RfxNumber,
		"BROWSER_E2E_REJECT_EVENT_ID="+fix.Reject.EventID.String(),
		"BROWSER_E2E_REJECT_RFX_NUMBER="+fix.Reject.RfxNumber,
		"BROWSER_E2E_WINDOW_NOT_STARTED_EVENT_ID="+fix.NotStarted.EventID.String(),
		"BROWSER_E2E_WINDOW_NOT_STARTED_RFX_NUMBER="+fix.NotStarted.RfxNumber,
		"BROWSER_E2E_WINDOW_EXPIRED_EVENT_ID="+fix.Expired.EventID.String(),
		"BROWSER_E2E_WINDOW_EXPIRED_RFX_NUMBER="+fix.Expired.RfxNumber,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
