# Low-code Pilot Week-3 Monitoring Cadence Runbook v0.1

## Purpose

Operational guide for **event-based** low-code pilot monitoring after **Monitoring Cadence Decision v0.1** (`CADENCE_AD_HOC_ON_EVENT`).

Replaces the default **Pilot Monitoring Continuation v0.x** daily loop. Use this runbook when a trigger event fires (see trigger matrix).

## Selected Cadence

| Field | Value |
|-------|-------|
| Cadence | **CADENCE_AD_HOC_ON_EVENT** |
| Default continuation packs | **disabled** (no automatic v0.8+) |
| Full evidence refresh pack | **Monitoring Evidence Refresh Pack v0.1** (stakeholder request) |
| Reference | `LOW_CODE_PILOT_WEEK3_MONITORING_CADENCE_DECISION_V0.1.md` |

## When To Run Monitoring

Run read-only checks when **any** trigger occurs:

| Trigger | Minimum action |
|---------|----------------|
| Human PM assigns operators/dates | health + audit; update session docs |
| `LIVE_SESSION_CONFIRMED` | health + audit + active templates; prep Capture Retry |
| Ops ready for Remote Auth-On Repeat | health; hand off to Auth-On Repeat pack |
| P0/P1 suspected | full health + smoke; escalate to Fix Pack |
| Runtime/template change | health + active templates + audit delta |
| Stakeholder requests fresh evidence | full runbook + Evidence Refresh pack |
| Real feedback collected | triage per feedback runbook; no assumed fixes |
| One week with no changes | optional spot-check (health + audit only) |

## When Not To Run Monitoring

Do **not** run a monitoring pack or create continuation docs when:

- No trigger event has occurred since last evidence snapshot
- Only purpose is "daily habit" without new PM/operator/ops/runtime data
- Feedback/sessions status unchanged and stakeholder has not requested evidence
- Environment is down and fix is out of scope for current pack (document blocker only)

**Rule:** absence of change is not a trigger for a full **Pilot Monitoring Continuation** pack.

## Minimum Read-only Checks

Execute in order when a trigger fires:

```powershell
cd D:\Projects\freight-platform
git status --short
git log --oneline -5
make health-check
```

Pilot tenant read-only API (via api-gateway):

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
curl.exe -s -o NUL -w "audit HTTP %{http_code}`n" -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/audit-events?limit=20"
curl.exe -s -o NUL -w "metrics HTTP %{http_code}`n" `
  "http://localhost:8088/metrics"
```

**If runtime/template changed** — add active template GETs:

```powershell
curl.exe -s -o NUL -w "TO template HTTP %{http_code}`n" -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER&template_code=transport_order_default"
curl.exe -s -o NUL -w "SH template HTTP %{http_code}`n" -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=SHIPMENT&template_code=shipment_default"
curl.exe -s -o NUL -w "BR template HTTP %{http_code}`n" -H "X-Tenant-ID: $T" `
  "http://localhost:8080/api/v1/low-code/form-templates/active?entity_type=BILLING_REGISTER&template_code=billing_register_default"
```

**If stakeholder requests full evidence or P0/P1 ruled out but assurance needed:**

```powershell
make integration-smoke-test
cd apps\web-admin; npm run build
```

## Evidence Rules

1. Record snapshot time, commit hash, and HTTP status codes.
2. Create or update evidence docs **only when** trigger warrants (not on every calendar day).
3. Reference prior snapshot when nothing changed: "no delta since v0.7".
4. Do not fabricate operator feedback or session confirmations.
5. Prefer **Monitoring Evidence Refresh Pack v0.1** over new continuation v0.x numbering.

## No-Write Rules

During any cadence-triggered check:

| Operation | Allowed |
|-----------|---------|
| GET health / audit / metrics / templates | **yes** |
| PUT / POST / PATCH / DELETE | **no** |
| migration execute / batch execute / import execute | **no** |
| template publish | **no** |
| production writes | **no** |
| manual DB edits | **no** |
| destructive Docker commands | **no** |

## Escalation Rules

| Condition | Escalation | Next pack |
|-----------|------------|-----------|
| Health-check failure | P0/P1 triage | Runtime Pilot Fix Pack v0.1 |
| Smoke/build failure after runtime change | P1 | Runtime Pilot Fix Pack v0.1 |
| Audit/metrics unavailable | P1–P2 | Fix Pack or DevOps |
| Ops staging ready | Parallel track | Remote Auth-On Repeat v0.1 |
| Sessions confirmed | Unblock feedback | Capture Retry Pack v0.1 |
| No trigger for 7+ days | Optional spot-check only | None (unless stakeholder asks) |

## Next Action Mapping

| Situation | Next pack |
|-----------|-----------|
| Ops ready | Remote Auth-On Repeat Pack v0.1 |
| `LIVE_SESSION_CONFIRMED` | First Real Operator Feedback Capture Retry Pack v0.1 |
| Fresh evidence requested | Monitoring Evidence Refresh Pack v0.1 |
| P0/P1 suspected | Runtime Pilot Fix Pack v0.1 |
| No trigger | **No automatic pack** |

**Unblock path:** human PM → operators + dates → `LIVE_SESSION_CONFIRMED` → Capture Retry Pack v0.1
