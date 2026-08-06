# Control Tower Shadow Observation — Daily Report

Copy this template for each observation day. Store completed reports in protected storage outside Git.

---

## Report metadata

| Field | Value |
|-------|-------|
| Date | |
| Environment | selectel-staging |
| Deployed SHA | 7d560a3 |
| Migration version | 18 |
| Observation day | Day 0 / Day 1 / … / Day 7 |

---

## Cohort and convergence

| Field | Value |
|-------|-------|
| Cohort size | |
| MATCH count | |
| Mismatch count | |
| Incomplete count | |

---

## Consumer and errors

| Field | Value |
|-------|-------|
| Maximum lag | |
| Lag recovery duration | |
| Dead-letter delta | |
| Offset commit error delta | |
| Gateway 5xx delta | |

---

## Latency

| Field | Value |
|-------|-------|
| Legacy p95 | |
| Read-model p95 | |
| Relative latency | |

---

## Operations

| Field | Value |
|-------|-------|
| Rebuild operations | |
| Rollback operations | |
| Open incidents | |

---

## Safety

| Field | Value |
|-------|-------|
| Primary status | **DISABLED** |
| Public source | LEGACY (required) |

---

## Daily verdict

| Verdict | Criteria |
|---------|----------|
| **PASS** | Primary disabled; public source legacy; all converged cohort tenants MATCH; dead-letter delta=0; offset commit errors delta=0; no sustained mismatch; consumer lag recovered; no stuck rebuild job; no data-loss incident |
| **WARN** | Brief expected propagation lag; planned restart backlog; approved maintenance only |
| **FAIL** | Sustained mismatch; dead-letter growth; offset reset; public source changed; primary enabled; rollback after live write; data loss; unknown status omitted |

**Daily verdict:** PASS / WARN / FAIL

**Notes:**

---

## Observation day numbering

| Day | Purpose |
|-----|---------|
| Day 0 | Deployment baseline |
| Day 1–6 | Daily observation |
| Day 7 | Final gate |

Window: 7 consecutive calendar days (minimum 5 business days).

Window starts only after: deployment healthy, migration version=18, cohort manifest approved, baseline snapshot completed.

---

## Final gate (Day 7)

After Day 7, complete sign-off templates and record overall PASS/FAIL for the seven-day window.

Do **not** declare primary canary ready until explicit post-observation review.
