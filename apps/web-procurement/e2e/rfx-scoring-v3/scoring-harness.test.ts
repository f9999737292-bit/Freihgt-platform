import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

describe("scoring E2E harness conventions", () => {
  it("matrix runner script enforces readiness failure stop", () => {
    const scriptPath = process.env.RFX_SCORING_MATRIX_RUNNER_SCRIPT;
    if (!scriptPath) return;
    const source = readFileSync(scriptPath, "utf8");
    expect(source).toContain("if ($readExit -eq 0)");
    expect(source).toContain("READINESS_FAILS -> ACCEPTANCE_STARTED=NO");
    expect(source).toContain("MATRIX STOPPED ON RUN");
  });

  it("registers studio response listener before navigation in gotoScoringStep", () => {
    const source = readFileSync(resolve(__dirname, "helpers.ts"), "utf8");
    const fnStart = source.indexOf("export async function gotoScoringStep");
    expect(fnStart).toBeGreaterThan(-1);
    const fnBody = source.slice(fnStart, fnStart + 900);
    const listenerIdx = fnBody.indexOf("waitForStudioLoad(page)");
    const gotoIdx = fnBody.indexOf("page.goto(");
    expect(listenerIdx).toBeGreaterThan(-1);
    expect(gotoIdx).toBeGreaterThan(-1);
    expect(listenerIdx).toBeLessThan(gotoIdx);
  });

  it("READY-03 registers score-model tenant listener before navigation", () => {
    const source = readFileSync(resolve(__dirname, "helpers.ts"), "utf8");
    const fnStart = source.indexOf(
      "export async function loadScoringStepWithTenantProbe",
    );
    expect(fnStart).toBeGreaterThan(-1);
    const fnBody = source.slice(fnStart, fnStart + 700);
    const listenerIdx = fnBody.indexOf("waitForScoreModelTenantRequest(page)");
    const gotoIdx = fnBody.indexOf("gotoScoringStep(page)");
    expect(listenerIdx).toBeGreaterThan(-1);
    expect(gotoIdx).toBeGreaterThan(-1);
    expect(listenerIdx).toBeLessThan(gotoIdx);
    expect(fnBody).toContain('scoreModelTenantHeader(request.headers())');
    expect(fnBody).not.toContain('observedTenant = ""');
  });

  it("acceptance uses stable criterion helper instead of cached locator scroll", () => {
    const acceptance = readFileSync(
      resolve(__dirname, "scoring-acceptance.spec.ts"),
      "utf8",
    );
    expect(acceptance).toContain("addScoringCriteria(page, 1)");
    expect(acceptance).toContain("addScoringCriteria(page, 2)");
    expect(acceptance).not.toMatch(/getByTestId\("scoring-add-criterion"\)/);

    const helpers = readFileSync(resolve(__dirname, "helpers.ts"), "utf8");
    expect(helpers).toContain("export async function addScoringCriteria");
    expect(helpers).toContain('getByTestId("scoring-state-loading")');
    expect(helpers).toContain("toBeHidden");
  });

  it("criterion loop spec covers ten stable add cycles", () => {
    const source = readFileSync(
      resolve(__dirname, "scoring-acceptance-criterion-loop.spec.ts"),
      "utf8",
    );
    expect(source).toContain("const CRITERION_CYCLES = 10");
    expect(source).toContain("addScoringCriteria(page, 2)");
    expect(source).toContain("scoring-knockout-boolean-false");
    expect(source).not.toContain("scoring-save-draft");
  });

  it("READY-03 isolation contract requires non-empty tenant header predicate", () => {
    const source = readFileSync(resolve(__dirname, "helpers.ts"), "utf8");
    const fnStart = source.indexOf(
      "export function waitForScoreModelTenantRequest",
    );
    expect(fnStart).toBeGreaterThan(-1);
    const fnBody = source.slice(fnStart, fnStart + 650);
    expect(fnBody).toContain("isScoreModelGetRequest(req.url(), req.method(), forEvent)");
    expect(fnBody).toContain("scoreModelTenantHeader(req.headers()).length > 0");
  });
});
