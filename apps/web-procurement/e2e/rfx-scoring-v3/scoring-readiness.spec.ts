import { test, expect } from "@playwright/test";
import {
  adminURL,
  eventId,
  gatewayURL,
  jwt,
  seedBuyerAdminSession,
  stubBuyerCompanies,
  loadScoringStepWithTenantProbe,
  scoringModelPath,
  gotoScoringStep,
  waitForScoringModelReady,
} from "./helpers";

test.describe("RFx v3.0D scoring readiness diagnostics", () => {
  test.beforeEach(async ({ page }) => {
    await seedBuyerAdminSession(page);
    await stubBuyerCompanies(page);
  });

  test("SCORING-E2E-READY-01 fails fast when score-model API returns HTTP 500", async ({
    page,
  }) => {
    test.skip(!adminURL || !gatewayURL, "BROWSER_E2E URLs required");

    await page.route(`**${scoringModelPath()}`, async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({
          error: {
            code: "INTERNAL",
            message: "injected scoring model failure",
          },
        }),
      });
    });

    const started = Date.now();
    await gotoScoringStep(page);
    await expect(page.getByTestId("scoring-state-load-failed")).toBeVisible({
      timeout: 30_000,
    });

    await expect(async () => {
      await waitForScoringModelReady(page, { timeout: 15_000 });
    }).rejects.toThrow(/Scoring model load failed.*score-model.*HTTP 500/);

    expect(Date.now() - started).toBeLessThan(60_000);
  });

  test("SCORING-E2E-READY-02 waits for delayed score-model response once", async ({
    page,
  }) => {
    test.skip(!adminURL || !gatewayURL, "BROWSER_E2E URLs required");

    let delayedFirstRequest = false;
    await page.route(`**${scoringModelPath()}`, async (route) => {
      if (!delayedFirstRequest) {
        delayedFirstRequest = true;
        await new Promise((resolve) => setTimeout(resolve, 1500));
      }
      await route.continue();
    });

    await gotoScoringStep(page);
    await waitForScoringModelReady(page, { timeout: 120_000 });
    expect(delayedFirstRequest).toBe(true);
    await page.getByTestId("scoring-add-criterion").click();
    await expect(page.getByTestId("scoring-criterion-card")).toHaveCount(1);
  });

  test("SCORING-E2E-READY-03 does not substitute tenant on score-model probe", async ({
    page,
  }) => {
    test.skip(!adminURL || !gatewayURL || !jwt, "BROWSER_E2E URLs required");

    const observedTenant = await loadScoringStepWithTenantProbe(page);
    expect(observedTenant).toBe(process.env.BROWSER_E2E_TENANT_ID);
    expect(observedTenant.length).toBeGreaterThan(0);
  });
});
