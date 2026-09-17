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
});
