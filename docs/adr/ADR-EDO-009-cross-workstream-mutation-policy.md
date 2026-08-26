# ADR-EDO-009: Cross-Workstream Mutation Policy

## Status

Accepted — architecture freeze (EDO-0.2)

## Context

Parallel engineering uses Git worktrees and Task Contracts. Discovery defined workstream ownership (PLAT, LOG, CT, FC, EDO, TEDO, MM, FF, INFRA) and policy `NO_CROSS_WORKSTREAM_MUTATION`. EDO agents must not modify LOG-owned code directly.

## Decision

### Workstream ownership (frozen)

| Code | Owns |
|------|------|
| **PLAT** | api-gateway, identity-service, company-service, tenant/RBAC conventions, `core.*` identity schema |
| **LOG** | transport-order-service, shipment-service, rfx-service, contract-rate-service, tracking-service, `transport.*`, `rfx.*`, `contract_rate.*` |
| **CT** | control-tower-read-model-service, gateway Control Tower BFF, `control_tower.*` |
| **FC** | billing-register-service, freight-cost-service, `billing.*` (except payment tables), `freight_cost.*` |
| **FF** | payment-service, future receivable/factoring modules, `billing.payment_*` |
| **EDO** | document-service evolution, EDO domain extensions, `documents.*` |
| **TEDO** | transport-edo-service (future), EPD operator adapters |
| **MM** | TransportJourney/Leg/CargoHandover design — **implements via LOG/shipment-service** per ADR-EDO-003 |
| **INFRA** | migrations coordination, docker-compose, Selectel ops scripts, Kafka topic provisioning |

### Mutation rules

1. **No direct edits** across workstream allowed paths without approved `CROSS_WORKSTREAM_REQUEST`.
2. **Shared collision zones** — OpenAPI (`packages/openapi/**`), coordinated migrations, gateway proxy map — require integrator/orchestrator assignment.
3. **Read-only consumption** — EDO/TEDO/FF may read canonical IDs from other contexts via API or events; never duplicate SSOT tables.
4. **EDO agents** — forbidden from modifying `services/shipment-service/**`, `services/rfx-service/**`, etc.

### Request template

Formal template: [docs/program/cross-workstream-request-template.md](../program/cross-workstream-request-template.md)

Required fields: `REQUEST_ID`, `FROM`, `TO`, `REASON`, `AFFECTED_AGGREGATE`, `REQUIRED_CONTRACT`, `BREAKING_CHANGE`, `MIGRATION_IMPACT`, `SECURITY_IMPACT`, `ROLLBACK_IMPACT`.

### Example

```text
REQUEST_ID=CWS-EDO-2026-002
FROM=EDO
TO=LOG
REASON=Index shipment_id on document relationships for EPD correlation queries
AFFECTED_AGGREGATE=Document (documents schema only — EDO-owned)
REQUIRED_CONTRACT=Optional shipment_id FK in documents.document_relationships (EDO migration)
BREAKING_CHANGE=NO
MIGRATION_IMPACT=documents schema only; no transport.shipments change
SECURITY_IMPACT=tenant_id predicate on new index
ROLLBACK_IMPACT=drop index / nullable column
```

## Consequences

- Task Contracts must declare workstream and allowed paths
- MM leg implementation requires LOG Task Contract even if designed by MM architect

## References

- `docs/engineering/TASK_CONTRACT_TEMPLATE.md`
- `docs/engineering/COLLISION_POLICY.md`
- Discovery cross-workstream rules
