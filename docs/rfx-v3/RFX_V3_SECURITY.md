# RFx v3.0A — Security & Audit

**Status:** Architecture draft  
**Normative companion:** [RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md](./RFX_V3_RESPONSE_VALIDATION_AND_DRAFT_SAFETY.md)

---

## 1. Trust boundary

- External clients authenticate at API Gateway (JWT Bearer).
- Tenant and user identity from gateway-verified headers only.
- **Company context is server-verified** — never trust client-supplied company ID as authority.
- Carrier response writes scoped to participant company membership resolved server-side.
- Buyer writes scoped to RFx owner company / authorized buyer roles resolved server-side.

Mandatory flags:

```text
CLIENT_SUPPLIED_COMPANY_AUTHORITY=FORBIDDEN
TENANT_AUTHORITY=SERVER_VERIFIED
USER_AUTHORITY=SERVER_VERIFIED
COMPANY_AUTHORITY=SERVER_VERIFIED
CROSS_COMPANY_SPOOF=DENIED
CROSS_TENANT_SPOOF=DENIED
```

If `X-Company-ID` is used internally, it may only reflect membership validated from authenticated user + tenant + participant/owner authorization. Mismatch → `403`.

---

## 2. Answer provenance (`AnswerProvenance`)

Every **accepted production** persisted answer records:

| Field | Required |
|---|---|
| `answer_value` | Yes |
| `answer_version` | Yes |
| `answer_source` | Yes — authoritative sources only (see §2.1) |
| `updated_by` | Yes |
| `updated_at` | Yes |
| `validation_version` | Yes |

### 2.1 Authoritative `answer_source` values

| Source | Allowed on production `Answer` |
|---|---|
| `CARRIER_DECLARED` | Yes |
| `CARRIER_PROFILE` | Yes |
| `DOCUMENT_VERIFIED` | Yes |
| `BINTRANS_OPERATIONAL_DATA` | Yes |
| `BUYER_REVIEW` | Yes (audited) |
| `EXTERNAL_VERIFICATION` | Yes |
| `AI_EXTRACTED_PENDING_REVIEW` | Yes (until confirmed) |
| `BUYER_PREVIEW_TEST` | **NO — forbidden** |

Preview sandbox data uses ephemeral/isolated storage tagged `PREVIEW_DATA_ONLY=YES`. It must never appear as production provenance.

Qualification-relevant answers also:

| Field | Required |
|---|---|
| `rule_version` | Yes |
| `score_model_version` | Yes |

Invalid client drafts and preview sandbox values **must not** appear as production provenance.

---

## 3. Preview & test isolation

| Rule | Value |
|---|---|
| `PREVIEW_DATA_ONLY` | YES |
| `REAL_RESPONSE_CREATED` | NO |
| `PREVIEW_DATA_NOT_IN_PRODUCTION_ANSWER` | YES |
| Preview answers in audit/scoring history | **Excluded** |
| `BUYER_PREVIEW_TEST` on production `Answer` | **FORBIDDEN** |

Preview sessions use ephemeral or isolated preview storage; cannot be promoted to carrier evidence without a real authenticated carrier response flow.

---

## 4. Validation authority

| Rule | Value |
|---|---|
| `SERVER_VALIDATION_REQUIRED` | YES |
| Client-only validation as authority | **FORBIDDEN** |
| Partial persist of invalid batch | **FORBIDDEN** |

422 responses must not leak stack traces, SQL, or internal service identifiers.

---

## 5. Audit & historical integrity

- Published `RfxVersion` records are immutable.
- Knockout/disqualification evidence must remain auditable (`KNOCKOUT_ANSWER_PERSISTED=YES`).
- Rule/scoring model changes require new calculation version; no silent rewrite of historical answers.

Material post-publish changes require `ChangeImpactAnalysis` audit event.

---

## 6. Local recovery boundary

If browser local recovery is implemented:

```
LOCAL_RECOVERY_IS_NOT_AUTHORITATIVE = YES
```

Restored local values must re-pass server validation before persistence.

---

## 7. Tenant isolation

All response and answer queries predicate on trusted `tenant_id`. Cross-tenant access denied at gateway and service layers.

---

## 8. Mandatory flags (security-relevant subset)

```text
INVALID_DATA_PERSISTENCE=FORBIDDEN
SERVER_VALIDATION_REQUIRED=YES
SAVE_ONLY_VALID_STATE=YES
LAST_VALID_SERVER_STATE_PRESERVED=YES
KNOCKOUT_BLOCKS_SAVE=NO
PREVIEW_DATA_ONLY=YES (preview contexts)
SUBMIT_WITH_ERRORS=FORBIDDEN
PUBLISH_WITH_ERRORS=FORBIDDEN
```

Full list: validation contract §22.

---

## 9. References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md)
- [RFX_V3_API.md](./RFX_V3_API.md)
- [ADR-RFX-011](./adr/ADR-RFX-011-RESPONSE-VALIDATION-AND-DRAFT-SAFETY.md)
- Platform rules: `.cursor/rules/20-security-tenancy.mdc`
