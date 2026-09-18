import { test, expect } from "@playwright/test";
import {
  adminURL,
  addScoringCriteria,
  gatewayURL,
  procurementURL,
  seedBuyerAdminSession,
  seedBuyerProcurementSession,
  stubBuyerCompanies,
  bootstrapProcurementSession,
  gotoScoringStep,
  waitForScoringModelReady,
  scoringWorkspaceLoading,
} from "./helpers";

const CRITERION_CYCLES = 10;

test.describe("SCORING-E2E criterion add stability loop", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(!adminURL || !procurementURL || !gatewayURL, "BROWSER_E2E URLs required");
    await seedBuyerAdminSession(page);
    await seedBuyerProcurementSession(page);
    await stubBuyerCompanies(page);
    await bootstrapProcurementSession(page);
  });

  for (let cycle = 1; cycle <= CRITERION_CYCLES; cycle++) {
    test(`criterion DOM stability cycle ${cycle}/${CRITERION_CYCLES}`, async ({
      page,
    }) => {
      await gotoScoringStep(page);
      await waitForScoringModelReady(page);

      await addScoringCriteria(page, 1);
      await addScoringCriteria(page, 2);

      const cards = page.getByTestId("scoring-criterion-card");
      await expect(cards).toHaveCount(2, { timeout: 30_000 });

      const codeInputs = page.getByTestId("scoring-criterion-code");
      await codeInputs.nth(0).fill(`HSE_${cycle}`);
      await page.getByTestId("scoring-criterion-name").nth(0).fill(`HSE ${cycle}`);
      await page.getByTestId("scoring-criterion-weight").nth(0).fill("40");
      await codeInputs.nth(1).fill(`CAP_${cycle}`);
      await page.getByTestId("scoring-criterion-name").nth(1).fill(`Capacity ${cycle}`);
      await page.getByTestId("scoring-criterion-weight").nth(1).fill("60");

      const bindings = page.getByTestId("scoring-question-binding");
      await expect(bindings.nth(0).locator("option")).toHaveCount(3, {
        timeout: 120_000,
      });
      await bindings.nth(0).selectOption("ADR_AVAILABLE");
      await bindings.nth(1).selectOption("FLEET_COUNT");

      await cards
        .nth(0)
        .getByTestId("scoring-knockout-boolean-false")
        .check();

      await expect(page.getByTestId("scoring-add-criterion")).toBeEnabled({
        timeout: 30_000,
      });
      await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout: 30_000 });
    });
  }
});
