# RFx v3.0A — Carrier 360

**Status:** Architecture draft  
**Goal:** Carriers should not repeatedly enter verified information across RFx responses.

---

## 1. Purpose

Carrier 360 aggregates reusable carrier data from platform sources for autofill, verification, and scoring input — with explicit provenance, freshness, and buyer visibility rules.

---

## 2. Data domains

| Domain | Source services / tables | RFx use |
|---|---|---|
| **Company profile** | `company-service` | Legal name, tax ID, contacts |
| **Fleet** | Company assets / future fleet module | Vehicle count, types, age |
| **Drivers** | Identity / HR integrations | Licensed drivers, ADR certs |
| **Documents** | `document-service` | Licenses, insurance scans |
| **Insurance** | Document + expiry metadata | Coverage limits, validity |
| **Certificates** | Document classifications | ISO, ADR, GDP |
| **Shipment history** | `shipment-service` | Lanes served, volume |
| **SLA / KPI** | Control Tower / operational metrics | On-time %, lead time |
| **POD** | Shipment documents | Delivery proof rate |
| **Claims** | Claims module (future) | Claim frequency |
| **Acceptance rate** | RFx + bid history | Tender win/respond ratio |
| **Cancellation rate** | Shipment cancellations | Reliability signal |
| **Incidents** | HSE / incident registry | Safety record |

**Current repository state:** Company list exists; aggregated Carrier 360 API is **ABSENT** (see [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md)).

---

## 3. Provenance model

Each Carrier 360 field carries:

| Attribute | Meaning |
|---|---|
| `source` | `CARRIER_DECLARED`, `DOCUMENT_VERIFIED`, `BINTRANS_OPERATIONAL_DATA`, `EXTERNAL_VERIFICATION`, `AI_EXTRACTED_PENDING_REVIEW` |
| `source_record_id` | FK to originating entity |
| `captured_at` | When value was recorded |
| `verified_at` | When verification completed (nullable) |
| `verified_by` | User or system process |
| `confidence` | 0–1 for AI/extracted values |

When autofilled into RFx answers, `answer_source` reflects provenance (see domain model §4.4).

---

## 4. Freshness & expiry

| Rule | Behavior |
|---|---|
| Document expiry | Field marked `STALE` after expiry date |
| KPI windows | Rolling window (e.g. 90-day on-time %) |
| Re-verification | Prompt carrier confirm if stale before submit |
| Buyer visibility | Show freshness badge + source icon |

Stale data may prefill but triggers `WARNING` or re-confirmation requirement.

---

## 5. Carrier confirmation

Before submit, carrier must confirm autofilled values:

| State | UX |
|---|---|
| `AUTO_FILLED` | Read-only with «Подтвердить» action |
| `CONFIRMED` | Carrier accepted; `answer_source=CARRIER_PROFILE` |
| `MODIFIED` | Carrier edited; `answer_source=CARRIER_DECLARED` |
| `REJECTED` | Carrier declined autofill; manual entry required |

Confirmation is audited (`updated_by`, `updated_at`).

---

## 6. Buyer visibility

| Data class | Buyer sees |
|---|---|
| Confirmed profile fields | Value + source + freshness |
| Operational KPI | Aggregated metrics only (no PII) |
| Pending AI extraction | «Требует проверки» — not scored until confirmed |
| Internal carrier notes | Hidden |

Conflict handling: when Carrier 360 and response answer diverge, evaluation UI shows both with provenance; buyer manual review path available.

---

## 7. Conflict handling

| Scenario | Resolution |
|---|---|
| Document says 50 trucks; carrier enters 45 | Flag conflict; prefer confirmed response answer; audit both |
| KPI improves after response submit | Re-score only on explicit recalculation |
| Cross-tenant data | **Denied** — tenant isolation on all queries |

---

## 8. Integration with scoring

`SYSTEM_DERIVED` scoring mode reads Carrier 360 operational KPI with:

- Snapshot at submit time
- `score_model_version` binding
- Explainability payload citing KPI source and window

See [RFX_V3_SCORING_ENGINE.md](./RFX_V3_SCORING_ENGINE.md).

---

## 9. References

- [RFX_V3_DOMAIN_MODEL.md](./RFX_V3_DOMAIN_MODEL.md) — `AnswerProvenance`
- [RFX_V3_DATA_MODEL.md](./RFX_V3_DATA_MODEL.md)
- [ADR-RFX-006](./adr/ADR-RFX-006-CARRIER-360-AUTOFILL.md)
- [ADR-RFX-007](./adr/ADR-RFX-007-ANSWER-PROVENANCE.md)
