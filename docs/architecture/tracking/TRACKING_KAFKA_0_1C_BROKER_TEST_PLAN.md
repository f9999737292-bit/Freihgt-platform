# Tracking Kafka 0.1C — isolated broker test plan

Status: plan only. `BROKER_STARTED=NO`. `BROKER_SAFETY_PROVEN=NO`.

No broker, proxy, staging cluster, or production cluster is started by this stage. PR #372 observed the current consumer on an isolated Redpanda and is not this matrix. kfake is not evidence for these cases. The client under test is `github.com/twmb/franz-go` v1.21.5 (`services/tracking-service/go.mod`).

PR #357 at `0cf21055dd2a91c5ef071d6982c1fcc70cd11670` remains the rebalance contract. This plan defines how a later isolated run may observe it. A passing run of the provable rows still leaves coordinator apply time unproven where this plan says so.

## 1. What an observation can show

| Moment | How a later test sees it | What that evidence is |
|---|---|---|
| Client formed the `OffsetCommit` | Test hook before the client write | Local only |
| Proxy released the request toward the broker | Proxy log after the hold | Delivery time, once the proxy is the only path |
| Coordinator processed the request | Not visible to this plan | Apply time stays unobserved |
| Client received the response | Client log | Response receipt |
| Committed offset changed | A later `OffsetFetch` | A stored offset after the fetch, not the apply instant |
| Partition owner changed | `DescribeGroups` or the assignment callback at one sample | A point-in-time membership sample |

`PROXY_FRAME_ORDER_IS_BROKER_ACCEPTANCE_TIME=NO`. A successful response that arrives after a rebalance sample shows that the client observed success after that sample. It does not show that the coordinator applied the commit after the rebalance.

## 2. Environment the later run must have

The run needs a controller authorization that this stage does not give.

- One isolated broker process, one topic, one partition, and group `tracking-execution-stop`.
- The process is not `freight_redpanda`, not the freight-platform compose network, and not a staging or production broker.
- Host port selection avoids the Windows excluded range `29186–29285`. A dual listener is required when an in-container admin client must not follow an advertised host port that is closed inside the container.
- Clients disable auto-commit.
- A proxy sits on the client path and can hold a request before forwarding it, and can hold a response after the broker has already produced it. Those are different holds.
- The proxy records the moment it releases a held request. It does not record coordinator apply time.
- `OffsetFetch` is a separate call after the response, not a field of the commit response.
- The first `Fetch` of the next owner is recorded with its offset.

A request that the proxy has already forwarded before the rebalance is `NOT_TESTABLE_WITH_CURRENT_ORACLE` for apply-versus-loss. Holding only the response is the same class. Neither result is converted into a pass.

## 3. Cases

Each case lists the stimulus, the expected stored outcome, and the oracle limit. Expected tracking rows refer to the future consumer. The current consumer is not modified here.

### 3.1 In-flight OffsetCommit

Stimulus: consumer A sends `OffsetCommit` for offset 11, and the membership sample changes while that RPC is outstanding.
Expected committed offset: unknown until `OffsetFetch`.
Limit: `NOT_TESTABLE_WITH_CURRENT_ORACLE` for whether the apply happened before or after the loss. `PARTIALLY_TESTABLE` for the later stored offset and for B's first `Fetch`.

### 3.2 Rebalance timeout

Stimulus: A stops heartbeats until the broker removes A. The session timeout default in this client is 45 seconds and the rebalance timeout default is 60 seconds. The test records the removal from a membership sample.
Expected committed offset: the last `OffsetFetch` value from before the removal, unless a commit response was already stored.
Limit: `PARTIALLY_TESTABLE` for removal and the following assignment. The timeout values are client defaults, not a measurement that the coordinator used those same numbers.

### 3.3 Partition owner change

Stimulus: A leaves, or the group rebalances, and B's assignment contains the only partition.
Expected: B's first `Fetch` starts at the `OffsetFetch` offset. No record before that offset is skipped.
Limit: `TESTABLE` for the fetch position against `OffsetFetch`. `PARTIALLY_TESTABLE` for the claim that every prior record had a terminal effect. The terminal effect is a database proof from `TRACKING_KAFKA_0_1C_PERSISTENCE.md`, which is not implemented.

### 3.4 Delayed request

Stimulus: A builds `OffsetCommit`. The proxy holds the request. A membership sample shows A absent and B assigned. Only then does the proxy release the request. The test reads the response, then `OffsetFetch`, then B's first `Fetch`.
Expected: if the stored offset moves because of A's held request after B owns the partition, the case fails the safety claim. B must observe every record that has no terminal database effect.
Limit: `PARTIALLY_TESTABLE`. The release time is visible. The coordinator apply instant is not. `DELAYED_REQUEST_TEST_DEFINED=YES`.

### 3.5 Delayed response

Stimulus: the proxy forwards the request immediately and holds only the response until after the membership sample.
Expected: the test records response time and a later `OffsetFetch`. It does not classify the commit as applied before or after the sample.
Limit: `NOT_TESTABLE_WITH_CURRENT_ORACLE` for apply-versus-loss. `DELAYED_RESPONSE_IS_NOT_PROOF=YES`.

### 3.6 Late commit, offset 30 after offset 50

Stimulus: B's commit of offset 50 is visible through `OffsetFetch`. A's earlier `OffsetCommit` of offset 30 is then released by the delayed-request hold.
Expected: a stored offset of 30 fails the case. The cursor must remain 50, and B's fetch must not skip a record without a terminal effect.
Limit: `PARTIALLY_TESTABLE` for this held-request order. A pair of requests already inside the broker is `NOT_TESTABLE_WITH_CURRENT_ORACLE` for which one the coordinator applied last. `LATE_COMMIT_REWIND_RISK=DOCUMENTED`.
`SINGLE_FLIGHT_BLOCKED_COMMIT` blocks a new local send after the process has recorded loss. It does not cancel an RPC that already left, and it does not stop the coordinator from storing a lower offset. A blocked local send is not a pass for this case.

### 3.7 Unknown commit result

Stimulus: the client times out or loses the response. The test then calls `OffsetFetch` and does not send a second commit for the same offset.
Expected: the consumer stays on that offset until the fetch shows it stored, and a stored offset still needs the database proof before the next record is eligible.
Limit: `TESTABLE` for the refusal to send a second commit and for the fetch. `PARTIALLY_TESTABLE` for the claim that the timeout means the broker did not apply the first request.

### 3.8 Restart and replay

Stimulus: the process stops after a database transaction commits and before commit response processing finishes. It restarts and fetches the same record.
Expected: with the current schema, replay is `NON_TERMINAL` because `last_event_id` is absent. The offset stays unchanged. Invalid JSON and an unknown type also stay uncommitted.
Limit: `TESTABLE` for the refusal. A future durable prior-effect proof waits for the persistence columns.

### 3.9 Consumer A then consumer B

Stimulus: A processes a record without a terminal effect and is removed. B receives the partition.
Expected: B fetches that record. B does not commit it merely because A attempted it.
Limit: `TESTABLE` for the fetch and the unmoved offset when no terminal row exists. `PARTIALLY_TESTABLE` while the terminal proof columns are absent.

### 3.10 Contiguous terminal prefix

Stimulus: offsets 0 and 1 have `COMMITTED_STATE_EFFECT` or a future `VERIFIED_EQUIVALENT_PRIOR_EFFECT`. Offset 2 does not.
Expected: `OffsetFetch` may show 2 and must not show 3. A gap before the first unproven record fails the case.
Limit: `PARTIALLY_TESTABLE` until the persistence proof exists. The prefix rule is a database rule plus `OffsetFetch`. It does not reveal apply time.

## 4. Pass rule for a later run

A later run may report a case as `TESTABLE` only for the rows this plan marks `TESTABLE`. A `PARTIALLY_TESTABLE` row can support its visible observations and still leaves `BROKER_SAFETY_PROVEN=NO`. A `NOT_TESTABLE_WITH_CURRENT_ORACLE` row cannot be turned into a pass by a successful response or by proxy order.

This stage executes none of the cases.
