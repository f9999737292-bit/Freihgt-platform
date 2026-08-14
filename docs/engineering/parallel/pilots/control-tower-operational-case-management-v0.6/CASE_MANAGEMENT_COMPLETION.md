# Control Tower Operational Case Management — v0.6.1 Backend Core

Backend checkpoint for centralized Case Health, derived severity, override semantics, participant APIs, KPI/filter reuse. Builds on v0.6 commit `183a47c`.

## Case Health Model

Single canonical projection: `repository.BatchCaseHealth(ctx, tenantID, caseIDs) → map[caseID]CaseHealth`.

Fields:

| Field | Source |
|-------|--------|
| `hasSlaBreach` | Any linked active exception with SLA status breached |
| `hasSlaWarning` | Any linked active exception with SLA status warning |
| `nearestSlaDueAt` | Earliest active SLA deadline (server UTC) |
| `highestExceptionPriority` | Max priority among active linked exceptions (p1 > p2 > p3 > p4) |
| `highestRiskLevel` | Max level among active linked risks (critical > high > medium > low) |
| `openActionCount` | Actions with status `open` or `in_progress` |
| `overdueActionCount` | Open/in_progress actions with `due_at < NOW()` (UTC) |
| `nearestActionDueAt` | Earliest due date among open/in_progress actions |
| `activeWorkItemCount` | Active linked exceptions + active risks (deduplicated) |

Reused identically for: case list enrichment, case detail, KPI aggregation, `sla_at_risk` / `overdue_actions` filters, severity derivation input.

## SLA Semantics

- Only **active** linked exceptions (`status <> resolved`) contribute.
- `hasSlaBreach` and `hasSlaWarning` may both be true on one case (different linked items).
- Display precedence: breach > warning > normal.
- **SLA at risk** = `hasSlaBreach OR hasSlaWarning` (one case counts once in `slaAtRiskCases` KPI).

## Action Overdue Semantics

Overdue when: `status IN (open, in_progress) AND due_at IS NOT NULL AND due_at < server_now (UTC)`.

Not persisted; derived at read time.

## Derived Severity

Centralized in `domain.DeriveCaseSeverity(CaseSeverityInput)`:

| Severity | Rule |
|----------|------|
| **critical** | P1 exception + SLA breach, OR critical business impact + P1 exception |
| **high** | P1 exception, OR SLA breach, OR critical predictive risk |
| **medium** | P2 exception, OR SLA warning, OR high predictive risk |
| **low** | otherwise |

Recalculated via `refreshDerivedSeverity` on link changes and case create; skipped when `severity_override = true`.

## Effective Severity

```
effectiveSeverity = severityOverride ? manual : derivedSeverity
```

`severity_override` is a boolean flag; clearing sets override false and effective = freshly derived.

Override recalculation of derived severity never overwrites manual override.

## Severity Override / Clear

- **Set**: `PATCH /cases/{id}` with `{ "severity": "medium", "version": N }` → `case_severity_overridden` audit event.
- **Clear**: `{ "clearSeverityOverride": true, "version": N }` → `case_severity_override_cleared` audit event.

Audit metadata: `previousDerivedSeverity`, `previousEffectiveSeverity`, `previousOverride`, `newOverride`, `newEffectiveSeverity`, actor, timestamp.

## Materialized Risk Deduplication

Materialized risks linked to an active exception (`actual_event_id`) are excluded from health/severity counts to avoid double-counting with the exception work item.

## Participant Owner Invariant

- Canonical owner: `operational_case.owner_user_id`.
- Participant API accepts only `collaborator` and `observer`.
- Role `owner` rejected; use claim/assign/reassign/unassign.
- Cannot remove canonical owner via participant DELETE.

## Participant RBAC

Gateway enforces `CanManageCaseParticipants` (Control Tower access roles). Read permission alone does not grant participant mutation.

## Tenant User Validation

Before add: `core.users` existence check for `(userId, tenantId)`. Display names never trusted from request body.

## Duplicate Protection

`ON CONFLICT (case_id, user_id) DO UPDATE SET role` — idempotent re-add with same or updated role.

## Batch Aggregation / Performance

Case list: one `BatchCaseHealth` call for all page case IDs (batched SQL for actions, links, exceptions, risks). No per-case N+1.

KPI: single ID scan + one `BatchCaseHealth` over open cases.

## KPI Counting

Each metric counts **cases once**:

- `casesWithSlaBreach`: cases with any breach
- `casesWithSlaWarning`: cases with any warning (independent of breach)
- `slaAtRiskCases`: breach OR warning
- `casesWithOverdueActions`: `overdueActionCount > 0`

## Database

No new migration. Uses existing v0.6 schema (`000025` unapplied): participants, severity override flag, case events.

## API Additions

- `POST/PATCH/DELETE .../cases/{caseId}/participants`
- Case list/detail: `health` object, severity fields
- Filters: `preset=sla_at_risk`, `preset=overdue_actions`, `hasSlaBreach`, `hasSlaWarning`, `overdueActions`
- KPI: `casesWithSlaBreach`, `casesWithSlaWarning`, `slaAtRiskCases`, `casesWithOverdueActions`
