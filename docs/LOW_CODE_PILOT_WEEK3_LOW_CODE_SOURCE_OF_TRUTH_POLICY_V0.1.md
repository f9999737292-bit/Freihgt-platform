# Low-code Pilot Week-3 Low-code Source-of-Truth Policy v0.1

## Summary

Defines **financial/legal source-of-truth boundaries** for low-code custom fields in the controlled pilot (PR-GAP-010). **Docs-only** — no code or config changed.

**Decision:** **SOURCE_OF_TRUTH_POLICY_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

**PR-GAP-010:** **SOURCE_OF_TRUTH_POLICY_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

## Purpose

Confirm that low-code custom fields are **advisory** unless separately approved as source of truth for financial or legal decisions.

## Scope

- Low-code **custom field values** on TRANSPORT_ORDER, SHIPMENT, BILLING_REGISTER, and related entity types
- Does **not** close PR-GAP-010 without named Product/Legal/Finance owner and final approval

## Current Status

| Field | Value |
|-------|-------|
| Policy Owner | **TBD** |
| Approval | **pending** |
| Production-ready claimed | **no** |
| Controlled pilot | **CONTROLLED_PILOT_APPROVED** — active |
| Code changed | **no** |

## Source-of-Truth Rules

| Rule | Description |
|------|-------------|
| **Advisory by default** | Low-code custom fields are **not** financial or legal source of truth unless explicitly approved |
| **Core systems authoritative** | Core billing register status, payment status, and billing services API remain authoritative |
| **validation_context advisory** | Runtime validation context fields are operational hints only — not financial SoT |
| **Signed legal documents excluded** | Signed contracts, legal filings, and payment instruments are **outside** low-code SoT scope unless separately approved |
| **Payment data excluded** | Raw payment card/bank data must **not** be stored in low-code fields |
| **Operator briefing** | BILLING_REGISTER operator quick guide documents advisory role — reference Week-2 BR materials |

## Entity Scope

| Entity | Low-code role | Authoritative SoT |
|--------|---------------|-------------------|
| TRANSPORT_ORDER | Operational custom fields (advisory) | Core transport order services |
| SHIPMENT | Operational custom fields (advisory) | Core shipment services |
| BILLING_REGISTER | Custom fields (advisory) | Core billing register + billing API |

## Forbidden Without Approval

- Using low-code fields as **sole** basis for invoicing, payment release, or legal compliance
- Treating custom field values as **contractual** or **legally binding** without separate approval
- Storing signed legal documents or payment credentials in low-code fields
- Claiming production-ready based on this policy draft alone

## Evidence Rules

| Allowed | Forbidden |
|---------|-----------|
| Policy doc references | JWT / tokens / passwords |
| Entity type names | Raw production financial payloads |
| Advisory/advisory-only labels | Personal production data dumps |

## Owner Requirements

Before PR-GAP-010 closure:

1. Named **Product / Legal / Finance** owner assigned
2. Policy reviewed and explicitly approved
3. Final owner sign-off in Source-of-Truth Owner Approval Pack v0.1

**Assigned owner:** **Product / Legal / Finance — TBD**

## Decision

**SOURCE_OF_TRUTH_POLICY_PACK_CREATED_PENDING_OWNER_ASSIGNMENT**

## Next Steps

1. **Low-code Pilot Week-3 Source-of-Truth Owner Approval Pack v0.1**
2. Assign named policy owner
3. Do **not** change runtime code or billing logic in this pack

Reference: `LOW_CODE_PILOT_WEEK3_LOW_CODE_SOURCE_OF_TRUTH_OWNER_NOTE_V0.1.md`, `LOW_CODE_PILOT_WEEK2_BILLING_REGISTER_WRITE_MONITORING_V0.1.md`
