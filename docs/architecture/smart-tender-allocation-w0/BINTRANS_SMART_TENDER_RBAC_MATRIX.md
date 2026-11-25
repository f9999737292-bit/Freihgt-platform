# BINTRANS Smart Tender — RBAC / Commission / Commitments (Spec v0.9)

**Main:** `892361db87985f35132aef86550a025909af648a`  
**Existing roles:** `000009` + `FORWARDER_MANAGER` (`000013`)  
**Existing RFx policies:** `buyerManageRoles` / `carrierRespondRoles` in `api-gateway/.../rfxrbac/policies.go`  
**New API authz:** under `/api/v1/rfx/allocations` (Decision D6)

---

## Permissions (server-side)

tender.read · tender.round.create · tender.round.approve · tender.bid.evaluate ·  
allocation.policy.manage · allocation.simulate · allocation.proposal.create · allocation.approve · allocation.override ·  
commission.vote · commission.admin ·  
carrier.commitment.respond · carrier.commitment.sign_authorize ·  
contract.appendix.prepare · contract.appendix.send_edo · contract.appendix.view_signed ·  
allocation.activate · allocation.reallocate · allocation.audit.read  

Checks always include tenant_id, company_id, object, permission. No frontend-authoritative roles.

Prefer permissions + membership over inventing duplicate global role names for every TZ §3.1 title.

---

## Forbidden combinations

- Commission vote ≠ carrier sign authority  
- Bid view ≠ commission vote  
- Ops logistician ≠ legal guarantee change  
- Platform admin ≠ act as client in commercial approval without controlled override + audit  

---

## Commission FSM

DRAFT → SUBMITTED → IN_REVIEW → APPROVED | REJECTED  
Also: CHANGES_REQUESTED, SUPERSEDED, CANCELLED, EXPIRED  

Policies: ALL_REQUIRED | QUORUM | ROLE_MANDATORY | SEQUENTIAL | PARALLEL | HYBRID  

Vote: actor_id, company_id, permission proof, **subject_hash** (= decision snapshot), decision, timestamp, comment, audit digest.  
Material change → new workflow; no silent vote carry-over without approved equivalence rule.

---

## Carrier commitment FSM

PROPOSED → VIEWED → ACCEPTED | PARTIALLY_ACCEPTED | CHANGE_REQUESTED | REJECTED | EXPIRED  
Then AGREED → DOCUMENT_PENDING → SIGNATURE_PENDING → EFFECTIVE (gates only)  

Partial: new version; budgets recalculated; maybe re-commission; freed volume → reallocation.  
`sign_authorize` separate from commercial respond. Silence ≠ consent (default).

---

## Guarantees

Types: TARGET_VOLUME, MAXIMUM_VOLUME, SHIPPER_VOLUME_COMMITMENT, CARRIER_CAPACITY_COMMITMENT, CARRIER_ACCEPTANCE_COMMITMENT, MUTUAL_VOLUME_COMMITMENT.  
Units: %, trips, trips/day|week|month, calendar, tons if contract-defined.  
No auto equal daily split of monthly guarantee. Execution buckets as Spec §14.

---

## EDO boundary

Immutable appendix content_hash; idempotent send; trusted evidence only.  
No live signing / EFFECTIVE from UI until document P0 + legal + Controller GO.
