# Cross-Workstream Request Template

## Status

Formal template — EDO-0.2 freeze

## Usage

Required before any agent modifies paths outside its workstream allowed scope. Submit to orchestrator / integrator for Task Contract issuance.

## REQUEST_ID convention (canonical)

Format:

```text
CWS-<FROM>-<YEAR>-<NNN>
```

| Segment | Rule |
|---------|------|
| `FROM` | Originating workstream code: `PLAT`, `LOG`, `CT`, `FC`, `EDO`, `TEDO`, `MM`, `FF`, `INFRA` |
| `YEAR` | Calendar year of request submission, e.g. `2026` |
| `NNN` | Zero-padded sequence **per FROM workstream per year**, starting at `001` |

Examples: `CWS-EDO-2026-001`, `CWS-MM-2026-001`, `CWS-TEDO-2026-001`.

**Do not use** alternate shapes such as `CWS-MM-001`, `CWS-EDO-FC-001`, or `CWS-EDO-PLAT-001`.

## Planned requests (EDO-0.2 inventory — not yet submitted)

| REQUEST_ID | FROM → TO | Affected aggregate / contract |
|------------|-----------|-------------------------------|
| `CWS-MM-2026-001` | MM → LOG | `TransportJourney`, `TransportLeg`, `CargoHandover` on `shipment-service`; events `mm.transport_leg.*`, `mm.cargo_handover.*` |
| `CWS-EDO-2026-001` | EDO → FC | BillingRegister ↔ legal `Document`; `edo.document.signed` v1 consumer; mandatory `document_id` before operator-facing UPD states |
| `CWS-EDO-2026-002` | EDO → LOG | Optional `shipment_id` / document correlation indexes in `documents` schema (EDO-owned migration only) |
| `CWS-EDO-2026-003` | EDO → PLAT | `core.user_roles` canonical write path per ADR-PLAT-001; stop dual-write |
| `CWS-TEDO-2026-001` | TEDO → INFRA | Operator credentials vault, archive object storage (S3/WORM), crypto session infrastructure |

---

```text
CROSS_WORKSTREAM_REQUEST

REQUEST_ID:        CWS-{FROM}-{YYYY}-{SEQ}
FROM:              PLAT | LOG | CT | FC | EDO | TEDO | MM | FF | INFRA
TO:                <target workstream(s)>
REASON:            <why the change is needed>
AFFECTED_AGGREGATE:<domain aggregate name>
REQUIRED_CONTRACT:<API endpoint, event name, schema, migration description>
BREAKING_CHANGE:   YES | NO
MIGRATION_IMPACT:  <schemas, tables, rollout notes>
SECURITY_IMPACT:   <tenant isolation, RBAC, trust boundary>
ROLLBACK_IMPACT:   <how to revert safely>
```

---

## Field guidance

| Field | Required | Notes |
|-------|----------|-------|
| REQUEST_ID | yes | Unique; referenced in Task Contract |
| FROM / TO | yes | Must match ADR-EDO-009 ownership |
| REASON | yes | Business or technical justification |
| AFFECTED_AGGREGATE | yes | e.g. Document, Shipment, BillingRegister |
| REQUIRED_CONTRACT | yes | Concrete deliverable the TO workstream must expose |
| BREAKING_CHANGE | yes | If YES, requires ADR and consumer inventory update |
| MIGRATION_IMPACT | yes | Include schema names; `NONE` if docs-only |
| SECURITY_IMPACT | yes | Default: tenant_id scoping review |
| ROLLBACK_IMPACT | yes | Migration down strategy or feature flag |

## Example

```text
CROSS_WORKSTREAM_REQUEST

REQUEST_ID:        CWS-EDO-2026-001
FROM:              EDO
TO:                FC
REASON:            Billing UPD status mirror must react to EDO document signing
AFFECTED_AGGREGATE: UPDDocument (billing), Document (documents)
REQUIRED_CONTRACT: billing-register-service subscribes to edo.document.signed v1;
                   updates upd_documents.status mirror when document_id bound
BREAKING_CHANGE:   NO
MIGRATION_IMPACT:  billing service config + consumer table; no billing schema change
SECURITY_IMPACT:   tenant_id validation on consumer; reject foreign tenant events
ROLLBACK_IMPACT:   disable consumer; billing reverts to manual status updates
```

## Policy reference

[ADR-EDO-009](../adr/ADR-EDO-009-cross-workstream-mutation-policy.md)
