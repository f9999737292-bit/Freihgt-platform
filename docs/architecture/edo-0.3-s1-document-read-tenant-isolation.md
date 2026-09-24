# EDO 0.3 S1 Document Read Tenant Isolation

## Status

```text
CONTROLLER_VERDICT=ACCEPT_EDO_0_3_S1
EDO_0_2_STATUS=IMPLEMENTED_ACCEPTED
EDO_0_3_DISCOVERY_STATUS=DISCOVERY_ACCEPTED
EDO_0_3_S1_STATUS=IMPLEMENTED_ACCEPTED
EDO_0_3_IMPLEMENTATION_AUTHORIZED=S1_ONLY
DOCUMENT_READ_TENANT_ISOLATION_STATUS=REMEDIATED_ACCEPTED
EDO_0_3_I1_I4_STATUS=NOT_AUTHORIZED
LEGAL_VERIFICATION_STATUS=OPEN
OPENAPI_CHANGED=NO
SCHEMA_MIGRATION=NO
```

Independent controller review accepted the S1 remediation. This record does not close legal verification and does not assert that a signature is legally valid.

```text
PR=https://github.com/f9999737292-bit/Freihgt-platform/pull/164
ACCEPTED_PRODUCT_HEAD=dff9eacf04e0fd8c0e381d8716cf5a4039c90ff1
ACCEPTED_CI_RUN=35991273506
TEST_DOCUMENT_READ_TENANT_ISOLATION=PASS
DOCUMENT_READ=TENANT_SCOPED
SIGNING_SESSION_READ=TENANT_SCOPED
FOREIGN_MISSING_DELETED_DOCUMENT=SAME_404
MISSING_TRUSTED_TENANT=401
OPENAPI_CHANGED=NO
SCHEMA_MIGRATION=NO
BLOCKING_FINDINGS=NONE
```

## Defect

`GET /v1/documents/{id}` followed `DocumentHandler.GetByID` → `DocumentService.GetByID` → `GetDetail` → `repository.GetByID`. The SQL predicate was `id` and `deleted_at` only. `GET /v1/signing-sessions/{id}` used `GetSessionByID` with an `id` predicate only. Both routes are public through the API gateway prefixes `/api/v1/documents` and `/api/v1/signing-sessions`.

`documents.documents.tenant_id` and `documents.signing_sessions.tenant_id` already exist. No migration was added.

## Trust boundary

The API gateway strips client identity headers and sets `X-Tenant-ID` from the verified JWT. Document reads take the tenant only from that header. Query `tenant_id`, a JSON body, and a client-supplied header that the gateway has not replaced are not a tenant selector for these reads. A missing or non-UUID header fails closed with `401` and does not query the row. The external OpenAPI contract already declared `X-Tenant-ID` and was not changed.

List and mutation routes still use their existing tenant inputs. Those paths were not widened.

## Read paths in this wave

- `GET /v1/documents/{id}` loads detail with `GetByIDAndTenant`: `id`, `tenant_id`, and `deleted_at IS NULL` in one statement. Versions and files are loaded only after that statement returns a row.
- `GET /v1/signing-sessions/{id}` loads the session with `GetSessionByIDAndTenant`.

A missing id, a foreign tenant, and a soft-deleted document return the same `404` envelope: `NOT_FOUND` / `document not found`, with empty details. A foreign signing session uses the same envelope as a missing session. The body does not include the other tenant's document number, company, or identifiers.

`GET` does not insert documents, versions, files, sessions, signatures, or upload intents.

Internal POD completion now passes the tenant already bound to the upload intent into the same document read. That call site changed because the read method requires a tenant. POD behavior was not otherwise expanded.

Unscoped repository methods `GetByID` and `GetSessionByID` remain in the repository and are not used by these HTTP reads.

## Test matrix

| Scenario | Result |
| --- | --- |
| Tenant A reads its document | 200 |
| Tenant B reads Tenant A's document, including a query `tenant_id` of the owner | 404 |
| Unknown document id | same 404 envelope |
| Soft-deleted document | same 404 envelope |
| No authentication at the gateway | 401 |
| Spoofed identity header at the gateway | replaced by the JWT tenant |
| Direct downstream call without a trusted tenant header | 401, no row read |
| Repeated GET | no inserted rows |
| Existing list filter and cross-tenant version create | still isolated |
| Own signing session | 200 |
| Foreign signing session | same 404 envelope as a missing session |

## Out of scope

I1–I4 are not started. `DocumentPackage`, `DocumentRelationship`, revision-bound signatures, `ArchiveManifest`, MChD, an EDI operator, and e-ТрН were not implemented. Legal verification stays `OPEN`.
