# Control Tower Operator Workspace v0.5.1 — UX Completion

Baseline: `404ec8988e4f33aecfa5a96802a987faa1420010` (v0.5 operator workspace foundations).

v0.5.1 closes product-facing UX gaps in the operator workspace without introducing a new domain layer. Backend v0.5 APIs are reused; one additive read-model field (`criticalWork` per operator in workload) was added because the team workload table required critical-urgency counts not previously exposed per operator.

## Gap audit (v0.5 → v0.5.1)

| Area | v0.5 state | v0.5.1 |
|------|------------|--------|
| Handoff modal / preview / confirm | Missing | Full flow with recipient picker, note, preview, confirm |
| Handoff partial results | Missing | Inline result panel with per-item failure reasons |
| Handoff history | Missing | Recent handoffs panel with open-details |
| Handoff details | Missing | Modal with transferred/failed items and navigation |
| Team workload | Missing | Supervisor table + unassigned pool + view queue |
| Saved views CRUD | Create + apply only | Create, apply, rename, update, duplicate, delete, set default, private/shared |
| Bulk unassign / acknowledge | Partial | Surfaced with mixed-selection intersection validation |
| Bulk partial results + retry | Missing | Result panel; retry failed only (idempotent items) |
| Claim 409 UX | Toast only | Friendly RU/EN/ZH message + workspace refresh |
| Ownership badges | Missing | Semantic text badges (mine / other / unassigned) |
| Pagination | Missing | Server page controls preserving filters |
| Active / completed navigation | Missing | Mode tabs; completed preset for history |
| Unified details drawer | Minimal | Exception + risk fields, materialized link, timeline categories |
| Materialized risk navigation | Missing | Link from risk drawer to actual exception |
| Empty states | Generic | Preset-specific empty states |
| Default saved view on load | Partial | Server default view applied on workspace load |

## Handoff UX

1. Select eligible work items → **Handoff**
2. Choose receiving operator (tenant user lookup; no free-text user IDs)
3. Optional note (max 2000 chars)
4. **Preview transfer** — recipient, counts (exceptions, risks, critical, SLA breached), note
5. **Confirm** — server authoritative transfer
6. Result shows transferred/failed counts; failures list reference + safe reason (ownership changed, completed, permission denied, unavailable)

## Team workload

- `GET /api/v1/control-tower/workload` (RBAC: same roles as Control Tower access)
- Table: operator, active, critical urgency, P1, P2, SLA breached, SLA warning, critical/high risks
- **Unassigned pool** row separate from operators
- **View queue** navigates to filtered workspace (does not change ownership)
- No rankings, leaderboards, or productivity scoring

## Saved views

- Persist supported fields only: `preset`, `queueMode` in filters JSON (not exposed in UI)
- Explicit **Update view** action (no silent rewrite on filter change)
- Duplicate creates a new private view from source filters
- Delete confirms; if default deleted, falls back to product default (`my_work`)
- Default view loaded from server preference on workspace init

## Bulk actions

- Supported: claim, assign, unassign, acknowledge (no bulk resolve / clear / materialize)
- Mixed exception + risk selection: actions disabled unless intersection of `availableActions` supports them
- Partial outcome panel: requested / succeeded / failed + failure table
- **Retry failed** re-submits only failed items from last outcome

## Claim conflicts (409)

Message: ownership changed by another operator. Queue refreshes; open drawer reconciles if possible.

## Pagination

Server `page` / `limit` / `hasNext` / `total`. Invalid page after mutation resets gracefully.

## Completed history

**Completed** mode uses `include_completed` + `completed` preset. Active queue remains separate.

## Materialized risk navigation

Risk drawer shows **Materialized as → Open actual exception** when `linkedEventId` is present.

## Timeline semantics

Categories: SYSTEM, OPERATOR, WORKFLOW, RISK, HANDOFF — chronological ordering preserved.

## Backend additive fix (v0.5.1)

**Workload `criticalWork` per operator** — counts active items with `urgency=critical` during in-memory workload scan. Required for team workload Critical column; no new migration.

Files: `workitem.go` (domain), `workitem_repository.go`, read-model `workspace_handler.go`, gateway `workspace.go`, `client_workspace.go`.

## Known limitations

| Limitation | Notes |
|------------|-------|
| `shipment.updated_at` proxy | Staleness proxy only — not GPS/telematics offline |
| Slot windows | Status-only; no precise slot countdown |
| Read model required | Mutations show API unavailability when projection disabled |
| Workload scale | In-memory scan capped at 10 000 active items per tenant — SQL aggregation follow-up |
| Migration 000024 | Exists but not applied in this pilot |

## Testing

Deferred by product owner — no tests run in v0.5.1 delivery.
