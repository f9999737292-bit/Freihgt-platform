# W1 ABAC Authorization Matrix (R2) — J-STA-0.9-003 / J-W1-H002 / J-W1-M003

**Status:** REQUIRED before any W1 HTTP API or persistence  
**W1 ships:** in-process eval + AuthContext checks only (no endpoints/repos yet)  
**Headers:** Never trust client `X-Tenant-ID` / `X-User-ID` / `X-Company-ID` as authority without server verification.

---

## 1. Attribute catalog

`tenant_id` · `buyer_company_id` · `carrier_company_id` · `campaign_id` · `policy_owner` · `actor_membership` · `actor_permission` · `policy_version` · `resource_status` · `lane_scope`

---

## 2. Authorization layering (J-W1-H002)

### 2.1 API Gateway (future `/api/v1/rfx/allocations…`)

| Responsibility | Behavior |
|---|---|
| JWT verification | Validate signature/alg/exp when `AUTH_ENABLED` |
| Trusted identity context | Set actor subject/email from JWT claims only |
| Company membership | Resolve via identity-service; not from client body alone |
| Route permission | Map route → permission; deny if missing |
| Spoofed headers | Strip/ignore inbound authority headers; overwrite from JWT (existing gateway pattern) |
| Company selection | If client nominates company, must be ∈ verified memberships; else 403 |

### 2.2 rfx-service

| Responsibility | Behavior |
|---|---|
| Re-check ActorContext | Reject empty/missing tenant/actor |
| Policy ownership | `policy_owner` must match actor’s authorized buyer company for manage/draft ops |
| Campaign ownership | Campaign buyer company must match |
| Resource status | Draft vs published gates (see §3) |
| Allowed attributes | Only AST catalog; unknown attr deny |
| Version preconditions | If-Match / expected version hash |
| Fail-closed | Missing authority ⇒ deny (no soft allow) |

### 2.3 Repository (future persistence — contract only in W1)

| Responsibility | Behavior |
|---|---|
| `tenant_id` predicates | Every query scoped |
| `owner_company_id` | Restrict to owner for drafts |
| Tenant-safe FKs | Composite tenant + id |
| No cross-company lookup | Deny by predicate, not “not found” oracle across tenants |
| No existence oracle | Cross-tenant → same 404/403 policy as authorized miss |
| Concurrent updates | Optimistic version / content hash |

W1 does **not** implement HTTP or repository; boundaries above are normative for later waves.

---

## 3. Permissions — clarify policy.read vs tender.read (J-W1-M003)

| Permission | Allows | Denies |
|---|---|---|
| `allocation.policy.manage` | Create/update **draft**, approve version (with rules), evaluate_test on own drafts | Carrier use; foreign company |
| `allocation.policy.read_own` | Read **own company** policies including **drafts** | Other companies’ drafts; other tenants |
| `allocation.policy.read_published` | Read **published** rules attached to a tender the actor may view | Drafts; other carriers’ confidential rule packs |
| `tender.read` | Tender metadata / non-sensitive published summary | **Not** sufficient for draft policy; **not** commission internals; **not** other carriers’ confidential settings |
| `allocation.simulate` | Run evaluation preview on authorized policy | Persist Award; cross-tenant attrs |
| `allocation.evaluation.read` | Read evaluation explanation for authorized campaign | Raw competitor bids beyond policy |

**Normative replacements for ambiguous “policy.read OR tender.read”:**  
- Draft read ⇒ `allocation.policy.read_own` or `manage` only.  
- Published tender-facing rules ⇒ `allocation.policy.read_published` (and tender participation as applicable).  
- `tender.read` alone ⇒ **DENY** draft policy.

**Carrier:** never see another carrier’s confidential rules or buyer commission internal settings.

---

## 4. Matrix (representative)

| ACTOR | ACTION | RESOURCE | TENANT | COMPANY | RESULT |
|---|---|---|---|---|---|
| Buyer PM + membership | evaluate_test | own draft | match | owner match | ALLOW |
| Buyer with only tender.read | read | own draft | match | match | **DENY** |
| Buyer PM | read | other company draft | match | mismatch | DENY |
| Buyer PM | evaluate | other tenant | mismatch | * | DENY |
| Carrier | policy.manage | any | * | * | DENY |
| Carrier | read_published | tender rules for invited tender | match | participant | ALLOW (non-confidential slice only) |
| Revoked membership | any | former company | match | was | DENY |
| Missing company | any | * | * | null | DENY |
| Forged X-Company-ID | any | * | * | forged | DENY |
| DRIVER role | manage | * | * | * | DENY |
| Stale If-Match | update | version | match | match | DENY |
| PLATFORM_ADMIN | act as client commercial | client draft | * | * | DENY w/o controlled override |

---

## 5. Engine binding

`Evaluate` requires verified AuthContext; candidate `tenant_id` must match; unknown attributes denied; no raw headers as AuthContext.
