# Document write role policy 0.1C

Stage: `DOCUMENT-WRITE-ROLE-POLICY-0.1C-DISCOVERY`

Finding: `J-DWA-004`

Related contract: pull request [#360](https://github.com/f9999737292-bit/Freihgt-platform/pull/360), `docs/architecture/DOCUMENT_WRITE_AUTHORITY_CONTRACT_0_1B.md`

Issue context: [#343](https://github.com/f9999737292-bit/Freihgt-platform/issues/343)

Base: `3f306fc407aeda4bdc652d825954292f4a54fad6`

Mode: design only. This document does not change pull request #360, an API, a service, a migration, a role seed, staging, or production.

`ROLE_POLICY_STATUS=PROPOSED_NOT_APPROVED`

`ROLES_APPROVED=NO`

`COMPANY_POLICY_DECISION=REQUIRED`

`MEMBERSHIP_ALONE_SUFFICIENT=UNDECIDED`

`PLATFORM_ADMIN_ALL_COMPANIES=NO`

`FINAL_VERDICT=POLICY_DECISION_REQUIRED`

`GAP-B-001` stays open. An internal `SIGNED` status is not an external EDO exchange.

## Question

Pull request #360 requires an active company membership before an operator document write, and it leaves a cross-company `PLATFORM_ADMIN` grant undecided. `J-DWA-004` asks the next question: is that `ACTIVE` membership enough to create, change, and sign documents, or does the user also need a separate write permission on that company?

This discovery does not answer that question. It records what the roles do on this base and the decision a controller still has to make.

## What exists

Seeded role codes are in `infrastructure/migrations/000009_seed_roles.up.sql` and `000013_seed_forwarder_manager_role.up.sql`:

| Code | Scope | Seed description |
| --- | --- | --- |
| `PLATFORM_ADMIN` | `GLOBAL` | Full platform administrator |
| `SHIPPER_ADMIN` | `TENANT` | Shipper company administrator |
| `SHIPPER_LOGIST` | `TENANT` | Creates and manages transport orders |
| `CARRIER_ADMIN` | `TENANT` | Carrier company administrator |
| `CARRIER_DISPATCHER` | `TENANT` | Manages bids, vehicles and drivers |
| `DRIVER` | `TENANT` | Driver mobile app user |
| `CONSIGNEE_OPERATOR` | `TENANT` | Receives goods and confirms delivery |
| `PROCUREMENT_MANAGER` | `TENANT` | Manages RFx and tenders |
| `FORWARDER_MANAGER` | `TENANT` | Forwarder manager |
| `FINANCE_MANAGER` | `TENANT` | Manages billing registers and closing documents |
| `GOV_INSPECTOR` | `GLOBAL` | Government inspection access |

`CONSIGNEE_VIEWER` and `CARRIER_ACCOUNTANT` appear in some gateway or admin-shell references. No migration in this tree inserts them. They are not document-write roles.

`core.permissions` is seeded by `000008_seed_permissions.up.sql` with `document.create`, `document.read`, and `document.sign`. `core.role_permissions` has no row for those three codes. The only `role_permissions` insert in this tree is the later manual-spot permission, which is not a document permission. Gateway and document-service do not read `document.create`, `document.read`, or `document.sign`.

Company membership is a separate fact. `core.company_memberships.membership_status` can be `ACTIVE`, `INVITED`, `SUSPENDED`, or `DELETED`. Gateway `IdentityClient.ListUserCompanies` loads `ACTIVE` memberships and the role codes on them. An `ACTIVE` membership is not one of the `document.*` permissions.

`core.user_roles.company_id` can be null. A null company is how a tenant-global `PLATFORM_ADMIN` is stored. Source-authority comments in shipment-service and rfx-service already say that role replaces a company role and does not replace membership. Payment and contract-rate checks deny `PLATFORM_ADMIN` when the company membership is missing. Those checks are not wired to document writes.

## Who can write documents today

document-service write handlers do not read a role or a permission. On a direct call, `X-Tenant-ID` is the tenant input. That gap is the subject of pull request #360 and is not re-litigated here.

When the public request goes through api-gateway and `AUTH_ENABLED=true`:

| Operation | Public route | Role check on this base | Company check |
| --- | --- | --- | --- |
| Read, shipper | `GET /api/v1/shipper/documents/**` | `RequireShipperRead`. The selected company must be type `SHIPPER` and the same membership must include `SHIPPER_ADMIN` or `SHIPPER_LOGIST`. | Membership of the selected company. `PLATFORM_ADMIN` is not sufficient. |
| Read, carrier | `GET /api/v1/carrier/documents/**` | `RequireCarrierRead`. The selected company must be type `CARRIER` and the same membership must include `CARRIER_ADMIN` or `CARRIER_DISPATCHER`. | Membership of the selected company. `PLATFORM_ADMIN` is not sufficient. |
| Read, operator | `GET /api/v1/documents/**` | `RequireOperatorShipmentRead`. Tenant-global `PLATFORM_ADMIN` only. | None. The guard comment says customer memberships are not an operator context. |
| Create | `POST /api/v1/documents` | Same operator guard. | None. `owner_company_id` is not a membership check. |
| Version and files | `POST /api/v1/documents/{id}/versions` and `.../files` | Same operator guard. | None. |
| Attachments, finalize, attachment signature | `POST /api/v1/documents/{id}/attachments/**` | Same operator guard. | None. |
| Ready for signing and signing session | `POST .../ready-for-signing` and `.../signing-sessions` | Same operator guard. | None. |
| Session signature | `POST /api/v1/signing-sessions/{id}/signatures` | Same operator guard. `signer_user_id` and `signer_company_id` are existence checks inside document-service, not a role check. | None. |
| Cancel and archive | `POST .../cancel` and `.../archive` | Same operator guard. | None. |
| Signing recovery | `POST .../signing-recovery` | No role. The handler returns `403`. | None. |
| POD upload | `POST /api/v1/driver/me/shipments/{shipmentId}/pod/uploads` and the content and complete routes | `ensureDriverAccess` requires the `DRIVER` role when auth is enabled. | The driver record's `companyId`. document-service then requires `transport.shipments.driver_id` to equal that driver. |

When `AUTH_ENABLED=false`, `RequireOperatorShipmentRead` and `ensureDriverAccess` do not apply those role checks. Pull request #360 already says the gateway must not issue a document-write credential in that mode.

Shipper and carrier apps do not create, version, sign, cancel, or archive documents. Their document routes are `GET`. Consignee has no document route. `apps/driver-mobile` is the POD client and is not a customer-portal role. `FINANCE_MANAGER` is described as managing closing documents, and the seeded permission for that area is `billing_document.create`, which is a different resource from `document.create`. No finance route writes a document-service document.

EDO signing on this base is the internal session and signature flow. It has no permission check named `document.sign`. It does not assign a signer role. Qualified external signature and an EDO operator are not implemented.

## Operations that must stay separate

A later policy has to name each operation. One membership or one role code must not be treated as all of them.

| Operation | Routes | Seeded permission today |
| --- | --- | --- |
| Read | shipper `GET`, carrier `GET`, operator `GET /api/v1/documents` | `document.read` exists and is not granted to any role |
| Create | `POST /v1/documents` | `document.create` exists and is not granted |
| Version and attachments | versions, files, attachment create, finalize, attachment signature | No permission code |
| Prepare for signing | `ready-for-signing`, `signing-sessions` | No permission code |
| Sign | `POST /v1/signing-sessions/{id}/signatures` | `document.sign` exists and is not granted |
| Cancel and archive | `cancel`, `archive` | No permission code |
| POD | `/internal/v1/pod-uploads` behind the driver routes | No permission code. The live check is the `DRIVER` role plus shipment assignment |

Signing recovery is not an operation in this policy. It stays HTTP `403`.

## Candidate company-scoped permissions

This is the proposed shape. It is not a grant.

A permission applies only to the company on the verified `ACTIVE` membership that carries it. A role or permission on another company does not transfer. `PLATFORM_ADMIN` does not fill every cell. `CompanyExists` does not fill any cell.

| PERMISSION | OPERATION | SEEDED | ASSIGNMENT |
| --- | --- | --- | --- |
| `document.read` | Read documents for that company | Yes | `DECISION_REQUIRED` |
| `document.create` | Create a document whose `owner_company_id` is that company | Yes | `DECISION_REQUIRED` |
| `document.version` | Add a version, file, or attachment, including attachment finalize and attachment signature | No | `DECISION_REQUIRED` |
| `document.prepare_signing` | Move to ready for signing and create a signing session | No | `DECISION_REQUIRED` |
| `document.sign` | Add a session signature as the verified user for that company | Yes | `DECISION_REQUIRED` |
| `document.cancel` | Cancel | No | `DECISION_REQUIRED` |
| `document.archive` | Archive | No | `DECISION_REQUIRED` |
| `pod.upload` | POD intent, content, and complete for a shipment whose `driver_id` is this driver | No | Separate from the company document permissions. See Driver POD. |

`document.read` does not imply `document.create`. `document.create` does not imply version, prepare, sign, cancel, or archive. `document.sign` does not imply create. `pod.upload` does not imply any `document.*` permission, and a `document.*` permission does not imply `pod.upload`.

The shipment or document relationship check from pull request #360 stays in front of these permissions: the company on the permission must also be a party on the shipment when the document names one. A permission does not replace that relationship check, the JWT tenant, or the internal caller credential.

### Role rows a controller would fill

Every cell below is `DECISION_REQUIRED`. None is granted by this document. The row for `PLATFORM_ADMIN` is a prohibition, not a grant: tenant-global `PLATFORM_ADMIN` does not receive these permissions for every company. A later administrative exception would be a named permission, an audit record, and its own tests, which pull request #360 already refused to approve.

| ROLE | READ | CREATE | VERSION AND ATTACHMENTS | PREPARE | SIGN | CANCEL AND ARCHIVE | POD |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `SHIPPER_ADMIN` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `SHIPPER_LOGIST` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `CARRIER_ADMIN` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `CARRIER_DISPATCHER` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `CONSIGNEE_OPERATOR` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `PROCUREMENT_MANAGER` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `FORWARDER_MANAGER` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `FINANCE_MANAGER` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `GOV_INSPECTOR` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` |
| `DRIVER` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | `DECISION_REQUIRED` | Separate `pod.upload` candidate. Not a company document permission. |
| `PLATFORM_ADMIN` | `NOT_ALL_COMPANIES` | `NOT_ALL_COMPANIES` | `NOT_ALL_COMPANIES` | `NOT_ALL_COMPANIES` | `NOT_ALL_COMPANIES` | `NOT_ALL_COMPANIES` | `NOT_ALL_COMPANIES` |
| `ACTIVE` membership with no document permission | Read only if a future read grant says so | `DENY` under options B and C | `DENY` under options B and C | `DENY` under options B and C | `DENY` under options B and C | `DENY` under options B and C | `DENY` |

Observed customer read access is narrower than this undecided matrix. Today `SHIPPER_ADMIN` and `SHIPPER_LOGIST` can read shipper documents for a shipper company, and `CARRIER_ADMIN` and `CARRIER_DISPATCHER` can read carrier documents for a carrier company. That read is a route guard, not `document.read` in `core.role_permissions`. It is not a write grant.

## Driver POD

POD is not a company document-write permission.

The public caller is a user who passes `CanAccessDriverRoutes`, which accepts `DRIVER` and no other seeded code. The company on the upload is the company on that driver record. The shipment authority is `transport.shipments.driver_id` in the same tenant. An `ACTIVE` membership in the carrier company, a `CARRIER_ADMIN` role, or a `document.create` permission does not make that user the assigned driver.

A `DRIVER` who also has an `ACTIVE` company membership does not gain create, version, prepare, sign, cancel, or archive from the driver role. Those document operations stay closed to the driver unless a future controller decision grants one of them on that company. This discovery does not make that grant.

Pull request #360's `POD-ASSIGN-DENY` case stays: a valid POD service credential plus an existing unassigned driver writes nothing.

## Decision options

The controller chooses one. This document does not.

Option A. `ACTIVE` membership is the write grant. A verified user may perform every operator document operation for a company in that membership when the shipment relationship matches. No extra permission is required. Risk: every active member of the company, including a member with only a read-oriented role, can create, sign, cancel, and archive. The unused `document.*` rows stay unused.

Option B. Membership is necessary and not sufficient. The user needs the company-scoped permission for that operation. `document.create`, `document.read`, and `document.sign` can be reused. Version, prepare, cancel, archive, and `pod.upload` need new permission codes and a later migration, which this discovery does not add. `ACTIVE` membership without the permission is a denial. Role-to-permission assignment stays a second controller decision. `PLATFORM_ADMIN` is not inserted into those grants.

Option C. Keep gateway role allowlists, as shipment and fleet routes do, instead of `core.permissions`. Each document operation gets an explicit role list scoped to the selected company membership. That list does not exist today. Choosing the codes is the same undecided matrix as above. `PLATFORM_ADMIN` is not added to the list as a tenant-wide override.

Option A conflicts with a reading of `J-DWA-004` that wants a member without write authority to be denied. Options B and C provide that denial and need a controller to name who receives which operation. No option is selected.

## Negative tests

These tests belong to the implementation that follows a controller decision. This document does not add them. They apply to options B and C. They do not apply to option A, which is why the decision has to come first.

The caller credential and tenant assertion are valid. The user has an `ACTIVE` membership in company C. The shipment, when named, lists C. The membership has no permission or role for the operation under test. Each case returns `HTTP_DENIED=YES` and `STORE_WRITES=0`. A membership or permission read is allowed. A repeat writes nothing.

- `ACTIVE` membership without `document.create` cannot `POST /v1/documents` for company C.
- The same membership with only `document.read` cannot create, add a version, add an attachment, prepare signing, sign, cancel, or archive.
- `ACTIVE` membership without `document.version` cannot add a version, file, or attachment.
- `ACTIVE` membership without `document.prepare_signing` cannot move the document to ready for signing or create a session.
- `ACTIVE` membership without `document.sign` cannot add a session signature, including when `signer_user_id` is that user and the company exists.
- `ACTIVE` membership without `document.cancel` or `document.archive` cannot cancel or archive.
- Tenant-global `PLATFORM_ADMIN` with no membership in C, and no separately approved administrative grant, cannot write a document for C.
- `DRIVER`, including a driver who also has an `ACTIVE` carrier membership, cannot call the operator document create, version, sign, cancel, or archive routes.
- `SHIPPER_ADMIN` with an `ACTIVE` shipper membership cannot create a POD intent for a shipment whose `driver_id` is someone else.
- A permission or role on company D cannot write company C.

Reaching the handler is not success. Success for a denied case is an unchanged store.

## Compatibility with pull request #360

This discovery does not edit that pull request.

The following parts of the 0.1B contract stay compatible with every option here:

- document-service confirms the internal caller with a dedicated token and an exact service name
- document writes, POD uploads, and POD action evidence use different credentials
- client `X-Tenant-ID` or `X-User-ID` alone is not authority
- gateway stripping of client identity and internal-authority headers does not depend on `AUTH_ENABLED`
- `AUTH_ENABLED=false` does not issue a document-write credential
- tenant isolation and the shipment-party relationship check
- signing recovery stays HTTP `403`
- empty and unknown internal tokens fail closed
- POD assignment denial with `STORE_WRITES=0`
- joint gateway and document-service rollout, with no staging or production apply in the design pull request

The company section of pull request #360 treats `ACTIVE` membership as the default company grant and sets `COMPANY_POLICY_DECISION=REQUIRED` for a cross-company administrator. That membership check is necessary under options B and C and is the whole company grant under option A. Implementing the membership check before this decision selects option A in practice. This discovery does not authorize that implementation and does not change the contract text.

A later implementation that follows option B or C adds the permission or role check after the caller credential and the membership check. It does not replace those checks. It does not reopen signing recovery. It does not copy the permission onto shipper or carrier `GET` routes unless the controller also decides `document.read`. It does not give `PLATFORM_ADMIN` the new permissions for every company.

## Out of scope

Qualified signature verification, an EDO operator, GAP-B-001, portal UI, payment rules, seeding `role_permissions`, new permission migrations, and production rollout are outside this discovery. Read-route authority beyond the split above is not redesigned here.
