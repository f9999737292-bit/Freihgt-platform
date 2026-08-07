# Control Tower Projection Rebuild — PR Handoff

Handoff package for manual PR creation and review. **Do not assume PRs are already opened.**

## Status

| Item | Status |
|------|--------|
| Feature branch | **Feature PR #1 MERGED** (`e3cc74fb1e03a0ac92aa4a8b6d749890ce2302b4`; runtime source `b75eb3d`) |
| Observation branch | **Observation PR #2 MERGED** (`64d218b6474d85075126cbf753fe73c1bbff94dd`; tooling source `9da6010`) |
| Repository release SHA | `64d218b6474d85075126cbf753fe73c1bbff94dd` |
| Staging execution | **STAGING_EXECUTION_PENDING_OPS_ACCESS** |
| Observation window | **NOT STARTED** |
| Primary readiness | **BLOCKED** |

---

## Feature PR

| Field | Value |
|-------|-------|
| Base | `main` |
| Head | `feat/control-tower-projection-rebuild-activation-v0.3` |
| Title | Control Tower projection rebuild: export, activation, rollback and live acceptance |
| Compare page | https://github.com/f9999737292-bit/Freihgt-platform/compare/main...feat/control-tower-projection-rebuild-activation-v0.3?expand=1 |
| Feature review SHA | `b75eb3d` |

### Manual PR creation steps

1. Open the feature compare URL above.
2. Confirm base = `main`.
3. Confirm compare branch = `feat/control-tower-projection-rebuild-activation-v0.3`.
4. Paste the approved title and PR body (from feature runbook / team template).
5. Assign reviewers: Go/backend, DBA, security/ops.
6. **Do not merge** until all required approvals are recorded.

---

## Observation PR

| Field | Value |
|-------|-------|
| Base | `main` |
| Head | `test/control-tower-staging-shadow-observation-v0.6` |
| Title | Control Tower staging shadow observation gates |
| Compare page | https://github.com/f9999737292-bit/Freihgt-platform/compare/main...test/control-tower-staging-shadow-observation-v0.6?expand=1 |
| Observation tooling SHA | `8708f2e` (plus handoff commit after merge review) |

### Dependency

**Depends on:** `feat/control-tower-projection-rebuild-activation-v0.3`

Observation tooling must not merge to `main` before the feature PR when it imports new commands, metrics, or contracts from that branch.

### Observation PR body (approved draft)

```markdown
## Scope

- Staging shadow observation collector
- Cohort aliasing and protected manifest
- Daily PASS/FAIL gates
- Provisional alert rules
- Rollback and consumer-restart drill wrappers
- Staging observation runbook and approval templates

## Safety

- Does not enable primary
- Does not switch the public response source
- Does not expose rebuild operations over HTTP
- Does not persist JWTs or snapshot payloads
- Does not reset Kafka offsets
- Does not run activation without explicit confirmation

## Dependency

Depends on the Control Tower projection rebuild feature PR.

## Current status

- Local tooling tests: PASS
- Selectel staging execution: PENDING OPS ACCESS
- Seven-day observation window: NOT STARTED
- Primary readiness: BLOCKED
```

### Manual PR creation steps

1. Open the observation compare URL above.
2. Confirm base = `main`.
3. Confirm compare branch = `test/control-tower-staging-shadow-observation-v0.6`.
4. Paste the observation PR body above.
5. Explicitly note dependency on the feature PR.
6. Assign Ops/observability reviewer.
7. **Do not mix** observation changes into the feature PR.
8. **Do not merge** until feature PR is approved and observation review passes.

---

## Recommended feature PR review order

Review commits in this order:

| Commit | Scope |
|--------|-------|
| `13d89d6` | Rebuild core infrastructure |
| `4c4091f` | Export/import |
| `d1e9ac2` | Activation/rollback |
| `d68f081` | Live acceptance |
| `a5163c3` | Kafka offset evidence (historical local acceptance SHA) |
| `b75eb3d` | AI audit remediation v1.2 (tenant isolation, backup nullability, CI/Docker alignment) |

### Key review areas

1. Snapshot protocol
2. `REPEATABLE READ` exporter
3. Persistent importer
4. Migration `000016`
5. Migration `000017`
6. Migration `000019`
7. Atomic activation
8. Exact rollback
9. Advisory lock placement
10. Same-group Kafka catch-up
11. Live acceptance scripts
12. **Absence of `primary`**

---

## Related handoff documents

| Document | Purpose |
|----------|---------|
| `docs/releases/CONTROL_TOWER_SHADOW_OBSERVATION_V0.6_RELEASE_MANIFEST.md` | Immutable release manifest |
| `docs/CONTROL_TOWER_SELECTEL_STAGING_DEPLOYMENT_HANDOFF.md` | Staging deployment procedure |
| `docs/CONTROL_TOWER_STAGING_OPERATOR_COMMANDS.md` | Operator command sheet |
| `docs/CONTROL_TOWER_STAGING_SHADOW_OBSERVATION.md` | Observation runbook |
| `docs/templates/CONTROL_TOWER_SHADOW_DAILY_REPORT_TEMPLATE.md` | Daily report template |
| `docs/templates/CONTROL_TOWER_SHADOW_SLO_APPROVAL.md` | SLO approval sheet |
| `docs/templates/CONTROL_TOWER_SHADOW_SECURITY_REVIEW.md` | Security sign-off |
| `docs/templates/CONTROL_TOWER_SHADOW_DBA_REVIEW.md` | DBA sign-off |
| `docs/templates/CONTROL_TOWER_SHADOW_OPS_APPROVAL.md` | Ops sign-off |
| `docs/CONTROL_TOWER_SHADOW_DASHBOARD_SPEC.md` | Dashboard specification |

---

## Seven-day observation gate (summary)

After staging deployment is healthy:

- **Day 0** — deployment baseline
- **Days 1–6** — daily snapshot + gate
- **Day 7** — final gate

Observation window starts only after: deployment healthy, migration version = 19, cohort manifest approved, baseline snapshot completed.

See `docs/templates/CONTROL_TOWER_SHADOW_DAILY_REPORT_TEMPLATE.md` for daily fields and PASS/FAIL semantics.

---

## Final PASS/FAIL after seven days

**PASS** requires all daily gates PASS (or approved WARN only), no primary activity, no sustained mismatch, no dead-letter growth, no offset commit errors, cohort fully converged.

**FAIL** triggers: sustained mismatch, dead-letter growth, offset reset, public source change, primary enabled, rollback after live write, data loss, or unknown status omitted.

Primary canary remains **BLOCKED** until explicit post-observation review.
