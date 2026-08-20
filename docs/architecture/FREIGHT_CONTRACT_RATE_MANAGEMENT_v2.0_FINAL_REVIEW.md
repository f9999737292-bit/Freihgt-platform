# FREIGHT CONTRACT & RATE MANAGEMENT v2.0
## FINAL ARCHITECTURE REVIEW

**Review date:** 2026-08-20  
**Reviewer mode:** Independent repository verification + architecture remediation (documentation only)  
**Repository:** `f9999737292-bit/Freihgt-platform`  
**Worktree:** `D:\Projects\freight-platform-wt\contract-rate-v2.0-final-review`

---

### GIT

| Field | Value |
|-------|-------|
| `ORIGIN_MAIN_SHA` | `a4471727239372840707e5ee5ef8aa882c826636` |
| `ARCH_BRANCH` | `arch/freight-contract-rate-management-v2.0` |
| `ARCH_HEAD_SHA_REVIEWED` | `d6ef3553efac674d3f67562a7d7c67a3c01b5933` |
| `REVIEW_BRANCH` | `review/freight-contract-rate-management-v2.0` |
| `FINAL_REVIEW_SHA` | *(set at commit)* |
| `WORKTREE_CLEAN` | YES (at review start) |
| `BASE_IS_ARCH_PR_HEAD` | YES |

Architecture branch HEAD matched expected SHA at review start; no drift.

---

### REVIEW SCOPE

Independent verification of **Freight Contract & Rate Management v2.0** architecture (`docs/engineering/FREIGHT_CONTRACT_RATE_MANAGEMENT_v2.0_ARCHITECTURE.md`) against merged main implementation at `a447172`. Reviews A–T, threat model, adversarial scenarios, slice dependency, financial integrity, and tenant security. **No runtime code, migrations, or frontend.**

---

### REPOSITORY DISCOVERY

| Area | Status | Evidence |
|------|--------|----------|
| `contract-rate-service` | **NOT_FOUND** | No `services/contract-rate-service/` |
| Contract / rate master aggregates | **NOT_FOUND** | No contract/rate tables in migrations |
| TO price / snapshot persistence | **NOT_FOUND** | `transport.transport_orders` has no amount/snapshot columns |
| Shipment price duplication | **NOT_FOUND** | `transport.shipments` — no price columns |
| RFx bid pricing | **FOUND** | `rfx.bids`, `rfx.bid_items` — `float64` domain |
| Formal award → TO link | **FOUND** | `rfx.rfx_award_transport_orders` migration `000039` |
| Settlement base freight | **FOUND** | `LoadShipmentContext` reads award link only |
| Payment exact decimal | **FOUND** | `payment-service/internal/domain/money.go` — `shopspring/decimal`, `MoneyScale=2` |
| Billing/settlement legacy float | **FOUND** | `billing-register-service` — `float64`, `round2()` |
| Canonical locations | **FOUND** | `transport.locations` — UUID PK, `tenant_id` |
| Equipment type | **PARTIAL** | Nullable `*string` on TO/RFx; no enum service |
| Generic TO idempotency | **NOT_FOUND** | No `Idempotency-Key` on TO create |
| Award conversion retry | **PARTIAL** | Scope-level idempotent return in `award_conversion_repository.go` |
| Shared money package | **NOT_FOUND** | Per-service money handling |

---

### FINDINGS

#### BLOCKER

| ID | Finding | Remediation |
|----|---------|-------------|
| — | None after review remediations | — |

#### HIGH

| ID | Finding | Remediation |
|----|---------|-------------|
| H-01 | Equipment type marked required for matching but TO/RFx allow NULL with no normalization rule | **FIXED** — §9.4 added: TRIM, case-sensitive, fail closed on blank |
| H-02 | PERCENT fuel rounding order not fully deterministic | **FIXED** — §11.2.1: round base, then percent, then sum total |
| H-03 | v2.0A/v2.0B REST endpoints listed while RBAC/gateway deferred to v2.0E — unsafe public exposure risk | **FIXED** — v2.0A/v2.0B slices require internal S2S-only until v2.0E |

#### MEDIUM

| ID | Finding | Remediation |
|----|---------|-------------|
| M-01 | `buyer_company_id` vs `shipper_company_id` mapping implicit | **FIXED** — §20.2 platform field mapping table |
| M-02 | Contract termination effective-date semantics underspecified | **FIXED** — §7.6–7.7 immediate TERMINATED + transition table |
| M-03 | Observability minimum not in architecture | **FIXED** — §31 Observability |
| M-04 | Physical DELETE semantics not explicit | **FIXED** — §30 Delete/Mutability |
| M-05 | Generic TO create idempotency absent (blocks v2.0C, not v2.0A) | Documented in architecture §16.2; v2.0C gate |

#### LOW

| ID | Finding | Notes |
|----|---------|-------|
| L-01 | Legacy billing settlement loader uses `float64` + cross-schema read of `rfx.*` | Acceptable during transition; v2.0C migrates to snapshot |
| L-02 | OpenAPI legacy schemas use JSON number for money | contract-rate v2.0E must use decimal strings |
| L-03 | `pricing_date` tenant TZ policy TBD | Implementation detail in v2.0B |

#### NIT

| ID | Finding |
|----|---------|
| N-01 | Architecture adds `CANCELLED` draft state — valid, not in minimal checklist |
| N-02 | `docs/architecture/README.md` service map outdated (no contract-rate-service) — update when service exists |

---

### BOUNDED CONTEXT

**`BOUNDED_CONTEXT_REVIEW=PASS`**

| Aggregate / concern | SSOT service | Notes |
|---------------------|--------------|-------|
| TransportContract lifecycle | **contract-rate-service** | NOT_FOUND today — new service justified |
| RateCard / RateCardVersion | **contract-rate-service** | |
| RateLine / RateComponent | **contract-rate-service** | |
| RateResolution (compute) | **contract-rate-service** | Read-only algorithm |
| RateSnapshot (persist) | **transport-order-service** | CR resolves; TO insert-only |
| PricingSource facts (RFx) | **rfx-service** | Internal API only |
| TO execution | **transport-order-service** | |
| Settlement execution accessorials | **billing-register-service** | |
| Billing register | **billing-register-service** | No rate resolution |
| Payment obligations | **payment-service** | No rate resolution |

**Rejected patterns verified absent:** no duplicated contract SSOT, no settlement-as-resolver, no payment-as-resolver, no RFx-as-contract-master.

**Cross-service today (legacy):** settlement reads `rfx.rfx_award_transport_orders` directly — **CONTRADICTS target boundary** but documented as migration-only fallback in v2.0C.

---

### MONEY MODEL

**`MONEY_MODEL_REVIEW=PASS`**  
**`MONEY_CANONICAL_TYPE=shopspring/decimal` (scale 2) + PostgreSQL NUMERIC(18,2)**  
**`ROUNDING_RULE=HALF_UP per-component then HALF_UP total (§11.2.1)`**

| Layer | Finding | Evidence |
|-------|---------|----------|
| PostgreSQL | **FOUND** NUMERIC(18,2) | `000039_rfx_award_transport_order_v1.4.up.sql`, `000004_create_rfx_tables.up.sql` |
| payment-service | **FOUND** exact decimal | `money.go`: `MoneyScale=2`, `RoundMoney`, `ValidateMoneyScale` |
| billing-register-service | **FOUND** float64 legacy | `billing_register_item.go:round2`, `freight_settlement_repository.go` scans `amount::float8` |
| rfx-service | **FOUND** float64 legacy | `bid.go`: `TotalAmount float64` |
| shared-go money | **NOT_FOUND** | |
| contract-rate v2.0 | **NOT_FOUND** (future) | Architecture forbids float64 — coherent |

**Frozen v2.0 rule:** contract-rate-service must not introduce float64 canonical money. JSON/API decimal strings required (§11.2.1).

---

### CONTRACT LIFECYCLE

**`CONTRACT_LIFECYCLE=PASS`**

States: `DRAFT`, `ACTIVE`, `SUSPENDED`, `TERMINATED`, `EXPIRED`, `CANCELLED` (draft abandon).

| Question | Answer |
|----------|--------|
| DRAFT editable? | YES — all mutable fields + draft rate versions |
| ACTIVE editable? | Metadata only — no pricing row edits |
| ACTIVE suspendable? | YES |
| Reactivatable? | YES from SUSPENDED if not EXPIRED |
| Termination | Immediate final; snapshots unchanged |
| Expired contracts resolve? | NO — fail closed |
| `valid_to` reached | System → EXPIRED (final) |

Full transition table: architecture §7.7 (added in remediation).

---

### RATE VERSIONING

**`ONE_ACTIVE_VERSION_DB_ENFORCED=YES`** (proposed partial unique index)  
**`RATE_VERSION_CONCURRENCY=PASS`**  
**`TEMPORAL_OVERLAP_POLICY=One ACTIVE version per RateCard at any instant; SUPERSEDED versions retained read-only; pricing_date must fall within ACTIVE version [valid_from, valid_to]; no future-scheduled ACTIVE in v2.0 MVP; cross-card duplicate lane scope forbidden at activation`

| Mechanism | Status |
|-----------|--------|
| Partial UNIQUE `(rate_card_id) WHERE status='ACTIVE'` | Proposed §25.4 — not yet migrated |
| `SELECT FOR UPDATE` on activation | Documented §8.5 |
| Idempotent duplicate activate | Documented §8.5, §26 |

**Concurrency scenarios:**

| Scenario | Expected |
|----------|----------|
| Two simultaneous activations | One wins; other sees locked row or idempotent state |
| Retry same activation | Idempotent ACTIVE return |
| Activate while another ACTIVE | Supersede in same TX |
| Backdated/future version dates | Eligibility via `pricing_date` window; still one ACTIVE |

---

### LANE MATCHING

**`LANE_MATCHING=PASS`** (after §9.4 remediation)

| Question | Answer | Evidence |
|----------|--------|----------|
| Location UUIDs tenant-scoped? | YES | `transport.locations.tenant_id` — migration `000003` |
| Cross-tenant UUID collision? | NO — queries filter `tenant_id` | TO repo, settlement loader |
| equipment_type canonical? | **PARTIAL** — free string, nullable today | TO/Rfx domain |
| Normalization defined? | YES (post-remediation) | §9.4 TRIM, case-sensitive |
| Empty equipment? | Fail closed `INVALID_EQUIPMENT_TYPE` | §9.4 |
| RFx same semantic? | **PARTIAL** — same field, nullable | `rfx_lot.go` |
| TO same semantic? | **PARTIAL** | `transport_order.go` |
| transport_mode ROAD? | YES for v2.0 MVP | TO default ROAD |
| Direction-sensitive? | YES — origin ≠ destination swap |

No fuzzy geography in v2.0 — confirmed.

**Party mapping:** `buyer_company_id` = `shipper_company_id`; `consignee_company_id` excluded from matching.

---

### RATE RESOLUTION

**`RATE_RESOLUTION_DETERMINISTIC=PASS`**  
**`AMBIGUITY_FAILS_CLOSED=YES`**

Precedence frozen:

```
RFQ_AWARD / ACCEPTED_SPOT_BID → CONTRACT_RATE → MANUAL_SPOT_FALLBACK → RATE_NOT_FOUND
```

| Rule | Status |
|------|--------|
| Explicit award/bid wins when linked | YES |
| Invalid explicit source — no contract fallback | YES §12.2, §15.4 |
| Multiple equal candidates → RATE_AMBIGUOUS | YES §9.3 |
| No arbitrary LIMIT 1 | YES |
| Currency mismatch → fail | YES (PRICING_SOURCE_MISMATCH / RATE_CURRENCY_MISMATCH family) |
| Client monetary values authoritative | NO |

---

### RFX BOUNDARY

**`RFX_SERVICE_BOUNDARY=PASS`**  
**`CROSS_SCHEMA_SQL=DENY`** (target for contract-rate-service)

contract-rate-service needs from rfx-service internal API (§15.4): tenant, source type/id, parties, lane, equipment, mode, currency, total_amount, optional base/components, status.

Aggregate-only invariant preserved: `base_amount` unknown → NULL; never infer from total.

**Failure when RFx unavailable with explicit linked source:** fail closed (`SOURCE_NOT_FOUND` / upstream error) — **no silent contract fallback**.

**Legacy note:** billing-register **currently** reads `rfx.*` cross-schema — v2.0C retires this for new TOs with snapshots.

---

### RATE SNAPSHOT

**`SNAPSHOT_SELF_CONTAINED=YES`**  
**`SNAPSHOT_IMMUTABLE=YES`**

Owner: TO persists full copy (`transport.transport_order_rate_snapshots` proposed §14.2).  
Minimum fields justified in §14.4. Aggregate-only: `base_amount=NULL`, `components=[]`, `component_breakdown_status=UNAVAILABLE`.

---

### TO ATOMICITY / IDEMPOTENCY

**`TO_PRICING_ATOMICITY=PASS`** (design — single TX in TO for order + snapshot)  
**`TO_IDEMPOTENCY_DESIGN=PASS`** (design — v2.0C requirement)

| Mechanism | Current | Target v2.0C |
|-----------|---------|--------------|
| Generic TO Idempotency-Key | **NOT_FOUND** | Required |
| Snapshot UNIQUE (tenant, transport_order_id) | **NOT_FOUND** | Required INSERT-only |
| Award conversion scope idempotency | **PARTIAL FOUND** | Reuse where applicable |

Partial failure risk (TO without snapshot) mitigated by single DB transaction in transport-order-service — not cross-service atomic TX.

---

### SETTLEMENT INTEGRATION

**`SETTLEMENT_INTEGRATION=PASS`**  
**`FUEL_DOUBLE_COUNT=DENY`**  
**`HISTORICAL_REPRICING=DENY`**

Current: `freight_settlement_repository.go:LoadShipmentContext` — award link `amount::float8` → `base_freight_amount`.

Target: `base_freight_amount = snapshot.total_amount` (agreed pre-execution freight including contractual fuel).

Transition precedence (v2.0C):

| TO type | Base freight source |
|---------|---------------------|
| New TO with snapshot | `transport_order_rate_snapshots.total_amount` |
| Legacy TO with award link | `rfx_award_transport_orders.amount` (fallback) |
| Legacy TO without either | Fail closed — no repricing from latest contract |

Formula: `settlement_total = snapshot.total_amount + approved_execution_accessorials ± adjustments`

---

### ACCESSORIAL BOUNDARY

**`ACCESSORIAL_BOUNDARY=PASS`**

Contract-rate owns unit **rules** (e.g. waiting RUB/hour). Settlement owns approved qty, approval, final charge. Settlement must not read live contract tables for historical closing — snapshot optional contracted unit rates in v2.0C+.

---

### TENANT ISOLATION

**`TENANT_ISOLATION_DESIGN=PASS`**

All proposed contract_rate tables include `tenant_id`. Queries filter verified auth tenant. Composite uniqueness `(tenant_id, …)`. Location FKs validated same-tenant on write. No client-trusted tenant header alone (gateway/auth pattern from payment-service `resolveVerifiedTenant`).

---

### PARTY / RBAC MODEL

**`PARTY_MODEL=PASS`**  
**`RBAC_MODEL_IMPLEMENTABLE=YES`**

Companies + memberships: `company-service`. PLATFORM_ADMIN: payment/rfx/billing patterns (`PLATFORM_ADMIN` role checks). Permission table §21 implementable via identity extension in v2.0A registry + full gateway wiring v2.0E.

v2.0A/v2.0B: **internal S2S only** until gateway RBAC — prevents unsecured intermediate exposure.

---

### AUDITABILITY

**`AUDITABILITY=PASS`**

Append-only `contract_rate.audit_event` for contract/rate mutations + resolution correlation. Required events §22.1. Historical price explanation: who/when/source/version/amount via snapshot + audit.

---

### DATABASE MODEL

**`DB_MODEL=PASS`**  
**`FINANCIAL_HISTORY_DELETION=DENY`**

UUID PKs, tenant indexes, partial unique ACTIVE version, NUMERIC(18,2), CHECK constraints, soft lifecycle, no cascade delete of commercial history. §30 delete semantics added.

---

### MUTABILITY RULES

**`MUTABILITY_RULES=PASS`**

Activated contract/version/snapshot: no product DELETE; INSERT-only snapshots; new version for pricing changes.

---

### FAILURE SEMANTICS

**`FAIL_CLOSED_PRICING=YES`**

| Condition | Behavior |
|-----------|----------|
| Rate not found | Reject TO create (default) |
| Multiple rates | RATE_AMBIGUOUS |
| Contract expired/suspended | Not eligible — fail |
| RFx unavailable (explicit source) | Fail — no fallback |
| Currency mismatch | Fail |
| Invalid lane/equipment | Fail |
| Manual unauthorized | FORBIDDEN |
| Snapshot/TO persistence failure | Roll back TX — no half-priced TO |
| Silent price = 0 | **DENY** |

---

### OBSERVABILITY

**`OBSERVABILITY_DESIGN=PASS`** (§31 added)

Minimum metrics defined; no sensitive high-cardinality labels.

---

### ROLLOUT / BACKWARD COMPATIBILITY

**`ROLLOUT_COMPATIBILITY=PASS`**

Safe sequence:

1. contract-rate schema/service  
2. contracts/rates  
3. resolver  
4. TO snapshot  
5. settlement snapshot consumption  
6. legacy retirement later  

No forced historical repricing. Legacy award-link path remains for pre-v2.0C orders.

---

### SLICE DEPENDENCY REVIEW

**`SLICE_PLAN_IMPLEMENTABLE=YES`**

```
v2.0A (contract/rate core, DB one-active invariant, S2S-only API)
  ↓
v2.0B (rate lines + resolution, S2S-only resolve)
  ↓
v2.0C (RFx adapter + TO snapshot + settlement + TO idempotency)
  ↓
v2.0D (workspace UI — depends v2.0B read APIs via gateway)
  ↓
v2.0E (OpenAPI merge, public gateway routes, RBAC, E2E)
```

**Security note:** Public gateway exposure deferred to v2.0E; v2.0A/B must not register public routes without auth.

---

### THREAT MODEL

| ID | Threat | Impact | Control | Residual | Verdict |
|----|--------|--------|---------|----------|---------|
| RATE-T01 | Cross-tenant rate read | Data leak | tenant_id on all queries + auth | Misconfigured query | Mitigated |
| RATE-T02 | Cross-tenant rate mutation | Integrity | tenant + party checks | Bug | Mitigated |
| RATE-T03 | Spoofed company context | Wrong pricing | Server membership validation | — | Mitigated |
| RATE-T04 | Unauthorized activation | Commercial harm | RBAC ACTIVATE_* + S2S until v2.0E | — | Mitigated |
| RATE-T05 | Historical rate edit | Financial fraud | Immutable ACTIVE versions | Direct DB | Mitigated (ops) |
| RATE-T06 | Concurrent double activation | Two ACTIVE | DB partial unique + FOR UPDATE | — | Mitigated |
| RATE-T07 | Resolver ambiguity | Wrong price | RATE_AMBIGUOUS fail closed | — | Mitigated |
| RATE-T08 | RFx source spoof | Wrong price | rfx-service internal API + validation | — | Mitigated |
| RATE-T09 | Manual price abuse | Under/over charge | USE_MANUAL_SPOT_PRICE + audit | — | Mitigated |
| RATE-T10 | Currency manipulation | FX fraud | ISO validation + mismatch fail | — | Mitigated |
| RATE-T11 | Precision loss | Penny drift | decimal + §11.2.1 | Legacy float at settlement boundary until v2.0C | Mitigated for new domain |
| RATE-T12 | Snapshot mutation | Historical rewrite | INSERT-only repo | DB admin | Mitigated |
| RATE-T13 | Settlement repricing | Wrong invoice | snapshot-only for new TOs | Legacy path | Mitigated at v2.0C |
| RATE-T14 | Fuel/accessorial double count | Overpay | total_amount includes fuel | — | Mitigated |
| RATE-T15 | Retry different price | Duplicate/conflicting TO | v2.0C idempotency + snapshot unique | Until v2.0C | Accepted gap pre-v2.0C |

---

### ADVERSARIAL SCENARIOS

| # | Scenario | Expected outcome | Verdict |
|---|----------|-------------------|---------|
| A | Two active eligible rates same lane | RATE_AMBIGUOUS or explicit rule — never arbitrary | PASS |
| B | Contract price changes after TO | Existing snapshot unchanged | PASS |
| C | Contract terminated after shipment start | Snapshot remains valid historical agreement | PASS |
| D | RFx award total only, base unknown | total preserved; base NULL; no synthetic split | PASS |
| E | Settlement with fuel in snapshot total | Fuel not added again | PASS |
| F | Two version activations race | DB one-ACTIVE invariant | PASS |
| G | TO retry after timeout + rate change | Same idempotent commercial result (v2.0C) | PASS (design) |
| H | Cross-tenant rate_card_id | DENY | PASS |
| I | Explicit RFx source unavailable | Fail closed — no silent fallback | PASS |
| J | Payment queries latest rate for historical txn | DENY BY ARCHITECTURE | PASS |

---

### REMEDIATIONS

| ID | Severity | Old rule | Corrected rule | Files changed |
|----|----------|----------|----------------|---------------|
| H-01 | HIGH | equipment required but undefined normalization | §9.4 TRIM, case-sensitive, fail on blank | `docs/engineering/FREIGHT_CONTRACT_RATE_MANAGEMENT_v2.0_ARCHITECTURE.md` |
| H-02 | HIGH | vague "half-up at boundaries" | §11.2.1 deterministic per-component algorithm | same |
| H-03 | HIGH | v2.0A API without security gate | S2S-only until v2.0E on v2.0A/v2.0B slices | same |
| M-01 | MEDIUM | implicit buyer/shipper | §20.2 mapping table | same |
| M-02 | MEDIUM | termination underspecified | §7.6–7.7 immediate + transition table | same |
| M-03 | MEDIUM | no observability section | §31 Observability | same |
| M-04 | MEDIUM | delete semantics implicit | §30 Delete/Mutability | same |

---

### FINAL GATES

| Gate | Value |
|------|-------|
| `ARCHITECTURE_CONSISTENT` | YES |
| `FINANCIAL_MODEL_SAFE` | YES |
| `MONEY_EXACT_DECIMAL` | YES (contract-rate domain) |
| `RATE_RESOLUTION_DETERMINISTIC` | YES |
| `AMBIGUITY_FAILS_CLOSED` | YES |
| `ONE_ACTIVE_VERSION_DB_ENFORCED` | YES (design) |
| `SNAPSHOT_IMMUTABLE` | YES |
| `SNAPSHOT_SELF_CONTAINED` | YES |
| `TO_IDEMPOTENCY_DESIGN` | YES (v2.0C) |
| `SETTLEMENT_REPRICING_DENY` | YES |
| `FUEL_DOUBLE_COUNT_DENY` | YES |
| `CROSS_SCHEMA_SQL_DENY` | YES (contract-rate; legacy billing until v2.0C) |
| `TENANT_ISOLATION_DESIGN` | YES |
| `FAIL_CLOSED_PRICING` | YES |
| `HISTORICAL_REPRICING_DENY` | YES |
| `SLICE_PLAN_IMPLEMENTABLE` | YES |

---

### FINAL VERDICT

**`FINAL_VERDICT=PASS_WITH_NITS`**  
**`READY_FOR_V2_0A=YES`**

Architecture is coherent and implementable after HIGH/MEDIUM remediations. Residual nits: legacy float64 at settlement boundary until v2.0C; equipment type enum centralization deferred; generic TO idempotency implemented in v2.0C not v2.0A.

**Do not start v2.0A until this review PR is merged into architecture branch** (team process).

---

*End of final architecture review.*
