# ADR-AN-003: Time, late events, and restatement

## Status

Accepted for the Analytics-0.1B architecture freeze. Business close length is not invented.

```text
ADR_AN_003=ACCEPTED
CANONICAL_STORAGE_TIMEZONE=UTC
DEFAULT_REPORTING_TIMEZONE_POLICY=TENANT_EXPLICIT_DEFAULT_REQUIRED
LOCATION_TIMEZONE_USAGE=KPI_DEFINITION_ONLY
BUSINESS_CLOSE_DURATION=TBD
RESTATEMENT_WINDOW=BUSINESS_DURATION_TBD
HARD_CLOSE_POLICY=PUBLISHED_SNAPSHOT_IMMUTABLE
SOFT_CLOSE_POLICY=PRELIMINARY_PERIOD_MAY_ACCEPT_LATE_EVENTS_AND_MUST_MARK_RESTATED
LATE_EVENT_AFTER_CLOSE_POLICY=APPEND_CORRECTION_DO_NOT_REWRITE_SNAPSHOT
IDEMPOTENT_REPLAY_POLICY=SAME_SOURCE_EVENT_IDENTITY_IS_ONE_EFFECT
CURRENT_STATE_IS_HISTORY=NO
```

## Context

0.1A found UTC `TIMESTAMPTZ` storage and an undefined reporting timezone. Location rows default to `Europe/Moscow`. That default is not a reporting policy. No cross-domain restatement policy existed. TMS keeps the first actual timestamp. Terminal stops are immutable.

## Decision

Storage and instant comparisons use UTC. A local business day uses the tenant's explicit reporting timezone. If the tenant has none, day grouping is blocked. A location timezone is used only when the KPI definition requires that location's day.

Business event time is the KPI clock when the source has it. `created_at` and `updated_at` do not replace a missing business event time. The row is excluded and completeness is partial, or the KPI stays blocked.

A published analytics snapshot is immutable. A late event after that publication appends a correction and a new snapshot version. A preliminary period may accept late events and must be marked restated. Replay of the same source event identity has one effect. No legal or accounting close length is set.

## Consequences

Analytics-0.2 does not bucket by local day. On-time delivery version 1 compares UTC instants. Historical trends wait until the source history class supports them.
