# BINTRANS E2E Transport Flow 0.1D — Financial Closing

`TestBintransE2ETransportFlow` keeps the single chain from PR #303 and continues it after the completed POD. One RFx event, one award, one transport order, and one shipment stay in place. Billing runs as `billing-register-service` on that same PostgreSQL database. The test does not insert a second shipment, a second transport order, a POD row, a payment obligation, or a freight-cost consumer.

## States

Settlement: `DRAFT` → `UNDER_REVIEW` → `APPROVED`.

Register: inclusion creates `DRAFT` with one item → `CALCULATED` → `APPROVED` → `CLOSING_DOCUMENTS_CREATED`.

`PAID` and `CLOSED` stay outside this stage. The test does not call `mark-documents-ready`, `mark-ready-for-payment`, `mark-sent-to-edo`, `mark-signed`, `mark-paid`, or `close`.

## HTTP sequence

Every call sends `X-Tenant-ID`, `X-User-ID`, `X-Company-ID`, and `X-Actor-Kind`. The service accepts the actor only when `core.company_memberships` is `ACTIVE` for that company and the derived kind matches the header. The shipper member with `PROCUREMENT_MANAGER` is `BUYER`. The awarded carrier member with `CARRIER_DISPATCHER` is `CARRIER`.

| Step | Call | Actor | Result |
| --- | --- | --- | --- |
| Create | `POST /v1/freight-settlements` with `shipment_id` and `idempotency_key` | CARRIER | `201`, `DRAFT` |
| Submit | `POST /v1/freight-settlements/{id}/submit-for-review` | CARRIER | `200`, `UNDER_REVIEW` |
| Approve | `POST /v1/freight-settlements/{id}/approve` | BUYER | `200`, `APPROVED` |
| Include | `POST /v1/freight-settlements/{id}/include-in-register` | BUYER | `200`, one `DRAFT` register and one item |
| Calculate | `POST /v1/billing-registers/{id}/calculate` | BUYER | `200`, `CALCULATED` |
| Approve register | `POST /v1/billing-registers/{id}/approve` | BUYER | `200`, `APPROVED` |
| Package | `POST /v1/billing-registers/{id}/closing-document-package` with an empty body | BUYER | `201`, `ACT_PLUS_VAT_INVOICE`, register `CLOSING_DOCUMENTS_CREATED` |

The empty package body keeps the default type. That default writes one row in `billing.acts` and one row in `billing.vat_invoices`.

## POD and money

Settlement eligibility reads `documents.documents` in the shared database. A POD counts when the row is undeleted, `document_type` is `POD`, `related_entity_type` is `SHIPMENT`, `related_entity_id` is this shipment, and `tenant_id` matches the caller. The completed document-service upload from the delivery stage already satisfies that predicate. The shipment stays `DELIVERED`. Direct `POST /v1/billing-registers/{id}/items` is not used: that path requires `READY_FOR_BILLING` or `DOCUMENTS_COMPLETED`.

The award order is `pricing_model_version = SNAPSHOT_V1`. Buyer, carrier, amount, and currency come from `transport.transport_order_rate_snapshots`. The snapshot does not supply `vat_rate`, so the runtime VAT amount is zero. The test compares settlement and register amounts with that snapshot row. It does not assert the commercial bid body.

Each accepted financial write is one database transaction and inserts `billing.settlement_audit_events`. Settlement and register mutations also insert `billing.freight_cost_outbox` rows with status `PENDING`. The outbox worker stays disabled, so those rows are not published.

## Idempotency and denials

| Call | Repeat or denial | Writes |
| --- | --- | --- |
| Create | Same settlement id, still one row, `201` | No second settlement |
| Include, calculate, register approve, package | Same register, item, totals, or package id | No second register, item, act, or VAT invoice |
| Include by another shipper or carrier | `403` before any replay | No settlement payload, and no change to registers, items, audit events, outbox rows, or settlement version |
| Include by a foreign tenant | `404` | No information about the settlement |
| Submit or settlement approve | `400` | No second audit event and no second settlement |
| Foreign tenant | `404` on create and on settlement read | No settlement |
| Other carrier as `CARRIER`, or other shipper as `BUYER` | `403` | No settlement and no status change |
| Body `tenant_id` different from `X-Tenant-ID` | `403` on calculate, register approve, and package create | Register status and document counts stay unchanged |

A denied call leaves `billing.payment_obligations` empty. The register does not become `PAID` or `CLOSED`. Billing does not add a row to `documents.documents`, and a foreign tenant still cannot read the POD.

## CI name

The workflow job id stays `bintrans-e2e-transport-flow`. Its displayed check name changes from `bintrans-e2e-transport-flow-v0.1c` to `bintrans-e2e-transport-flow`. This pull request does not change branch protection and does not mark the check required. On 2026-10-24 the required checks for `main` are repository safety, system security, the wave-2 business flow, the web admin and procurement frontends, scripts, five backend Go checks, and the PostgreSQL role foundation. Neither display name is in that set, so the rename does not remove a check `main` currently requires. Open pull requests that still run the workflow from `main` keep reporting the previous display name. After this workflow file is merged, new pull requests report the stable name. Making that name required remains a separate decision.

## Limits

No payment-service process is started. No freight-cost consumer is started. TMS activation and the enforcement gate stay off. PostgreSQL roles, grants, and `pg_hba` are unchanged. Product handlers, eligibility checks, and authorization checks are unchanged.
