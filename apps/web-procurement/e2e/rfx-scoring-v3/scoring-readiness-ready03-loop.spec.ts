import { test, expect } from "@playwright/test";
import {
  adminURL,
  gatewayURL,
  jwt,
  loadScoringStepWithTenantProbe,
  seedBuyerAdminSession,
  stubBuyerCompanies,
} from "./helpers";

const READY03_CYCLES = 10;

test.describe("SCORING-E2E READY-03 tenant isolation loop", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(!adminURL || !gatewayURL || !jwt, "BROWSER_E2E URLs required");
    await seedBuyerAdminSession(page);
    await stubBuyerCompanies(page);
  });

  for (let cycle = 1; cycle <= READY03_CYCLES; cycle++) {
    test(`READY-03 isolation cycle ${cycle}/${READY03_CYCLES}`, async ({
      page,
    }) => {
      const observedTenant = await loadScoringStepWithTenantProbe(page);
      expect(observedTenant).toBe(process.env.BROWSER_E2E_TENANT_ID);
      expect(observedTenant.length).toBeGreaterThan(0);
    });
  }
});
