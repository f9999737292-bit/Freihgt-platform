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
| durable result SearchID == persisted run.ID | `RunDurableChainSearch` + `resolveSearchID` | I2B-EXEC-02 (budget path) | I2B-EXEC-01 | PASS |
| legacy engine idempotency disabled | `Repository=nil` + cleared `IdempotencyKey` | I2B-EXEC-03/04 spy Claim/Commit | I2B-EXEC-05 | PASS |
| persisted horizon used by real engine | `PersistedHorizon*` + `resolveHorizon` | I2B-HORIZON-02 delayed clock | I2B-HORIZON-01/03/05 | PASS |
| caller cannot inject HorizonStart/End | `ClaimRequest` (no horizon fields) | I2B-HORIZON-04 | I2B-HORIZON-03 | PASS |
| base.Request.Now cannot extend durable deadline | `PrepareDurable` Now=time.Now + absolute ctx | I2B-DEADLINE-06/07 | I2B-DEADLINE-08/09 | PASS |
| nil duration resolver fails closed | `resolveDuration` ErrDurationResolverRequired | I2B-DUR-09 | I2B-DUR-10/11 | PASS |
| real engine consumes asymmetric stored duration | `RunDurableChainSearch`→Engine→Planner | I2B-DUR-14 post-claim policy | I2B-DUR-12/13/15 | PASS |
