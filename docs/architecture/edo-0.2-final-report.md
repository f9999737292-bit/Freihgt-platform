# BINTRANS EDO-0.2

## DOMAIN & EVENT CONTRACT ARCHITECTURE FREEZE

## FINAL REPORT

**Phase:** EDO-0.2  
**Mode:** ARCHITECTURE_ONLY  
**Baseline discovery:** BINTRANS ECOSYSTEM + EDO ARCHITECTURE DISCOVERY v0.1 (`DISCOVERY_PASS_WITH_FINDINGS`)  
**Date:** 2026-08-26

---

### BASELINE

```text
REPOSITORY_ROOT=D:\Projects\freight-platform-wt\edo-ecosystem-architecture-v0.1
CURRENT_WORKTREE=D:\Projects\freight-platform-wt\edo-ecosystem-architecture-v0.1
CURRENT_BRANCH=discovery/edo-ecosystem-architecture-v0.1
HEAD_SHA=d0005bd8b055b0d2250e5092a0c1c0484decf540
ORIGIN_MAIN_SHA=d0005bd8b055b0d2250e5092a0c1c0484decf540
WORKTREE_ISOLATED=YES
WORKTREE_CLEAN=NO (documentation additions only — see git status)
```

Primary BINTRANS dev worktree `D:\Projects\freight-platform` on branch `test/control-tower-projection-rebuild-live-acceptance-v0.4` at `a5163c3` — **untouched**.

---

### SAFETY

```text
PRODUCT_CODE_MODIFIED=NO
DATABASE_MODIFIED=NO
MIGRATIONS_CREATED=NO
STAGING_MODIFIED=NO
PRODUCTION_MODIFIED=NO
SELECTEL_MODIFIED=NO
CURRENT_BINTRANS_FLOW_IMPACT=NONE
```

Documentation created/updated only under `docs/architecture/**`, `docs/adr/**`, `docs/events/**`, `docs/program/**`.

---

### ADRS

| ID | Title | Status |
|----|-------|--------|
| ADR-EDO-001 | Canonical EDO document ownership | Accepted (freeze) |
| ADR-EDO-002 | Billing ↔ EDO boundary (F-009) | Accepted (freeze) |
| ADR-EDO-003 | Canonical Shipment and multimodal extension | Accepted (freeze) |
| ADR-EDO-004 | EPD operator port ownership | Accepted (freeze) |
| ADR-EDO-005 | Receivable vs PaymentObligation | Accepted (freeze) |
| ADR-EDO-006 | Event naming/versioning | Accepted (freeze) |
| ADR-EDO-007 | Legal archive boundary | Accepted (freeze) |
| ADR-EDO-008 | External operator first / own operator ready | Accepted (freeze) |
| ADR-EDO-009 | Cross-workstream mutation policy | Accepted (freeze) |
| ADR-PLAT-001 | Membership/user_roles canonical writer (F-002) | Accepted (freeze) |

Index: [docs/adr/README.md](../adr/README.md)

---

### CANONICAL_OWNERSHIP

| Entity | Owner |
|--------|-------|
| Shipment | shipment-service (sole canonical; ONE_SHIPMENT_ID) |
| TransportJourney / TransportLeg / CargoHandover | shipment-service (Shipment aggregate extension) |
| Terminal | transport-order-service (Location reference) |
| TransportMode | LOG platform enum |
| Document / Package / Signature / Archive | document-service (EDO) |
| EPD operator port / transactions | transport-edo-service (TEDO) |
| BillingRegister / UPD commercial projection | billing-register-service |
| PaymentObligation | payment-service |
| Receivable (future) | FF workstream — distinct aggregate |
| user_roles | identity-service (canonical writer per ADR-PLAT-001) |
| company_memberships | company-service |

**Rejected:** `MultimodalShipment`, `edo_*` parallel identity/logistics tables, shared-go EPD port (deferred).

---

### DOCUMENT_DOMAIN

Aggregate map with immutability and retention: [edo-0.2-domain-model-freeze.md](edo-0.2-domain-model-freeze.md)

14 entities evaluated: Document, DocumentPackage, DocumentRevision, DocumentRelationship, DocumentSignature, CertificateEvidence, PowerOfAttorneyEvidence, DocumentEvent, DeliveryEvidence, OperatorReceipt, FormatDefinition, FormatVersion, ValidationResult, ArchiveManifest.

---

### BILLING_EDO_BOUNDARY

ADR-EDO-002 frozen:

- Billing owns monetary/commercial truth; EDO owns legal/XML/signatures/archive.
- Mandatory `document_id` before operator-facing UPD states.
- UKD/corrections via new Document + DocumentRelationship, not payload duplication.
- Resolves **F-009 UPD SSOT split**.

---

### MULTIMODAL_BOUNDARY

ADR-EDO-003 frozen:

```text
Shipment → TransportJourney → TransportLeg[] → CargoHandover[]
ONE_SHIPMENT_ID_ACROSS_BINTRANS=YES
```

Road-only backward compatible via implicit single leg. Detail: [edo-0.2-multimodal-boundary.md](edo-0.2-multimodal-boundary.md)

---

### RECEIVABLE_BOUNDARY

ADR-EDO-005 frozen:

- **Receivable** ≠ **PaymentObligation**
- Factoring chain: Receivable → FactoringApplication → FactorOffer → Assignment → Financing → Settlement
- PaymentObligation remains payment execution rail; optional cross-reference only

---

### EPD_OPERATOR_PORT

ADR-EDO-004 + [edo-0.2-epd-operator-port-spec.md](edo-0.2-epd-operator-port-spec.md):

- Owner: `transport-edo-service` (not shared-go)
- Operations: SubmitDocument, GetTransactionStatus, ReceiveInboundDocument, Acknowledge, Reject, GetDeliveryEvidence
- `EXTERNAL_OPERATOR_MODE=YES`, `FUTURE_OWN_OPERATOR_READY=YES`, `GIS_EPD_CONNECTED=NO`

---

### DOCUMENT_STATE_MODEL

Orthogonal dimensions (never collapsed):

| Dimension | Owner |
|-----------|-------|
| BUSINESS_STATE | document-service |
| DELIVERY_STATE | document-service |
| SIGNATURE_STATE | document-service |
| OPERATOR_TRANSACTION_STATE | transport-edo-service |

Lifecycle specs for Generic Document, UPD, Transport EPD, EETD package: [edo-0.2-document-state-machines.md](edo-0.2-document-state-machines.md)

---

### EVENT_CONTRACTS

Audited existing events; proposed namespaces `edo.document.*`, `tedo.epd.*`, `mm.transport_leg.*`, `ff.receivable.*`, `ff.factoring.*`.

Full inventory: [docs/events/edo-0.2-event-contracts.md](../events/edo-0.2-event-contracts.md)

6 EXISTING shipment + 12 EXISTING driver Kafka events preserved. DO_NOT_CREATE duplicates documented.

---

### EVENT_VERSIONING

Policy: [docs/events/edo-0.2-event-versioning-policy.md](../events/edo-0.2-event-versioning-policy.md)

Covers: compatibility rules, schema evolution, breaking-change policy, idempotency, outbox requirements, consumer retry, DLQ, ordering assumptions.

---

### CROSS_WORKSTREAM_CONTRACT

Ownership frozen in ADR-EDO-009. Formal template: [docs/program/cross-workstream-request-template.md](../program/cross-workstream-request-template.md)

EDO agents must not modify LOG-owned code directly.

---

### MEMBERSHIP_OWNERSHIP

**F-002 resolved at architecture level** — ADR-PLAT-001:

| Table | Canonical writer |
|-------|------------------|
| `core.company_memberships` | company-service |
| `core.user_roles` | identity-service |

Dual-write continues in code until PLAT-0.1. **OWNERSHIP_FREEZE_BLOCKED=NO** (sufficient evidence from repository).

---

### LEGAL_REQUIREMENTS_REGISTRY

Structure: [edo-0.2-legal-requirements-registry.md](edo-0.2-legal-requirements-registry.md)

All regulatory entries default to `EXTERNAL_LEGAL_VERIFICATION_REQUIRED`. No unsupported compliance claims.

---

### ARCHIVE_BOUNDARY

[edo-0.2-archive-boundary.md](edo-0.2-archive-boundary.md) + ADR-EDO-007

Requirements defined; S3/WORM not provisioned; Selectel WORM **EXTERNAL_INFRA_VERIFICATION_REQUIRED**.

---

### FUTURE_OPERATOR_READINESS

ADR-EDO-008:

```text
EXTERNAL_OPERATOR_MODE=YES
FUTURE_OWN_OPERATOR_READY=YES
OWN_IS_EPD_OPERATOR_MODE=NO
```

Adapter substitution without core workflow change. Gaps documented; no certification claims.

---

### PROGRAM_STATUS_MODEL

[docs/program/workstream-status-v0.1.md](../program/workstream-status-v0.1.md) — PLAT, LOG, CT, FC, EDO, TEDO, MM, FF, INFRA rows with phase, status, BASE_SHA, dependencies, blockers, next phase.

---

### FINDINGS

| ID | EDO-0.2 disposition |
|----|------------------------|
| F-001 Docs placeholders | **Addressed** — event catalog + ADR pack published |
| F-002 Dual user_roles writers | **Resolved (architecture)** — ADR-PLAT-001; code fix → PLAT-0.1 |
| F-003 Mock EDO / no operator | **Resolved (design)** — EPD port spec; impl → TEDO-0.4+ |
| F-004 Local disk storage | **Open** — archive requirements frozen; INFRA-0.1 |
| F-005 Legal verification | **Open** — registry structure; external review required |
| F-006 Event naming duplication | **Addressed** — ADR-EDO-006 + event contracts |
| F-007 Multimodal ROAD-only | **Resolved (design)** — ADR-EDO-003; no MultimodalShipment |
| F-008 No receivable model | **Resolved (design)** — ADR-EDO-005 |
| F-009 UPD SSOT split | **Resolved (architecture)** — ADR-EDO-002 |
| F-010 No trailer entity | **Open** — LOG backlog when EPD requires |
| F-011 No Redis | **Open** — INFRA evaluate for crypto sessions |
| F-012 CT shadow mode | **Open** — EDO must not assume PRIMARY CT |

---

### ARCHITECTURE_GATES

| Gate | Result | Notes |
|------|--------|-------|
| GATE_CANONICAL_OWNERSHIP | **PASS** | Shipment, Document, Billing, EPD port owners frozen |
| GATE_DOCUMENT_BOUNDARY | **PASS** | Billing↔EDO split + orthogonal state dimensions |
| GATE_MULTIMODAL_BOUNDARY | **PASS** | ONE_SHIPMENT_ID; no MultimodalShipment |
| GATE_FINANCE_BOUNDARY | **PASS** | Receivable distinct from PaymentObligation |
| GATE_EVENT_CONTRACTS | **PASS_WITH_FINDING** | JSON Schema files not yet in repo (markdown SSOT only) |
| GATE_CROSS_WORKSTREAM | **PASS** | Template + ADR-EDO-009 |
| GATE_LEGAL_EXTENSIBILITY | **PASS_WITH_FINDING** | Registry empty of verified rules — external review pending |
| GATE_OPERATOR_ABSTRACTION | **PASS** | Port in transport-edo-service; adapter-ready |
| GATE_CURRENT_BINTRANS_COMPATIBILITY | **PASS** | Additive-only; road shipment FSM unchanged |

---

### FINAL_VERDICT

```text
ARCHITECTURE_FREEZE_PASS_WITH_FINDINGS
```

Architecture contracts are frozen and documented. Remaining findings (legal verification, infra archive, JSON Schema artifacts, PLAT dual-write code remediation) are bounded and do not block proceeding to implementation phases under Task Contracts.

---

### NEXT_RECOMMENDED_PHASE

**EDO-0.3 — Document domain extensions (document-service only)**

Deliverables: DocumentPackage, DocumentRelationship, immutable revision rules, signing evidence extensions — additive `documents` schema migrations only, authorized via EDO Task Contract. Parallel: **PLAT-0.1** membership write-path remediation; **INFRA-0.1** object storage design.

**Do not start EDO-0.3 without explicit Task Contract authorization.**

---

### FINAL SAFETY GATE

```text
PRODUCT_CODE_MODIFIED=NO
DATABASE_MODIFIED=NO
MIGRATIONS_CREATED=NO
STAGING_MODIFIED=NO
PRODUCTION_MODIFIED=NO
SELECTEL_MODIFIED=NO
CURRENT_BINTRANS_FLOW_IMPACT=NONE
```

---

## Deliverable index

| Path | Description |
|------|-------------|
| `docs/adr/README.md` | ADR index |
| `docs/adr/ADR-EDO-001` … `009` | Core ADR pack |
| `docs/adr/ADR-PLAT-001` | Membership ownership |
| `docs/architecture/edo-0.2-*` | Domain, states, boundaries, port spec |
| `docs/events/edo-0.2-event-contracts.md` | Event inventory |
| `docs/events/edo-0.2-event-versioning-policy.md` | Versioning policy |
| `docs/program/workstream-status-v0.1.md` | Program status |
| `docs/program/cross-workstream-request-template.md` | CWS template |
