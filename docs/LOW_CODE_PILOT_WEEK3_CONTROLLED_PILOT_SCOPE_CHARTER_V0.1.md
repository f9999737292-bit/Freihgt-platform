# Low-code Pilot Week-3 Controlled Pilot Scope Charter v0.1

## Purpose

Defines **approved scope** for controlled internal low-code pilot after `CONTROLLED_PILOT_APPROVED`.

## Pilot Identity

| Field | Value |
|-------|-------|
| Pilot name | Week-3 Low-code Controlled Pilot |
| PM / Coordinator | **Феликс Асаев** |
| Approval date | 2026-06-26 |
| Status | **ACTIVE** |

## Tenant & Environment

| Field | Value |
|-------|-------|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Environment | **dev / demo** |
| Production data | **not approved** |

## Approved Users

| User | Role | Entity focus |
|------|------|--------------|
| Пейсахов Семен | Operator | TRANSPORT_ORDER |
| Крылова Любовь | Operator | SHIPMENT |
| Курганова Наталья | Operator | BILLING_REGISTER |
| Pilot team / dev admin | Support | All (dev access) |

## Approved Demo Entities

| Entity type | Demo ID | Entity UUID |
|-------------|---------|-------------|
| TRANSPORT_ORDER | DEMO-TO-001 | `2db04b49-665c-469f-bcb1-ffeb1274fedb` |
| SHIPMENT | DEMO-SH-PLANNED | `14d405e2-0152-4030-b356-eec464a3cc66` |
| BILLING_REGISTER | DEMO-BR-001 | `cf7dbc77-395f-42a2-9717-476e4cd93796` |

## Templates (Read / Limited Write)

| Entity | Template code | Status |
|--------|---------------|--------|
| TRANSPORT_ORDER | `transport_order_default` | PUBLISHED v1 |
| SHIPMENT | `shipment_default` | PUBLISHED v1 |
| BILLING_REGISTER | `billing_register_default` | PUBLISHED v1 |

## Allowed Operations

| Operation | Allowed |
|-----------|---------|
| Login / panel review | **yes** |
| Read custom field values | **yes** |
| Approved limited writes | **yes** — per pilot runbooks |
| Audit history review | **yes** |
| Dev seed scripts (local) | **yes** — existing Makefile targets |

## Prohibited Operations

| Operation | Prohibited |
|-----------|------------|
| Template publish | **yes** — without approved pack |
| Migration execute | **yes** |
| Batch migration execute | **yes** |
| Import execute | **yes** |
| Production writes | **yes** |
| Manual DB edits | **yes** |
| Destructive Docker commands | **yes** |

## Monitoring

| Cadence | `CADENCE_AD_HOC_ON_EVENT` |
|---------|---------------------------|
| Runbook | `LOW_CODE_PILOT_WEEK3_MONITORING_CADENCE_RUNBOOK_V0.1.md` |
| Trigger matrix | `LOW_CODE_PILOT_WEEK3_MONITORING_TRIGGER_MATRIX_V0.1.md` |

## Operator Feedback Baseline

All three operators: scenario **да**, rating **5**, decision **ready**, **замечаний нет** (session 26.06.2026 12:30).

No feedback-derived fixes required from intake.

## Parallel Tracks

| Track | Status |
|-------|--------|
| Remote Auth-On Repeat | **pending ops** — BL-W3-003 |
| Production readiness | **not approved** — future governance |

Reference: `LOW_CODE_PILOT_WEEK3_CONTROLLED_PILOT_APPROVAL_V0.1.md`
