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
LATE_EVENT_CHANGES_DEFINITION_VERSION=NO
RESTATEMENT_PRESERVES_DEFINITION_VERSION=YES
PUBLISHED_SNAPSHOT_REVISIONING=YES
ANALYTICS_0_2_PUBLICATION_REVISION=NOT_APPLICABLE
GENERATED_AT_IS_SOURCE_WATERMARK=NO
FRESHNESS_FAIL_OPEN=NO
MISSING_SOURCE_WATERMARK_STATUS=UNKNOWN
```

## Context

0.1A found UTC `TIMESTAMPTZ` storage and an undefined reporting timezone. Location rows default to `Europe/Moscow`. That default is not a reporting policy. No cross-domain restatement policy existed. TMS keeps the first actual timestamp. Terminal stops are immutable.

## Decision

Storage and instant comparisons use UTC. A local business day uses the tenant's explicit reporting timezone. If the tenant has none, day grouping is blocked. A location timezone is used only when the KPI definition requires that location's day.

Business event time is the KPI clock when the source has it. `created_at` and `updated_at` do not replace a missing business event time. The row is excluded and completeness is partial, or the KPI stays blocked.

`DEFINITION_VERSION` changes only when the KPI's business meaning changes. `PUBLICATION_REVISION` changes when the same definition is republished because source data changed, a late event arrived, or a correction was applied. A late event does not change `DEFINITION_VERSION`.

A published snapshot is immutable. The same `KPI_ID`, `DEFINITION_VERSION`, and period may gain `PUBLICATION_REVISION=2` after a correction. Revision 1 remains. A preliminary period may accept late events and must be marked restated. Replay of the same source event identity has one effect. No legal or accounting close length is set.

Analytics-0.2 does not store a snapshot. `PUBLICATION_REVISION` is not part of that response. `generatedAt` is response time. Without a source watermark, `dataFreshness.status` is `UNKNOWN`.

## Consequences

Analytics-0.2 does not bucket by local day. On-time delivery version 1 compares UTC instants. Historical trends wait until the source history class supports them.
