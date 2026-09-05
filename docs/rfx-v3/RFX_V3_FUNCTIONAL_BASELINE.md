# RFx v3.0A — Functional Baseline

**Status:** Architecture draft  
**Source:** Controller-reviewed Transporeon RFI video (functional patterns only)  
**Restriction:** No UI, proprietary text, screens, graphics, or source code reproduced.

---

## 1. Purpose

Establish a functional baseline for enterprise supplier qualification RFx and document BINTRANS v3.0A differentiators.

---

## 2. Baseline functional patterns (industry RFI reference)

The following capabilities are **expected** in enterprise supplier RFx platforms:

| Area | Baseline capability |
|---|---|
| Multi-stage RFI | Qualification before commercial engagement |
| Basic / cover information | Header metadata, deadlines, contacts |
| Question catalogue | Reusable question library |
| Sections | Grouped questionnaire structure |
| Multiple question types | Text, number, boolean, select, file, date, … |
| Mandatory answers | Required field enforcement |
| Scoring | Weighted evaluation of responses |
| Knockout | Disqualification rules |
| Supplier selection / groups | Targeted invitation lists |
| Access control | Role-based buyer and supplier access |
| Attachments | Document upload with validation |
| Communications | Buyer–supplier messaging around tender |
| Reminders | Deadline and incomplete response reminders |
| Preview-as-carrier | Buyer tests supplier experience |
| Draft / save / continue | Long-form response persistence |
| Editing | Draft editing before submit; controlled post-submit |

These patterns inform v3.0A architecture but are **not** copied from any vendor implementation.

---

## 3. BINTRANS v3.0A differentiators

| Differentiator | BINTRANS approach |
|---|---|
| **Modern RFx Studio** | Unified buyer workspace with autosave safety |
| **Autosave safety** | Valid-only persistence; last valid server state preserved |
| **Carrier 360** | Reuse verified fleet, docs, KPI — reduce duplicate entry |
| **Conditional logic** | Native rule engine (not bolt-on custom fields) |
| **Actual logistics KPI** | Shipment/on-time/cancellation metrics in scoring |
| **Answer provenance** | Full source chain: carrier, document, operational, AI |
| **Explainable scoring** | Every point traceable to rule, weight, input |
| **AI (bounded)** | Assist only — no autonomous publish/reject |
| **Qualification pool** | Reusable prequalified carrier sets |
| **End-to-end lifecycle** | RFI → RFQ → Award → Execution → KPI feedback loop |

---

## 4. Lifecycle mapping

```text
RFI (qualify)
  → RFQ/RFP (commercial + technical)
    → Evaluation + scoring + knockout
      → Award
        → Transport order / execution
          → KPI capture → Carrier 360 refresh → next RFI cycle
```

Current repository: RFI/RFQ/RFP types exist (`000004`); full lifecycle architecture defined in v3 docs; execution link partial via award → transport order (`000039`).

---

## 5. Mandatory v3.0A gates (above baseline)

From validation contract — not optional add-ons:

| Gate | Flag |
|---|---|
| Server validation | `SERVER_VALIDATION_REQUIRED=YES` |
| Valid-only save | `SAVE_ONLY_VALID_STATE=YES` |
| Submit without errors forbidden | `SUBMIT_WITH_ERRORS=FORBIDDEN` |
| Publish without errors forbidden | `PUBLISH_WITH_ERRORS=FORBIDDEN` |
| Preview isolation | `PREVIEW_DATA_NOT_IN_PRODUCTION_ANSWER=YES` |

---

## 6. References

- [RFX_V3_GAP_MATRIX.md](./RFX_V3_GAP_MATRIX.md) — current vs target
- [RFX_V3_UX.md](./RFX_V3_UX.md) — carrier workspace UX
- [RFX_V3_ROADMAP.md](./RFX_V3_ROADMAP.md) — v3.0A–J waves
