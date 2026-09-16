# RFx v3.0E7 — ERP API E1 Implementation

**Wave:** E1 (schema and principal foundation)
**Branch:** `feat/rfx-erp-schema-principals-e1-v3.0e7`
**Status:** `IMPLEMENTED_ACCEPTED`

## Authorization markers

```
ERP_API_IMPLEMENTATION_AUTHORIZED=YES
ERP_API_IMPLEMENTATION_STARTED=YES
ERP_API_IMPLEMENTATION_STATUS=IMPLEMENTATION_IN_PROGRESS
ERP_API_E1_AUTHORIZED=YES
ERP_API_E1_STATUS=IMPLEMENTED_ACCEPTED
MIGRATION_000074_AUTHORIZED=YES
MIGRATION_000074_CREATED=YES
ERP_API_E2_AUTHORIZED=NO
ERP_API_E2_STATUS=NOT_STARTED
ERP_API_E3_AUTHORIZED=NO
ERP_API_E4_AUTHORIZED=NO
ERP_API_E5_AUTHORIZED=NO
ERP_API_E6_AUTHORIZED=NO
PUBLIC_ERP_ROUTES_ADDED=NO
OPENAPI_CHANGED=NO
ROUTE_MANIFEST_CHANGED=NO
CONTROLLER_VERDICT=ACCEPT_ERP_E1_CI_JOB
CONTROLLER_REVIEW_HEAD=f66f45fa34e575c29e1adc01b174f79100458dfb
CONTROLLER_CI_RUN=35070290799
CONTROLLER_CI_RESULT=SUCCESS
FOLLOW_UP_ID=ERP-E1-CI-DEDICATED-JOB
FOLLOW_UP_STATUS=IMPLEMENTED_ACCEPTED
ERP_E1_DEDICATED_POSTGRES_CI_JOB=ADDED
ERP_CI_JOB_NAME=rfx-erp-e1-postgres-integration
ERP_CI_JOB_RESULT=SUCCESS
E7P2_INT_187=PASS
BRANCH_PROTECTION_GAP_ACCEPTED_FOR_CURRENT_POLICY=YES
ERP_E1_JOB_REQUIRED_BY_BRANCH_PROTECTION=NO
NEXT_ACTION=CONTROLLED_MERGE_PR140
```

## Delivered scope

| Area | Deliverable |
|---|---|
| Migration | `000074_rfx_erp_integration_principals_v3_0e7_phase2.{up,down}.sql` |
| Integration principals | `rfx_integration_principals` + repository |
| Credentials | Hash-only `rfx_integration_credentials` + repository |
| Scopes | `rfx_integration_scopes` + repository |
| Analysis ownership | XOR `actor_id` / `integration_principal_id` |
| Idempotency | Owner-kind partial unique indexes + repository API |
| External identity | Stable key without revision; revision history table |
| Reference mapping | Sets + entries + repository |
| Tests | `E7P2-INT-187` + wave-local checks in `internal/integration/erp/` |
| Release contract | Max migration pin raised to `000074` |

## Explicitly not in E1

- OAuth token endpoint / API-key HTTP auth
- ERP Preview / Commit / GET handlers
- Gateway routes / OpenAPI ERP operations
- `RFX_ERP_INTEGRATION_ENABLED` feature flag

## Rollback

Down migration `000074` restores post-`000073` schema when no ERP-owned analyses, ERP idempotency rows, mapping sets, or link revision rows exist. Principals/credentials/scopes tables are dropped last.

## Primary test

`E7P2-INT-187` — migration `000074` up / down / up with schema, constraint, repository, and rollback-guard coverage.
