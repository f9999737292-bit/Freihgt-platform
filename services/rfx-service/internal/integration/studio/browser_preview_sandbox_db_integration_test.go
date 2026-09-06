//go:build integration

package studio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func countCarrierResponseRows(t *testing.T, env *testEnv, tenantID, eventID string) (responses int, answers int) {
	t.Helper()
	ctx := context.Background()
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_responses WHERE tenant_id=$1 AND rfx_event_id=$2`,
		tenantID, eventID).Scan(&responses); err != nil {
		t.Fatalf("count responses: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_answers a
		JOIN rfx.rfx_responses r ON r.id = a.response_id
		WHERE r.tenant_id=$1 AND r.rfx_event_id=$2`,
		tenantID, eventID).Scan(&answers); err != nil {
		t.Fatalf("count answers: %v", err)
	}
	return responses, answers
}

func TestRfxStudio_BrowserE2E_PreviewSandboxDbIsolation(t *testing.T) {
	if os.Getenv("BROWSER_E2E") != "1" {
		t.Skip("set BROWSER_E2E=1 to run live browser E2E against local stack")
	}
	if os.Getenv("PREVIEW_SANDBOX_DB_PROOF") != "1" {
		t.Skip("set PREVIEW_SANDBOX_DB_PROOF=1 to run preview sandbox DB isolation proof")
	}
	env := setupTestEnv(t)
	fix := seedBrowserStudioFixture(t, env)
	responsesBefore, answersBefore := countCarrierResponseRows(t, env, fix.TenantID.String(), fix.EventID.String())
	stack := startBrowserLiveStackWithEnv(t, env, fix)
	t.Cleanup(stack.shutdown)
	if err := runPlaywrightSpec(t, stack, "studio-acceptance.spec.ts"); err != nil {
		t.Fatalf("playwright preview sandbox spec: %v", err)
	}
	responsesAfter, answersAfter := countCarrierResponseRows(t, env, fix.TenantID.String(), fix.EventID.String())
	if responsesAfter != responsesBefore {
		t.Fatalf("response count delta: before=%d after=%d", responsesBefore, responsesAfter)
	}
	if answersAfter != answersBefore {
		t.Fatalf("answer count delta: before=%d after=%d", answersBefore, answersAfter)
	}
	t.Logf("PREVIEW_DB_ISOLATION responses=%d answers=%d delta=0", responsesAfter, answersAfter)
}

func startBrowserLiveStackWithEnv(t *testing.T, env *testEnv, fix browserStudioFixture) *browserLiveStack {
	t.Helper()
	const webPort = "3021"
	rfxURL, rfxSrv := startBrowserRfxService(t, env)
	identity := startBrowserIdentityStub(t, browserIdentityRolesForBuyer(fix.UserID.String()))
	webOrigin := browserGatewayEnvForStack(t, webPort)
	gatewayURL, gatewayProc := startBrowserProductionGateway(t, rfxURL, webOrigin, identity)
	webURL, webCmd := startBrowserWebAdmin(t, gatewayURL, fix, webPort)
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

func runPlaywrightSpec(t *testing.T, stack *browserLiveStack, specFile string) error {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		return err
	}
	e2eDir := filepath.Join(root, "apps", "web-procurement", "e2e", "rfx-studio")
	configPath := filepath.Join(e2eDir, "playwright.config.ts")
	cmd := exec.Command("npx", "playwright", "test", "--config", configPath, specFile)
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
