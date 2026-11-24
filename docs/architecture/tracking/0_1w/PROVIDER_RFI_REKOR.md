# RFI-4: Sigstore — public Rekor

Status: `NOT_SENT`. `PROVIDER_REPLY=NOT_RECEIVED`. Prepared 2026-11-18. Do not send until the controller names a recipient.

`CONTACT_NOT_VERIFIED` for this RFI.

The address `security@sigstore.dev` is published for vulnerability reports only.

https://github.com/sigstore/community/security/policy

This qualification is not a vulnerability report. Do not send it to that address.

## What we already take from public documentation

Rekor is a transparency log. The specification says a transparency system does not itself make history immutable. It lets a witness who kept an older checkpoint detect a mutation or a removal. Witnesses must operate in a trust domain separate from the log operator. The log assigns the index. A new shard starts that index at zero. Entries are signed artifacts of a pluggable type. HashedRekord records a digest. It does not record `(namespace, incarnation, seq, predecessor_digest)` and it does not reject a writer whose sequence is not the exact successor. Inclusion proofs and signed checkpoints are the evidence the log does define. No price and no availability contract for `rekor.sigstore.dev` were found. A private Rekor would be another service the operator administers. That is not authorized by this request.

https://docs.sigstore.dev/logging/overview/

https://github.com/sigstore/architecture-docs/blob/main/rekor-spec.md

## Questions

### Q1 — Exact successor

Does any supported entry type reject a submission unless `seq` is exactly one greater than the stored sequence for a caller-supplied namespace and incarnation, with a matching predecessor digest? The log-assigned index is not that sequence.

Evidence: the schema and the conflict response. A `409` for a duplicate artifact is not this rule.

### Q2 — Durable acknowledgement

Is a signed entry timestamp a promise to include the entry later, or a statement that the entry is already in the tree? After an inclusion proof is returned, can a later checkpoint omit that leaf?

Evidence: the specification sections for the signed entry timestamp, the inclusion proof, and the consistency proof.

### Q3 — Unknown outcome

If the client loses the response, how does it discover whether the entry was integrated? Is the retry idempotent for the same signed artifact, and does that idempotency cover an application sequence?

Evidence: the lookup key and the result of a retry.

### Q4 — Account-owner deletion

Can the log operator, or anyone else, remove an integrated entry or publish a new tree that drops it? The specification already says a witness can detect a split view. State who is obliged to refuse service when that happens, and whether BINTRANS can be that party without running the log.

Evidence: the operator's powers and the witness's powers.

### Q5 — Rollback and restore

Can a shard rotation, a new tree, or a restored log present a lower size that omits receipted entries? Detection by a witness who stored the old checkpoint is the current documented behavior. State whether anything prevents the new tree from being the one new clients accept.

Evidence: the shard and checkpoint rules.

### Q6 — Independent administrator

Is there a supported deployment in which a custodian who is not a BINTRANS administrator runs or contracts the log, BINTRANS can only submit and read, and BINTRANS cannot choose the checkpoints? The public instance already has an operator other than BINTRANS. State whether that instance is a supported dependency for a private sequence, or only a public good for signed artifacts.

Evidence: the operating model and its support terms.

### Q7 — Verifiable evidence

Can a receipt be stored outside the log and verified offline against a witness checkpoint, for an entry that is not a software-signing event but the BINTRANS tuple?

Evidence: the entry type that would carry that tuple, or a statement that no such type exists.

### Q8 — Availability and partition

What does `rekor.sigstore.dev` return when it cannot integrate an entry? Is there a rate limit, an availability target, and a response the issuer must treat as refusal? An empty or unreachable log must not mean sequence zero.

Evidence: the published limit and the error.

### Q9 — Recovery

If the log remains available, can a client fetch every entry from an old checkpoint to the new one and prove the log only grew?

Evidence: the consistency-proof API. A client that did not keep the old checkpoint is part of the answer.

### Q10 — Pilot and cost

Is there a price, a region, a minimum fee, and a test environment for either the public log or a supported private log? If the public log is free and unsupported for this use, say so. A private log that BINTRANS would operate is outside this request.

Evidence: a published price or a written "no service offering". `PRICE_NOT_VERIFIED` remains until one of those exists.

A reply that restates "append-only transparency log" without the successor rule and the witness obligation does not qualify Rekor.
