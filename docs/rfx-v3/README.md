# BINTRANS Enterprise RFx v3.0A — Architecture Index

**Status:** Discovery / architecture draft  
**Branch:** `discovery/bintrans-enterprise-rfx-v3.0a`  
**Mode:** Documentation only — no product code in this stream

---

## Purpose

Canonical architecture for **Enterprise RFx v3.0A**: buyer RFx Studio, configurable questionnaire engine, carrier response workspace, scoring/qualification, preview/test modes, and versioned publication lifecycle.

---

## Document map

| Document | Scope |
|---|---|
| [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md) | Core aggregates, value objects, invariants |
| [RFX_V3_API.md](./RFX_V3_API.md) | Public API surfaces, autosave, validation envelopes |
| [RFX_V3_STATE_MACHINES.md](./RFX_V3_STATE_MACHINES.md) | RFx, response, and publication lifecycles |
| [RFX_V3_QUESTIONNAIRE_ENGINE.md](./RFX_V3_QUESTIONNAIRE_ENGINE.md) | Questionnaire definition, validation layers, scoring input rules |
| [RFX_V3_UX.md](./RFX_V3_UX.md) | Buyer Studio & carrier workspace UX contracts |
| [RFX_V3_SECURITY.md](./RFX_V3_SECURITY.md) | Tenancy, provenance, audit, preview isolation |
| [RFX_V3_ROADMAP.md](./RFX_V3_ROADMAP.md) | Implementation waves and non-deferrable capabilities |
| **[RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)** | **Normative:** server-authoritative validation, valid-only persistence, draft/autosave/resume, carrier error UX, preview safety, submit/publish gates, versioned post-publication edits |

### Architecture Decision Records

| ADR | Decision |
|---|---|
| [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md) | Authoritative response state contains valid persisted data only |

---

## Normative contracts

The following flags are **non-negotiable** across v3.0A (detail in validation contract §22):

```text
INVALID_DATA_PERSISTENCE=FORBIDDEN
SERVER_VALIDATION_REQUIRED=YES
SAVE_ONLY_VALID_STATE=YES
LAST_VALID_SERVER_STATE_PRESERVED=YES
FIELD_ERROR_INLINE=YES
GLOBAL_ERROR_SUMMARY=YES
ERROR_DEEP_LINK=YES
BUYER_AUTOSAVE=YES
CARRIER_RESPONSE_AUTOSAVE=YES
PREVIEW_AS_CARRIER=YES
SUBMIT_WITH_ERRORS=FORBIDDEN
PUBLISH_WITH_ERRORS=FORBIDDEN
KNOCKOUT_BLOCKS_SAVE=NO
```

When documents conflict, **`RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md`** is authoritative for validation, draft, autosave, and error UX semantics.

---

## Related platform docs

- `docs/engineering/VALIDATION_LEVELS.md`
- `packages/openapi/` — error envelope schemas
- Existing RFx v1 service boundaries: `services/rfx-service/`

---

**Document control:** Architecture changes require review before implementation tasks are authorized.
