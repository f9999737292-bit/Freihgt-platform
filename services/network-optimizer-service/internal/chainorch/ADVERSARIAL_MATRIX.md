# I2B Controller Remediation — Adversarial Matrix

| CONTRACT_REQUIREMENT | CODE_LOCATION | NEGATIVE_TEST | POSITIVE_TEST | RESULT |
|---|---|---|---|---|
| tenant in fingerprint | `fingerprint.go` `tenant_id=` | I2B-FP-02 | I2B-08 | PASS |
| stable horizon replay | `HorizonAnchorPolicy` / AvailableFrom/Until | I2B-FP-01 (no wall-clock) | I2B-FP-05 | PASS |
| internal search id generation | `orchestrator.go` Claim `uuid.New()` | I2B-FP-09 (no SearchID field) | I2B-01 | PASS |
| duration provenance server-owned | `Claim` + `DurationResolver` | I2B-DUR-02 | I2B-DUR-01 | PASS |
| immutable duration execution | `PrepareDurableChainRequest` / ApplyStored | I2B-DUR-04 | I2B-DUR-05 | PASS |
| absolute persisted deadline | `chain.Request.ComputeDeadline` + `effectiveComputeDeadline` | I2B-DEADLINE-02 | I2B-DEADLINE-01 | PASS |
| provider timeout <= remaining durable budget | `limitedProvider.Route` + ComputeDeadline | I2B-DEADLINE-03 | I2B-DEADLINE-03 | PASS |
| no compute in +5s grace | EnsureComputeAllowed + RunDurableChainSearch | I2B-DEADLINE-05 | I2B-DEADLINE-05 persist | PASS |
| atomic search+idempotency rollback | WithinChainSearch TX | I2B-ATOMIC-01 / I2B-02 | I2B-01 | PASS |
| concurrent same-key no orphan | Claim race | I2B-ATOMIC-02 orphan count | I2B-03 / ATOMIC-02 | PASS |
| owner fencing | MarkRunning / OwnerPersist / OwnerInsert | I2B-09 / I2B-23 | I2B-04 | PASS |
| late owner rejection | CompleteSearch after expiry | I2B-12 | I2B-11 grace persist | PASS |
| cancellation vs completion | CancelSearch / CompleteSearch | I2B-19 | I2B-16 | PASS |
| recovery vs completion | RecoverExpired / CompleteSearch | I2B-20 | I2B-07 | PASS |
| recovery idempotency | RecoverExpired | I2B-18 second no-op | I2B-17 | PASS |
| asymmetric pickup/delivery | ActionServiceSecondsFromSnapshot | (equal-only rejected removed) | I2B-DUR-06 | PASS |
| KNOWN_ZERO != UNKNOWN | fingerprint + ApplyStored | I2B-DUR-07 / I2B-15 | I2B-DUR-07 | PASS |
| durable result SearchID == persisted run.ID | `DurableSearchID` + result pin | I2B-EXEC-02 (budget path) | I2B-EXEC-01 | PASS |
| legacy engine idempotency disabled | `Repository=nil` + cleared `IdempotencyKey` | I2B-EXEC-03/04 spy Claim/Commit | I2B-EXEC-05 | PASS |
| persisted horizon used by real engine | `PersistedHorizon*` + `resolveHorizon` | I2B-HORIZON-02 delayed clock | I2B-HORIZON-01/03/05 | PASS |
| caller cannot inject HorizonStart/End | `ClaimRequest` (no horizon fields) | I2B-HORIZON-04 | I2B-HORIZON-03 | PASS |
| base.Request.Now cannot extend durable deadline | `PrepareDurable` Now=time.Now + absolute ctx | I2B-DEADLINE-06/07 | I2B-DEADLINE-08/09 | PASS |
| nil duration resolver fails closed | `resolveDuration` ErrDurationResolverRequired | I2B-DUR-09 | I2B-DUR-10/11 | PASS |
| real engine consumes asymmetric stored duration | `RunOwnedDurableChainSearch`→Engine→Planner | I2B-DUR-14 post-claim policy | I2B-DUR-12/13/15 | PASS |
| durable compute requires RUNNING | `RunOwnedDurableChainSearch` MarkRunning first | low-level PENDING reject | I2B-EXEC-06 | PASS |
| owner token required before compute | `classifyComputeStart` + MarkRunning | I2B-EXEC-07 | I2B-EXEC-06 | PASS |
| wrong owner => zero expensive work | `RunOwnedDurableChainSearch` | I2B-EXEC-07 | I2B-EXEC-01 | PASS |
| concurrent start => one compute | `MarkChainSearchRunning` race | I2B-EXEC-08 loser | I2B-EXEC-08 winner | PASS |
| terminal search => zero compute | `ErrSearchNotRunnable` | I2B-EXEC-10 | I2B-EXEC-06 | PASS |
| already RUNNING => no second compute | `ErrInProgress` | I2B-EXEC-11 | I2B-EXEC-06 | PASS |
| candidate IDs distinct from durable SearchID | `NewID` free; SearchID via DurableSearchID | I2B-EXEC-13 | I2B-EXEC-12 | PASS |
| candidate IDs unique across shortlist | engine candidate allocation | I2B-EXEC-12 dup check | I2B-EXEC-12 | PASS |
| real compute → terminal durable status | `ExecuteOwnedDurableChainSearch` | I2B-E2E-12/13 fail/cancel | I2B-E2E-01 COMPLETED | PASS |
| budget → durable FAILED | `classifyDurableTerminal` + FailSearch | I2B-E2E-12 | I2B-E2E-12 | PASS |
| context cancel → durable CANCELLED | CancelSearch cleanup ctx | I2B-E2E-13 | I2B-E2E-13 | PASS |
| real engine shortlist → chain_candidates | `ProjectDurableCandidates` | I2B-E2E-15 partial | I2B-E2E-02 | PASS |
| dense ordinal preservation (CHAIN-PERSIST-05) | ordinal 1..N engine order | I2B-E2E-03 | I2B-E2E-03 | PASS |
| UNFINISHED distinction | mapFeasibility ReasonUnfinished | I2B-E2E-07 / project_test | I2B-E2E-07 | PASS |
| known/unknown metric null semantics | json:"-" + nil preservation | I2B-E2E-09 unknowns | I2B-E2E-09 knowns | PASS |
| candidate fingerprint determinism | `ComputeCandidateFingerprint` | I2B-E2E-10 drift | I2B-E2E-10 | PASS |
| real counters persisted | `CountersFromEngineResult` | I2B-E2E-11 mismatch | I2B-E2E-11 | PASS |
| candidate failure → no partial COMPLETED | CompleteSearch TX rollback | I2B-E2E-15 | I2B-E2E-14 path | PASS |
| terminal replay → same shortlist + zero recompute | Claim OutcomeReplay + ErrSearchNotRunnable | I2B-E2E-17 | I2B-E2E-16 | PASS |
| private shipper/customer/contract absent | E2E-18 JSON scan | I2B-E2E-18 | I2B-E2E-18 | PASS |
| REJECTED never maps to COMPLETED | FailureUnexpectedEngineStatus | project_test reject | classify fail-closed | PASS |
| FAILED retains actual engine counters | `TerminalStatsFromEngine` + FailSearch | I2B-E2E-12/19 zero-reset | I2B-E2E-19 | PASS |
| CANCELLED retains actual engine counters | CancelSearch stats | I2B-E2E-13/20 zero-reset | I2B-E2E-20 | PASS |
| CANCELLED retains actual reason_counts | CancelSearch ReasonCounts | I2B-E2E-20 empty drop | I2B-E2E-20 | PASS |
| candidate_count equals actually persisted rows | TerminalStats.CandidateCount | I2B-E2E-15 partial COMPLETED | I2B-E2E-11/19/20 | PASS |
| canonical path ignores arbitrary caller counters | ExecuteOwned vs low-level FailSearch | I2B-E2E-22 arbitrary 99 | I2B-E2E-22 engine match | PASS |
| unexpected failure keeps measurable counters | RewriteEngineResultForTest + FailSearch | I2B-E2E-21 zero | I2B-E2E-21 | PASS |
