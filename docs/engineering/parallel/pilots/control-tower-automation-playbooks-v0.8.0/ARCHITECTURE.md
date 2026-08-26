# Control Tower Automation Rules & Operational Playbooks v0.8.0

Deterministic automation/rules layer and versioned operational playbooks for BINTRANS Control Tower.

## Scope

v0.8.0 introduces **recommend-first** automation: rules evaluate controlled triggers, match deterministic conditions, and create operator recommendations linked to immutable playbook versions. Operators accept recommendations to start playbook executions with step-level progress tracking.

**In scope:** tenant-scoped rules, playbooks, recommendations, executions, dry-run, audit, RBAC, Control Tower / Case / Work Item integration.

**Out of scope:** autonomous resolve/close/rebook, external notifications, arbitrary code in rules, LLM playbook generation, migration apply, tests, deployment.

## Principle

```
AUTOMATION SHOULD FIRST RECOMMEND
```

Modes `observe` and `recommend` only. `guarded_auto` reserved in schema but rejected on activation.

## Diagram A — Evaluation lifecycle

```text
Operational Event
      ↓
Trigger Normalize
      ↓
Active Rules (by triggerType)
      ↓
Condition Evaluation (pure, no I/O)
      ↓
Rule Match
      ↓
Idempotency Key
      ↓
Already Exists? ── YES → No Duplicate
      ↓ NO
Recommendation (recommend mode)
      ↓
Operator Accept / Dismiss
      ↓
Playbook Execution → Steps → Completed
```

## Diagram B — Rule lifecycle

```text
draft → active ⇄ disabled
         ↓
       retired (history preserved)
```

## Diagram C — Recommendation lifecycle

```text
Recommendation
   ↓ Accept
Execution → Steps → Completed

Recommendation
   ↓ Dismiss (reason required)
Dismissed (immutable)
```

## Diagram D — Deduplication

```text
Trigger → Rule Match → Idempotency Key
                          ↓
                    UNIQUE (tenant, key)
                          ↓
              Existing? → skip (deduplicated)
```

## Discovery summary

| Item | Finding |
|---|---|
| Existing rule engine | NO |
| Existing playbook model | NO |
| Event bus | Kafka/outbox in shipment-service; not used for CT automation |
| Audit | `automation_audit_event` + case event patterns |
| Persistence | `control-tower-read-model-service` / `control_tower` schema |

## Domain model

### AutomationRule

Tenant-scoped rule with integer `version`, controlled `triggerType`, JSONB `conditions` (schema v1), optional `playbookId`, `executionMode`, `priority`, statuses `draft|active|disabled|retired`.

### AutomationTrigger

Normalized envelope: `triggerId`, `triggerType`, tenant, optional shipment/work item/case/risk/exception IDs, typed `attributes`, `correlationId`, `causationId`.

### Conditions

- Logic: `ALL` / `ANY`, max nesting depth 2, max 25 clauses
- Allowlisted fields and operators (`eq`, `in`, `gte`, …)
- No JS/SQL/CEL/Python execution

### OperationalPlaybook

Versioned template: `operational_playbook` + `operational_playbook_version` + `operational_playbook_step`. Active playbook edits create new immutable version.

### AutomationRecommendation

Frozen `ruleVersion` + `playbookVersion`. Statuses: `pending|accepted|dismissed|expired|completed`. Unique `idempotency_key` per tenant.

### PlaybookExecution

One execution per accepted recommendation (unique constraint). Steps copied from immutable version; statuses `pending|in_progress|done|skipped`. Required steps block completion.

## Evaluation

Centralized `EvaluateRules(trigger, context, activeRules)` in read-model service:

1. Load active rules for `triggerType` only (index: tenant + status + trigger_type)
2. Build `AutomationContext` from trigger attributes
3. Pure condition evaluation — no network/DB from evaluator
4. Priority sort + stable tie-break by rule ID
5. Same-playbook dedup: highest priority rule wins

### Trigger integration (v0.8.0)

- Risk sync (gateway): `risk_created` for high/critical assessments
- Internal: `POST /internal/v1/control-tower/automation/evaluate`

Read-time enrichment (work item/case detail) does **not** create recommendations.

## Security

- Tenant from verified headers/JWT — never from browser body
- RBAC: `VIEW_AUTOMATION`, `MANAGE_AUTOMATION_RULES` (PLATFORM_ADMIN), `MANAGE_PLAYBOOKS`, `VIEW_RECOMMENDATIONS`, `START_PLAYBOOK`, `MANAGE_PLAYBOOK_EXECUTION`
- SYSTEM actor for automated matches; user actor for accept/dismiss/step mutations

## Failure isolation

Automation errors are logged; risk sync, exceptions, tracking, ETA, slots remain primary — automation failure does not block canonical operations.

## v0.8.1 extension point

Schema includes `guarded_auto` and `system_action` step type (rejected in v0.8.0). Loop protection metadata (`correlationId`, `causationId`) prepared for guarded automation phase.

## Example rule

```text
WHEN slot_projected_miss
AND slotProjectedLateSeconds >= 900
AND riskLevel IN [high, critical]
THEN recommend playbook "Delivery Slot Miss Response"
```

## Example playbook

**Delivery Slot Miss Response**

1. Review ETA and telemetry freshness
2. Contact carrier (`contact_carrier`)
3. Confirm latest ETA
4. Request slot reschedule if required (`request_slot_reschedule`)
5. Update operational case (`create_case` action guidance)
6. Continue monitoring (`monitor`)

## Migration

`000029_add_control_tower_automation_playbooks_v0.8.0` — **NOT APPLIED** in this pilot.

Tables:

- `control_tower.automation_rule` / `automation_rule_version`
- `control_tower.operational_playbook` / `operational_playbook_version` / `operational_playbook_step`
- `control_tower.automation_recommendation`
- `control_tower.playbook_execution` / `playbook_execution_step`
- `control_tower.automation_audit_event`

---

## v0.8.0.1 — Runtime completion

### Runtime flow

```text
Domain signal (risk/ETA/slot/tracking/SLA/exception/case)
      ↓
TriggerAdapter (api-gateway or read-model ingress)
      ↓
AutomationTriggerIngress (loop guard + metrics)
      ↓
EvaluateTrigger → active rules by triggerType
      ↓
Condition evaluation (fail-closed)
      ↓
Recommendation (idempotent UNIQUE tenant+key)
      ↓
Operator accept → PlaybookExecution
      ↓
automation_audit_event
```

### Wired trigger types (v0.8.0.1)

| Trigger | Source |
|---|---|
| `risk_created` | Risk sync (high/critical) |
| `eta_at_risk` | Risk evaluator ETA signals |
| `eta_projected_late` | Risk evaluator late ETA signals |
| `slot_at_risk` / `slot_projected_miss` / `slot_actual_missed` | Risk evaluator slot signals |
| `tracking_stale` / `tracking_lost` | Risk evaluator telemetry signals |
| `sla_warning` / `sla_breached` | Shipment SLA + exception workflow SLA |
| `exception_created` | Exception workflow ensure (new only) |
| `case_created` | Case create (read-model, skips automation causation) |

### Idempotency

Pipe-delimited `idempotency_key` includes tenant, rule, rule version, trigger type/id, entity IDs, and `stateVersion`. DB constraint `UNIQUE (tenant_id, idempotency_key)` with `ON CONFLICT DO NOTHING`.

### Loop protection

Triggers with `causationId` prefix `automation:` or `sourceOrigin=automation` are skipped. Case create honors `X-Causation-ID` header.

### Observability

Prometheus metrics: `automation_triggers_total`, `automation_trigger_duplicates_total`, `automation_rule_evaluations_total`, `automation_rule_matches_total`, `automation_executions_total`, `automation_execution_duration_seconds`.

### Known limitations

- No Kafka consumer for automation (synchronous HTTP ingress only)
- `guarded_auto` and `system_action` remain disabled
- Playbook actions are operator-guided; no autonomous side effects
- `work_item_created` / `case_status_changed` triggers defined but not yet wired
- Audit read API not exposed
