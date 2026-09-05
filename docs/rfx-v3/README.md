# BINTRANS Enterprise RFx v3.0A — Architecture Index

**Status:** Architecture freeze (v3.0A)  
**Branch:** `discovery/bintrans-enterprise-rfx-v3.0a`  
**Mode:** Documentation only — no product code in this stream  
**PR:** [#101](https://github.com/f9999737292-bit/Freihgt-platform/pull/101) (draft — do not merge)

---

## Purpose

Canonical architecture for **Enterprise RFx v3.0A** and **Supplier Qualification**: buyer RFx Studio, configurable questionnaire engine, carrier response workspace, scoring/qualification, Carrier 360, preview/test modes, event architecture, AI boundaries, and versioned publication lifecycle.

**STOP_AFTER_V3_0A=YES** — this stream delivers architecture freeze only.

---

## Document map

### Core architecture

| Document | Scope |
|---|---|
| [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md) | Aggregates, value objects, invariants, company authority |
| [RFX_V3_API.md](./RFX_V3_API.md) | API surfaces, autosave, validation envelopes, server-verified company |
| [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md) | RFx, response, and publication lifecycles |
| [RFX_V3_QUESTIONNAIRE_ENGINE.md](./RFX_V3_QUESTIONNAIRE_ENGINE.md) | Questionnaire definition, validation layers, scoring input rules |
| [RFX_V3_UX.md](./RFX_V3_UX.md) | Buyer Studio & carrier workspace UX contracts |
| [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md) | Tenancy, provenance, preview isolation, company authority |
| [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md) | Current + target PostgreSQL schema |
| [RFX_V3_SCORING_ENGINE.md](./RFX_V3_SCORING_ENGINE.md) | Scoring modes, knockout, explainability |
| [RFX_V3_CARRIER_360.md](./RFX_V3_CARRIER_360.md) | Reusable carrier data, autofill, freshness |
| [RFX_V3_EVENTS.md](./RFX_V3_EVENTS.md) | Domain events, outbox, Kafka conventions |
| [RFX_V3_AI.md](./RFX_V3_AI.md) | Bounded AI assist and safety prohibitions |

### Normative contracts

| Document | Scope |
|---|---|
| **[RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)** | **Authoritative:** server validation, valid-only persistence, draft/autosave/resume, carrier error UX, preview safety, submit/publish gates, versioned post-publication edits |

### Discovery & baseline

| Document | Scope |
|---|---|
| [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md) | Repository-backed current vs target gap analysis |
| [RFX_V3_FUNCTIONAL_BASELINE.md](./RFX_V3_FUNCTIONAL_BASELINE.md) | Industry functional baseline + BINTRANS differentiators |
| [RFX_V3_ROADMAP.md](./RFX_V3_ROADMAP.md) | v3.0A–J release train |
| [RFX_V3_DIAGRAMS.md](./RFX_V3_DIAGRAMS.md) | Mermaid architecture diagrams (10) |

### Architecture Decision Records

| ADR | Decision |
|---|---|
| [ADR-RFX-001](./adr/ADR-RFX-001-UNIFIED-RFX-ENGINE.md) | Unified RFx engine in `rfx-service` |
| [ADR-RFX-002](./adr/ADR-RFX-002-QUESTIONNAIRE-VERSIONING.md) | Immutable published questionnaire versions |
| [ADR-RFX-003](./adr/ADR-RFX-003-CONDITIONAL-RULE-ENGINE.md) | Native conditional rule engine |
| [ADR-RFX-004](./adr/ADR-RFX-004-SCORING-ARCHITECTURE.md) | Configurable scoring + explainability |
| [ADR-RFX-005](./adr/ADR-RFX-005-QUALIFICATION-MODEL.md) | Qualification results and pools |
| [ADR-RFX-006](./adr/ADR-RFX-006-CARRIER-360-AUTOFILL.md) | Carrier 360 autofill with confirmation |
| [ADR-RFX-007](./adr/ADR-RFX-007-ANSWER-PROVENANCE.md) | Authoritative answer source enum |
| [ADR-RFX-008](./adr/ADR-RFX-008-TEMPLATE-VERSIONING.md) | RFx template versioning |
| [ADR-RFX-009](./adr/ADR-RFX-009-COLLABORATION-MODEL.md) | Buyer collaboration roles |
| [ADR-RFX-010](./adr/ADR-RFX-010-AI-SAFETY-BOUNDARY.md) | AI assist-only boundary |
| [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md) | Valid-only authoritative persistence |

---

## Normative contracts

The following flags are **non-negotiable** across v3.0A:

```text
INVALID_DATA_PERSISTENCE=FORBIDDEN
SERVER_VALIDATION_REQUIRED=YES
SAVE_ONLY_VALID_STATE=YES
LAST_VALID_SERVER_STATE_PRESERVED=YES
VALID_NEGATIVE_ANSWER_PERSISTED=YES
KNOCKOUT_ANSWER_PERSISTED=YES
FIELD_ERROR_INLINE=YES
GLOBAL_ERROR_SUMMARY=YES
ERROR_DEEP_LINK=YES
BUYER_AUTOSAVE=YES
CARRIER_RESPONSE_AUTOSAVE=YES
PREVIEW_AS_CARRIER=YES
PREVIEW_DATA_ONLY=YES
REAL_RESPONSE_CREATED=NO
PREVIEW_DATA_NOT_IN_PRODUCTION_ANSWER=YES
SUBMIT_WITH_ERRORS=FORBIDDEN
PUBLISH_WITH_ERRORS=FORBIDDEN
KNOCKOUT_BLOCKS_SAVE=NO
CLIENT_SUPPLIED_COMPANY_AUTHORITY=FORBIDDEN
TENANT_AUTHORITY=SERVER_VERIFIED
USER_AUTHORITY=SERVER_VERIFIED
COMPANY_AUTHORITY=SERVER_VERIFIED
CROSS_COMPANY_SPOOF=DENIED
CROSS_TENANT_SPOOF=DENIED
```

When documents conflict, **`RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md`** is authoritative for validation, draft, autosave, and error UX semantics.

---

## Controller findings closure

| Finding | Status | Resolution |
|---|---|---|
| F101-001 Preview provenance contradiction | **CLOSED** | `BUYER_PREVIEW_TEST` forbidden on production Answer; isolated preview storage |
| F101-002 Architecture freeze incomplete | **CLOSED** | Gap matrix, data model, scoring, events, AI, baseline, diagrams, ADRs |
| F101-003 Company context trust boundary | **CLOSED** | `CLIENT_SUPPLIED_COMPANY_AUTHORITY=FORBIDDEN`; server-verified membership |

---

## Related platform docs

- `docs/engineering/VALIDATION_LEVELS.md`
- `docs/SHIPMENT_STATUS_OUTBOX.md`
- `packages/openapi/` — error envelope schemas
- Existing RFx v1: `services/rfx-service/`, `infrastructure/migrations/000004_create_rfx_tables.up.sql`

---

**Document control:** v3.0A freeze complete. Implementation requires separate task contracts per roadmap wave (v3.0B+).
