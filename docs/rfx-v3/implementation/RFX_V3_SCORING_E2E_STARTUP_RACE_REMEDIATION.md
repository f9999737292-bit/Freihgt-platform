# RFx Scoring v3 Browser E2E Startup Race Remediation

## Failure context

| Marker | Value |
|---|---|
| SOURCE_FAILURE_RUN | 35103192900 |
| SOURCE_FAILURE_JOB | 104817557882 |
| SOURCE_SUCCESS_RERUN_JOB | 104822754384 |
| FAILURE_CLASSIFICATION | SERVICE_STARTUP_RACE + FRONTEND_READINESS_RACE |
| BASE_SHA | cb0878b1ab47af80136d3d6e03f75c8133fd3795 |

Attempt 1 timed out waiting for `scoring-add-criterion` while the UI showed
«Не удалось загрузить модель скоринга» and the browser console logged HTTP 500.
Attempt 2 on the same commit passed in ~30s.

## Root cause

| Marker | Value |
|---|---|
| HTTP_500_ENDPOINT | `GET /api/v1/rfx-events/{eventId}/score-model` |
| READINESS_FALSE_POSITIVE | YES |
| SERVICE_RACE | YES |
| FRONTEND_READINESS_RACE | YES |
| ROOT_CAUSE_PROVEN | YES |

The browser harness waited only for Nuxt `/login` HTTP 200. That is not equivalent
to score-model readiness. The scoring workspace shell (`rfx-scoring-workspace`) is
always rendered, including loading and error states, so Playwright treated the page
as ready and then blocked on a control that only exists after a successful model load.

## Fix

| Marker | Value |
|---|---|
| FIX_LAYER | A + C |
| BOUNDED_READINESS | YES |
| ERROR_STATE_FAIL_FAST | YES |
| HTTP_500_MASKED | NO |

1. **Harness (A):** bounded gateway probe for `GET .../score-model` before Playwright starts.
2. **Frontend (C):** `data-testid="scoring-model-ready"` marks successful model load only.
3. **Tests (C):** `waitForScoringModelReady()` waits for model-ready / interactive add-criterion;
   fails fast with endpoint/status/body when `scoring-state-load-failed` is visible.
4. **Regression:** readiness diagnostics spec (`SCORING-E2E-READY-01..03`) runs via
   `TestRfxScoringV3_BrowserE2E_ReadinessDiagnostics`; CI gate keeps acceptance only.

## Validation markers

```
SCORING_E2E_REMEDIATION_STATUS=IMPLEMENTED_ACCEPTED
SCORING_E2E_STABILITY_STATUS=IMPLEMENTED_ACCEPTED
CONTROLLER_VERDICT=ACCEPT_SCORING_E2E_REMEDIATION
CORRECTIVE_COMMIT_REQUIRED=NO
MATRIX_10_10=PASS
READINESS_30_30=PASS
ACCEPTANCE_10_10=PASS
FINAL_CI_RUN=35351974141
FINAL_CI_HEAD=efdee4cf23c50e7a4dc4addb27c9a9c97b1cb15e
FINAL_CI_RESULT=SUCCESS
FINAL_CI_JOB_COUNT=51
BLOCKER_FINDINGS=0
HIGH_FINDINGS=0
MEDIUM_FINDINGS=0
LOW_FINDINGS=3
ERP_API_E2_STATUS=NOT_STARTED
ERP_API_E2_AUTHORIZED=NO
NEXT_ACTION=ACCEPTANCE_ALIGNMENT_AND_CONTROLLED_MERGE_PR141
```
