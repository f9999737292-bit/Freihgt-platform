# EDO signing recovery 0.1 — roadmap

Stage 0.1 is this documentation. Stages 0.2 through 0.7 are not started.

| Stage | Scope | Acceptance gate | Not included |
| --- | --- | --- | --- |
| 0.1 Discovery and architecture | Inventory, failure matrix, state machine, security, tests, ADR | Agent J review. No product diff | Implementation |
| 0.2 Internal recovery foundation | Domain states that do not duplicate existing statuses, transition guards, unit tests with fakes | Tests for F01, F02, F06, F07, F08, F09 show `STORE_WRITES=0` on deny. HTTP recovery remains 403 | Database migration, provider |
| 0.3 Persistence and concurrency | Operation row, idempotency key, fencing token, append-only audit insert, lock order preserved | Migration reviewed separately. Concurrent test on an isolated database. No role grant is inserted | Operator network |
| 0.4 Reconciliation | Adapter interface and a mock that can return accepted, rejected, timeout, and contradiction | Timeout stays `RESULT_UNKNOWN`. Contradiction stays `MANUAL_REVIEW_REQUIRED`. Mock makes no socket call to an operator | Real operator |
| 0.5 Operator adapter pilot | One named operator, one signature kind, after security and legal approval | A written provider idempotency contract or an explicit at-least-once limitation. Keys stay out of logs | Production |
| 0.6 Controlled staging pilot | Staging only under a separate infrastructure authorization | Tenant isolation and audit sample recorded without secret values | Production rollout |
| 0.7 Production readiness | Load, security review, rollback, operator runbook | Independent review of the release candidate. `SIGNED` is still not treated as legal confirmation by itself | Unattended enablement |

Each stage needs its own controller authorization. Completing 0.1 does not authorize 0.2.

Rollback for a later persistence stage is a forward migration or a feature flag that keeps HTTP recovery at 403 and stops the reconciler. It is not a deletion of signature rows.

## Dependencies

- Pull request #360 caller authentication, still unapproved as an implementation.
- Pull request #371 permission matrix. `document.sign` is not granted.
- A legal decision for qualified signature and the applicable operator. Not made here.
- GAP-B-001 completeness remains open and is not this roadmap.
