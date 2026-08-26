# Cross-Workstream Request Template

## Status

Formal template — EDO-0.2 freeze

## Usage

Required before any agent modifies paths outside its workstream allowed scope. Submit to orchestrator / integrator for Task Contract issuance.

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
