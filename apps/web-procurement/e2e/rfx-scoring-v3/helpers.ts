import { expect, type APIRequestContext, type Page } from "@playwright/test";

export const adminURL = process.env.BROWSER_E2E_ADMIN_URL || "";
export const procurementURL = process.env.BROWSER_E2E_PROCUREMENT_URL || "";
export const gatewayURL = process.env.BROWSER_E2E_GATEWAY_URL || "";
export const jwt = process.env.BROWSER_E2E_JWT || "";
export const tenantId = process.env.BROWSER_E2E_TENANT_ID || "";
export const companyId = process.env.BROWSER_E2E_BUYER_COMPANY_ID || "";
export const eventId = process.env.BROWSER_E2E_EVENT_ID || "";
export const userId = process.env.BROWSER_E2E_USER_ID || "";
export const carrierAJWT = process.env.BROWSER_E2E_CARRIER_A_JWT || "";
export const carrierACompany =
  process.env.BROWSER_E2E_CARRIER_A_COMPANY_ID || "";
export const carrierBJWT = process.env.BROWSER_E2E_CARRIER_B_JWT || "";
export const carrierBCompany =
  process.env.BROWSER_E2E_CARRIER_B_COMPANY_ID || "";
export const legacyEventId = process.env.BROWSER_E2E_LEGACY_EVENT_ID || "";

export function assertGatewayHost(url: string) {
  const expected = new URL(gatewayURL);
  const actual = new URL(url);
  expect(actual.host).toBe(expected.host);
}

export async function seedBuyerAdminSession(page: Page) {
  await page.addInitScript(
    ({ token, tenant, company, user }) => {
      localStorage.setItem(
        "freight_admin_session",
        JSON.stringify({
          token,
          user: {
            id: user,
            tenant_id: tenant,
            email: "buyer@freight.test",
            full_name: "Buyer",
            preferred_locale: "ru-RU",
            status: "ACTIVE",
            roles: ["PROCUREMENT_MANAGER"],
          },
        }),
      );
      localStorage.setItem("freight_admin_tenant_id", tenant);
      localStorage.setItem("freight_admin_company_id", company);
      document.cookie = "freight_admin_locale=ru-RU; path=/";
    },
    { token: jwt, tenant: tenantId, company: companyId, user: userId },
  );
}

export async function seedBuyerProcurementSession(page: Page) {
  await page.addInitScript(
    ({ token, tenant, company, user }) => {
      localStorage.setItem(
        "freight_procurement_session",
        JSON.stringify({
          token,
          user: {
            id: user,
            tenant_id: tenant,
            email: "buyer@freight.test",
            full_name: "Buyer",
            preferred_locale: "ru-RU",
            status: "ACTIVE",
            roles: ["PROCUREMENT_MANAGER"],
          },
        }),
      );
      localStorage.setItem("freight_procurement_tenant_id", tenant);
      localStorage.setItem("freight_procurement_company_id", company);
      document.cookie = "freight_procurement_locale=ru-RU; path=/";
    },
    { token: jwt, tenant: tenantId, company: companyId, user: userId },
  );
}

async function carrierSubmit(
  request: APIRequestContext,
  token: string,
  carrierCompany: string,
  adr: boolean,
  fleet: number,
) {
  const headers = {
    Authorization: `Bearer ${token}`,
    "X-Company-ID": carrierCompany,
    "Content-Type": "application/json",
  };
  const carrierQuery = `?carrier_company_id=${carrierCompany}`;
  const start = await request.post(
    `${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/start${carrierQuery}`,
    { headers, data: {} },
  );
  expect(start.ok()).toBeTruthy();
  const ws = await start.json();
  const questions =
    ws.questionnaire?.sections?.flatMap(
      (s: { questions?: Array<{ id: string; question_code: string }> }) =>
        s.questions ?? [],
    ) ?? [];
  const adrQ = questions.find(
    (q: { question_code: string }) => q.question_code === "ADR_AVAILABLE",
  );
  const fleetQ = questions.find(
    (q: { question_code: string }) => q.question_code === "FLEET_COUNT",
  );
  expect(adrQ?.id).toBeTruthy();
  expect(fleetQ?.id).toBeTruthy();
  const patch = await request.patch(
    `${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/answers${carrierQuery}`,
    {
      headers,
      data: {
        save_version: ws.save_version,
        answers: [
          { question_id: adrQ!.id, value: adr },
          { question_id: fleetQ!.id, value: fleet },
        ],
      },
    },
  );
  expect(patch.ok()).toBeTruthy();
  const saved = await patch.json();
  const submit = await request.post(
    `${gatewayURL}/api/v1/rfx-events/${eventId}/carrier-response/submit${carrierQuery}`,
    {
      headers,
      data: { save_version: saved.save_version },
    },
  );
  expect(submit.ok()).toBeTruthy();
}

export async function submitCarrierA(request: APIRequestContext) {
  await carrierSubmit(request, carrierAJWT, carrierACompany, true, 50);
}

export async function submitCarrierB(request: APIRequestContext) {
  await carrierSubmit(request, carrierBJWT, carrierBCompany, false, 100);
}

/** Company-service is not part of the scoring browser chain; stub directory lookups for evaluation UI labels. */
export async function stubBuyerCompanies(page: Page) {
  await page.route("**/api/v1/companies**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: [
          {
            id: companyId,
            legal_name: "Buyer A",
            company_type: "SHIPPER",
            status: "ACTIVE",
          },
          {
            id: carrierACompany,
            legal_name: "Carrier A",
            company_type: "CARRIER",
            status: "ACTIVE",
          },
          {
            id: carrierBCompany,
            legal_name: "Carrier B",
            company_type: "CARRIER",
            status: "ACTIVE",
          },
        ],
      }),
    });
  });
}

export function evaluationPath(forEventId = eventId) {
  return `/tenders/${forEventId}/evaluation`;
}

export function scoringModelPath(forEvent = eventId) {
  return `/api/v1/rfx-events/${forEvent}/score-model`;
}

function scoreModelTenantHeader(
  headers: Record<string, string>,
): string {
  return (
    headers["x-tenant-id"] ??
    headers["X-Tenant-ID"] ??
    ""
  ).trim();
}

function isScoreModelGetRequest(url: string, method: string, forEvent = eventId) {
  return (
    method === "GET" &&
    url.includes(scoringModelPath(forEvent))
  );
}

/** Install before navigation; resolves when a score-model GET carries X-Tenant-ID. */
export function waitForScoreModelTenantRequest(
  page: Page,
  options?: { timeout?: number; forEventId?: string },
) {
  const forEvent = options?.forEventId ?? eventId;
  const path = scoringModelPath(forEvent);
  return page.waitForRequest(
    (req) =>
      isScoreModelGetRequest(req.url(), req.method(), forEvent) &&
      scoreModelTenantHeader(req.headers()).length > 0,
    { timeout: options?.timeout ?? 120_000 },
  );
}

/** Open scoring step and return tenant header from the authoritative score-model GET. */
export async function loadScoringStepWithTenantProbe(page: Page) {
  const tenantRequest = waitForScoreModelTenantRequest(page);
  await gotoScoringStep(page);
  await waitForScoringModelReady(page);
  const request = await tenantRequest;
  return scoreModelTenantHeader(request.headers());
}

/** Wait for studio shell API before scoring workspace assertions. */
export async function waitForStudioLoad(page: Page) {
  return page.waitForResponse(
    (resp) => {
      const url = resp.url();
      return (
        url.includes(`/api/v1/rfx-events/${eventId}/studio`) &&
        !url.includes("tenant_id=") &&
        resp.status() < 500
      );
    },
    { timeout: 120_000 },
  );
}

/** Open scoring step after studio shell is loaded. */
export async function gotoScoringStep(page: Page) {
  const studioResponse = waitForStudioLoad(page);
  await page.goto(`${adminURL}/rfx/${eventId}/studio?step=scoring`, {
    waitUntil: "domcontentloaded",
  });
  if (await page.locator(".backend-status-banner--offline").isVisible().catch(() => false)) {
    throw new Error(
      `Backend unavailable banner visible before studio load completed (gateway=${gatewayURL})`,
    );
  }
  const studioLoad = await studioResponse;
  assertGatewayHost(studioLoad.url());
  if (studioLoad.status() >= 400) {
    throw new Error(
      `Studio API failed: GET ${studioLoad.url()} -> HTTP ${studioLoad.status()}`,
    );
  }
  await expect(page.getByTestId("rfx-scoring-workspace")).toBeVisible({
    timeout: 120_000,
  });
}

export type ScoringModelProbeResult = {
  ok: boolean;
  status: number;
  endpoint: string;
  requestId?: string;
  reason: string;
};

/** Direct gateway probe for scoring model load diagnostics (fail-fast messages). */
export async function probeScoringModelLoad(
  page: Page,
  forEventId = eventId,
): Promise<ScoringModelProbeResult> {
  const endpoint = scoringModelPath(forEventId);
  return page.evaluate(
    async ({ gw, path, company, tenantKey, sessionKey }) => {
      const raw = localStorage.getItem(sessionKey);
      if (!raw) {
        return {
          ok: false,
          status: 0,
          endpoint: `${gw}${path}`,
          reason: "missing-session",
        };
      }
      const session = JSON.parse(raw) as {
        token: string;
        user: { id: string; tenant_id: string };
      };
      const tenant = localStorage.getItem(tenantKey) || session.user.tenant_id;
      const resp = await fetch(`${gw}${path}`, {
        headers: {
          Authorization: `Bearer ${session.token}`,
          "X-Company-ID": company,
          "X-Tenant-ID": tenant,
          "X-User-ID": session.user.id,
          Accept: "application/json",
        },
      });
      const requestId =
        resp.headers.get("x-request-id") ??
        resp.headers.get("X-Request-Id") ??
        undefined;
      if (resp.ok) {
        return {
          ok: true,
          status: resp.status,
          endpoint: `${gw}${path}`,
          requestId,
          reason: "",
        };
      }
      const body = await resp.text();
      return {
        ok: false,
        status: resp.status,
        endpoint: `${gw}${path}`,
        requestId,
        reason: body.slice(0, 240),
      };
    },
    {
      gw: gatewayURL,
      path: endpoint,
      company: companyId,
      tenantKey: "freight_admin_tenant_id",
      sessionKey: "freight_admin_session",
    },
  );
}

export function scoringWorkspaceLoading(page: Page) {
  return page.getByTestId("scoring-state-loading");
}

/** Wait until scoring model UI is interactive; fail fast on load error banner. */
export async function waitForScoringModelReady(
  page: Page,
  options?: { timeout?: number },
) {
  const timeout = options?.timeout ?? 120_000;
  await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
  await expect
    .poll(
      async () => {
        if (await page.getByTestId("scoring-state-load-failed").isVisible()) {
          const probe = await probeScoringModelLoad(page);
          throw new Error(
            `Scoring model load failed: GET ${probe.endpoint} -> HTTP ${probe.status}${
              probe.requestId ? ` request_id=${probe.requestId}` : ""
            }${probe.reason ? ` body=${probe.reason}` : ""}`,
          );
        }
        if (await scoringWorkspaceLoading(page).isVisible()) return "pending";
        const ready = await page.getByTestId("scoring-model-ready").isVisible();
        if (!ready) return "pending";
        const addCriterion = page.getByTestId("scoring-add-criterion");
        if (
          !(await addCriterion.isVisible()) ||
          !(await addCriterion.isEnabled())
        )
          return "pending";
        return "ready";
      },
      { timeout, intervals: [250, 500, 1000] },
    )
    .toBe("ready");
}

/** Click add-criterion until criterion card count reaches expected value. */
export async function addScoringCriteria(
  page: Page,
  expectedCount: number,
  options?: { timeout?: number },
) {
  const timeout = options?.timeout ?? 30_000;
  const cards = page.getByTestId("scoring-criterion-card");
  await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
  let current = await cards.count();
  while (current < expectedCount) {
    await expect(page.getByTestId("scoring-add-criterion")).toBeEnabled({
      timeout,
    });
    await page.getByTestId("scoring-add-criterion").click();
    current += 1;
    await expect(cards).toHaveCount(current, { timeout });
    await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
  }
  await expect(cards).toHaveCount(expectedCount, { timeout });
}

/** Persist scoring draft and wait for PUT completion before further mutations. */
export async function saveScoringDraft(page: Page, options?: { timeout?: number }) {
  const timeout = options?.timeout ?? 60_000;
  await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
  const saveBtn = page.getByTestId("scoring-save-draft");
  await expect(saveBtn).toBeEnabled({ timeout });
  const saveResponse = page.waitForResponse(
    (resp) =>
      resp.request().method() === "PUT" &&
      resp.url().includes(scoringModelPath()) &&
      resp.status() < 500,
    { timeout },
  );
  await saveBtn.click();
  await saveResponse;
  await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
}

/** Run readiness validation and wait for ready panel. */
export async function validateScoringReadiness(
  page: Page,
  options?: { timeout?: number },
) {
  const timeout = options?.timeout ?? 60_000;
  await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
  const validateBtn = page.getByTestId("scoring-validate");
  await expect(validateBtn).toBeEnabled({ timeout });
  const validateResponse = page.waitForResponse(
    (resp) =>
      resp.url().includes("/score-model/validate") &&
      resp.request().method() === "POST" &&
      resp.status() < 500,
    { timeout },
  );
  await validateBtn.click();
  await validateResponse;
  await expect(scoringWorkspaceLoading(page)).toBeHidden({ timeout });
  await expect(page.getByTestId("scoring-readiness-ready")).toBeVisible({
    timeout,
  });
}

/** Seed procurement-origin localStorage and warm Pinia session before cross-app studio steps. */
export async function bootstrapProcurementSession(page: Page) {
  await page.goto(`${procurementURL}/tenders`, {
    waitUntil: "domcontentloaded",
  });
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/, { timeout: 30_000 });
  await page.waitForFunction(
    () => {
      const raw = localStorage.getItem("freight_procurement_session");
      if (!raw) return false;
      try {
        const data = JSON.parse(raw) as { token?: string };
        return Boolean(data.token);
      } catch {
        return false;
      }
    },
    undefined,
    { timeout: 30_000 },
  );
}

export async function assertBrowserScoresApi(
  page: Page,
  responseIds: string[],
  forEventId = eventId,
) {
  for (const responseId of responseIds) {
    const probe = await page.evaluate(
      async ({ gw, ev, responseId, company, tenantKey, sessionKey }) => {
        const raw = localStorage.getItem(sessionKey);
        if (!raw)
          return {
            ok: false,
            status: 0,
            reason: "missing-session",
            responseId,
          };
        const session = JSON.parse(raw) as {
          token: string;
          user: { id: string; tenant_id: string };
        };
        const tenant =
          localStorage.getItem(tenantKey) || session.user.tenant_id;
        const resp = await fetch(
          `${gw}/api/v1/rfx-events/${ev}/responses/${responseId}/score`,
          {
            headers: {
              Authorization: `Bearer ${session.token}`,
              "X-Company-ID": company,
              "X-Tenant-ID": tenant,
              "X-User-ID": session.user.id,
              Accept: "application/json",
            },
          },
        );
        const body = resp.ok ? await resp.json() : await resp.text();
        return {
          ok: resp.ok,
          status: resp.status,
          reason: resp.ok ? "" : String(body).slice(0, 240),
          responseId,
          calculationStatus: resp.ok
            ? body?.qualification?.calculation_status
            : undefined,
          totalScore: resp.ok ? body?.qualification?.total_score : undefined,
        };
      },
      {
        gw: gatewayURL,
        ev: forEventId,
        responseId,
        company: companyId,
        tenantKey: "freight_procurement_tenant_id",
        sessionKey: "freight_procurement_session",
      },
    );
    expect(
      probe.ok,
      `browser score probe failed for ${responseId}: status=${probe.status} ${probe.reason}`,
    ).toBeTruthy();
    expect(probe.calculationStatus).toBe("CALCULATED");
  }
}

export async function assertBrowserResponsesApi(
  page: Page,
  forEventId = eventId,
) {
  const probe = await page.evaluate(
    async ({ gw, ev, company, tenantKey, sessionKey }) => {
      const raw = localStorage.getItem(sessionKey);
      if (!raw) return { ok: false, status: 0, reason: "missing-session" };
      const session = JSON.parse(raw) as {
        token: string;
        user: { id: string; tenant_id: string };
      };
      const tenant = localStorage.getItem(tenantKey) || session.user.tenant_id;
      const resp = await fetch(`${gw}/api/v1/rfx-events/${ev}/responses`, {
        headers: {
          Authorization: `Bearer ${session.token}`,
          "X-Company-ID": company,
          "X-Tenant-ID": tenant,
          "X-User-ID": session.user.id,
          Accept: "application/json",
        },
      });
      const body = resp.ok ? "" : await resp.text();
      return { ok: resp.ok, status: resp.status, reason: body.slice(0, 240) };
    },
    {
      gw: gatewayURL,
      ev: forEventId,
      company: companyId,
      tenantKey: "freight_procurement_tenant_id",
      sessionKey: "freight_procurement_session",
    },
  );
  expect(
    probe.ok,
    `browser responses probe failed: status=${probe.status} ${probe.reason}`,
  ).toBeTruthy();
}
