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

const carrierXlsxBrowserPort = "3032"

type browserCarrierXlsxFixture struct {
	TenantID         uuid.UUID
	BuyerCompanyID   uuid.UUID
	CarrierCompanyID uuid.UUID
	UserID           uuid.UUID
	EventID          uuid.UUID
	ResponseID       uuid.UUID
	LotID            uuid.UUID
	JWT              string
	RfxNumber        string
	CompetitorOffer      string
	CompetitorName       string
	CompetitorAnswer     string
	CompetitorCompanyID  uuid.UUID
	CompetitorResponseID uuid.UUID
}

type browserCarrierXlsxLiveStack struct {
	webURL      string
	gatewayURL  string
	fixture     browserCarrierXlsxFixture
	rfxSrv      *http.Server
	gatewayProc *browserGatewayProcess
	webCmd      *webProcurementCmd
}

type browserCarrierCompanyStub struct {
	server *httptest.Server
}

func TestRfxCarrierXlsx_BrowserE2E_LiveUpdateDraft(t *testing.T) {
	requireCarrierXlsxBrowserGate(t)
	t.Cleanup(func() { verifyBrowserHarnessCleanup(t) })
	stack := startCarrierXlsxLiveStack(t)
	t.Cleanup(func() { stack.shutdown(t) })
	t.Cleanup(func() {
		dumpGatewayLogsOnFailure(t, stack.gatewayProc)
		writeGatewayFailureArtifact(t, stack.gatewayProc)
	})
	verifyCarrierXlsxGatewayProbe(t, stack)
	if err := runCarrierXlsxPlaywrightSuite(t, stack); err != nil {
		t.Fatalf("playwright carrier XLSX suite: %v", err)
	}
}

func requireCarrierXlsxBrowserGate(t *testing.T) {
	t.Helper()
	if os.Getenv("BROWSER_E2E") != "1" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") {
			t.Fatal("BROWSER_E2E=1 is required in CI for the carrier XLSX browser gate")
		}
		t.Fatal("BROWSER_E2E=1 is required for the carrier XLSX browser gate")
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("TEST_DATABASE_URL is required for the carrier XLSX browser gate")
	}
	if os.Getenv("BROWSER_E2E_EXCEL_FLAG") != "1" {
		t.Fatal("BROWSER_E2E_EXCEL_FLAG=1 is required for the carrier XLSX browser gate")
	}
}

func startCarrierXlsxLiveStack(t *testing.T) *browserCarrierXlsxLiveStack {
	t.Helper()
	env := setupTestEnv(t)
	fix := seedCarrierXlsxBrowserFixture(t, env)
	rfxURL, rfxSrv := startCarrierXlsxRfxService(t, env)
	identity := startBrowserIdentityStub(t, browserIdentityRolesForCarrier(fix.UserID.String()))
	companyStub := startCarrierXlsxCompanyStub(t, fix)
	webOrigin := browserGatewayEnvForStack(t, carrierXlsxBrowserPort)
	corsOrigins := webOrigin + ",http://localhost:" + carrierXlsxBrowserPort
	gatewayURL, gatewayProc := startCarrierXlsxProductionGateway(t, rfxURL, corsOrigins, identity, companyStub.URL())
	webURL, webCmd := startCarrierXlsxWebProcurement(t, gatewayURL, fix, carrierXlsxBrowserPort)
	waitForHTTP200(t, webURL+"/login", 120*time.Second)
	return &browserCarrierXlsxLiveStack{
		webURL:      webURL,
		gatewayURL:  gatewayURL,
		fixture:     fix,
		rfxSrv:      rfxSrv,
		gatewayProc: gatewayProc,
		webCmd:      webCmd,
	}
}

func seedCarrierXlsxBrowserFixture(t *testing.T, env *testEnv) browserCarrierXlsxFixture {
	t.Helper()
	ctx := context.Background()
	fix := seedBuyerFixture(t, env)
	deadline := time.Now().UTC().Add(48 * time.Hour)
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID:         fix.TenantID,
		OwnerCompanyID:   fix.CompanyA,
		Title:            "Carrier XLSX live",
		RfxType:          "SPOT_RFQ",
		Category:         "FREIGHT",
		RfxNumber:        "RFX-XLSX-CR-" + uuid.NewString()[:8],
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
		SectionCode: "MAIN", Title: "Main Section",
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	question, err := env.qSvc.CreateQuestion(ctx, fix.BuyerA, event.ID, sec.ID, domain.CreateQuestionInput{
		QuestionCode: "NOTES", QuestionType: domain.QuestionTypeText, Label: "Notes",
	})
	if err != nil {
		t.Fatalf("create question: %v", err)
	}
	desc := "Lane bundle"
	category := "FREIGHT"
	lot, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "L1", Name: "Lane bundle",
		Description: &desc, Category: &category,
	})
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	publishVersion(t, env, version.ID)
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	ws, err := env.crSvc.StartOrResume(ctx, fix.CarrierAct, event.ID, fix.CarrierID)
	if err != nil {
		t.Fatalf("start carrier response: %v", err)
	}
	if _, err := env.crSvc.SaveAnswers(ctx, fix.CarrierAct, event.ID, fix.CarrierID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: ws.Response.SaveVersion,
		Answers:             []domain.AnswerPatchItem{{QuestionID: question.ID, Value: json.RawMessage(`"YES"`)}},
	}); err != nil {
		t.Fatalf("save answers: %v", err)
	}
	comment := "own-offer"
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, ws.Response.ID, []domain.UpsertOfferLineInput{{
		RfxLotID: lot.ID, Amount: 12345.67, CurrencyCode: "RUB", Comment: &comment,
	}}); err != nil {
		t.Fatalf("save offer line: %v", err)
	}
	response, err := env.rfxRepo.GetResponseByID(ctx, ws.Response.ID, fix.TenantID)
	if err != nil {
		t.Fatalf("reload response: %v", err)
	}
	competitor := seedCarrierXlsxCompetitor(t, env, fix, event.ID, lot.ID, question.ID)
	return browserCarrierXlsxFixture{
		TenantID:         fix.TenantID,
		BuyerCompanyID:   fix.CompanyA,
		CarrierCompanyID: fix.CarrierID,
		UserID:           fix.CarrierAct.UserID,
		EventID:          event.ID,
		ResponseID:       response.ID,
		LotID:            lot.ID,
		JWT:              browserStudioJWT(fix.CarrierAct.UserID, fix.TenantID),
		RfxNumber:        event.RfxNumber,
		CompetitorOffer:      competitor.offer,
		CompetitorName:       competitor.name,
		CompetitorAnswer:     competitor.answer,
		CompetitorCompanyID:  competitor.companyID,
		CompetitorResponseID: competitor.responseID,
	}
}

type carrierXlsxCompetitor struct {
	offer      string
	name       string
	answer     string
	companyID  uuid.UUID
	responseID uuid.UUID
}

func seedCarrierXlsxCompetitor(t *testing.T, env *testEnv, fix buyerFixture, eventID, lotID, questionID uuid.UUID) carrierXlsxCompetitor {
	t.Helper()
	ctx := context.Background()
	sent := carrierXlsxCompetitor{
		offer:  "777777.02",
		name:   "SENTINEL-XLSX-CARRIER-B",
		answer: "SENTINEL-COMPETITOR-ANSWER-B",
	}
	carrierBID := uuid.New()
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		carrierBID, fix.TenantID, sent.name, "CARRIER"); err != nil {
		t.Fatalf("seed competitor company: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, eventID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: eventID, CompanyID: carrierBID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add competitor participant: %v", err)
	}
	responseBID := uuid.New()
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_responses (id, tenant_id, rfx_event_id, participant_company_id, status, commercial_score, total_score)
		VALUES ($1,$2,$3,$4,'SUBMITTED',75,75)`,
		responseBID, fix.TenantID, eventID, carrierBID); err != nil {
		t.Fatalf("insert competitor response: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_response_offer_lines (tenant_id, rfx_response_id, rfx_lot_id, amount, currency_code, comment)
		VALUES ($1,$2,$3,$4,'RUB',$5)`,
		fix.TenantID, responseBID, lotID, sent.offer, "SENTINEL-OFFER-B"); err != nil {
		t.Fatalf("insert competitor offer: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO rfx.rfx_answers (tenant_id, rfx_response_id, question_id, answer_value_json, answer_source, validation_version, version)
		VALUES ($1,$2,$3,$4::jsonb,$5,1,1)`,
		fix.TenantID, responseBID, questionID, `"`+sent.answer+`"`, domain.AnswerSourceCarrierDeclared); err != nil {
		t.Fatalf("insert competitor answer: %v", err)
	}
	sent.companyID = carrierBID
	sent.responseID = responseBID
	return sent
}

func startCarrierXlsxRfxService(t *testing.T, env *testEnv) (string, *http.Server) {
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

func startCarrierXlsxCompanyStub(t *testing.T, fix browserCarrierXlsxFixture) *browserCarrierCompanyStub {
	t.Helper()
	stub := &browserCarrierCompanyStub{}
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

func (s *browserCarrierCompanyStub) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func startCarrierXlsxProductionGateway(t *testing.T, rfxServiceURL, origins string, identity *browserIdentityStub, companyURL string) (string, *browserGatewayProcess) {
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
		"RFX_EXCEL_EXCHANGE_ENABLED=true",
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	verifyBrowserGatewayRfxRoute(t, gatewayURL, rfxServiceURL)
	return gatewayURL, proc
}

func startCarrierXlsxWebProcurement(t *testing.T, gatewayURL string, fix browserCarrierXlsxFixture, port string) (string, *webProcurementCmd) {
	t.Helper()
	webURL := "http://127.0.0.1:" + port
	env := overrideProcessEnv(os.Environ(),
		"NUXT_PUBLIC_API_BASE_URL="+gatewayURL,
		"NUXT_E2E_GATEWAY_URL="+gatewayURL,
		"NUXT_PUBLIC_DEFAULT_TENANT_ID="+fix.TenantID.String(),
		"NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED=true",
		"RFX_EXCEL_EXCHANGE_UI=1",
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

func (s *browserCarrierXlsxLiveStack) shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopBrowserWebProcurement(t, s.webCmd)
	if s.rfxSrv != nil {
		_ = s.rfxSrv.Shutdown(ctx)
	}
	if s.gatewayProc != nil {
		shutdownBrowserGatewayProcess(s.gatewayProc)
	}
	verifyDevPortsReleased(t, carrierXlsxBrowserPort)
}

func verifyCarrierXlsxGatewayProbe(t *testing.T, stack *browserCarrierXlsxLiveStack) {
	t.Helper()
	eventURL := stack.gatewayURL + "/api/v1/rfx-events/" + stack.fixture.EventID.String()
	probeCarrierXlsxGET(t, stack, eventURL+"/own-response?carrier_company_id="+stack.fixture.CarrierCompanyID.String(), "own-response")
	probeCarrierXlsxGET(t, stack, eventURL+"/carrier-responses/"+stack.fixture.ResponseID.String()+"/xlsx-export", "export")
}

func probeCarrierXlsxGET(t *testing.T, stack *browserCarrierXlsxLiveStack, url, label string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("probe %s request: %v", label, err)
	}
	req.Header.Set("Authorization", "Bearer "+stack.fixture.JWT)
	req.Header.Set("X-Company-ID", stack.fixture.CarrierCompanyID.String())
	req.Header.Set("Origin", stack.webURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe carrier XLSX %s: %v", label, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("probe carrier XLSX %s: status=%d url=%s", label, resp.StatusCode, url)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != stack.webURL {
		t.Fatalf("probe carrier XLSX %s CORS origin=%q want %q", label, got, stack.webURL)
	}
}

func runCarrierXlsxPlaywrightSuite(t *testing.T, stack *browserCarrierXlsxLiveStack) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "carrier-xlsx")
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
		"BROWSER_E2E_CARRIER_COMPANY_ID="+fix.CarrierCompanyID.String(),
		"BROWSER_E2E_BUYER_COMPANY_ID="+fix.BuyerCompanyID.String(),
		"BROWSER_E2E_EVENT_ID="+fix.EventID.String(),
		"BROWSER_E2E_RESPONSE_ID="+fix.ResponseID.String(),
		"BROWSER_E2E_LOT_ID="+fix.LotID.String(),
		"BROWSER_E2E_RFX_NUMBER="+fix.RfxNumber,
		"BROWSER_E2E_USER_ID="+fix.UserID.String(),
		"BROWSER_E2E_COMPETITOR_OFFER="+fix.CompetitorOffer,
		"BROWSER_E2E_COMPETITOR_NAME="+fix.CompetitorName,
		"BROWSER_E2E_COMPETITOR_ANSWER="+fix.CompetitorAnswer,
		"BROWSER_E2E_COMPETITOR_COMPANY_ID="+fix.CompetitorCompanyID.String(),
		"BROWSER_E2E_COMPETITOR_RESPONSE_ID="+fix.CompetitorResponseID.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
