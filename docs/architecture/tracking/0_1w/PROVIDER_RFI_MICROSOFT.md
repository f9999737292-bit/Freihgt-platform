# RFI-1: Microsoft — Azure Confidential Ledger

Status: `NOT_SENT`. `PROVIDER_REPLY=NOT_RECEIVED`. Prepared 2026-11-18. Do not send until the controller names a recipient.

This is a technical qualification, not a request for a sales proposal. BINTRANS needs an independent recovery anchor. Public pages already read for this request are PR #392 at `80747ebe117fa1a9f6d9430381ebd9c7d37c88df`, especially the 2026-11-18 re-read of [Write transaction receipts](https://learn.microsoft.com/en-us/azure/confidential-ledger/write-transaction-receipts) and the [service overview](https://learn.microsoft.com/en-us/azure/confidential-ledger/overview). A preview feature is not a production guarantee.

## Contact

`CONTACT_NOT_VERIFIED` as the recipient of this RFI.

A mailbox `accledgerdevs@microsoft.com` is published as the Azure Confidential Ledger team contact on the Power Automate connector page and as the Python client author address. Those pages do not say that the mailbox answers durability, deletion, or price questions. It is not authorized for use by this document.

https://learn.microsoft.com/en-us/connectors/acl/

https://pypi.org/project/azure-confidentialledger/

Azure technical support is a support-plan case, not a public mailbox.

https://azure.microsoft.com/en-us/support/

## What we already take from public documentation

- Data-plane entries are append-only. A receipt is a Merkle proof over `writeSetDigest`, `commitEvidence`, and `claimsDigest`.
- Preview REST `2026-07-31-preview` documents `waitForCommit=true` on `POST /app/transactions`. The response is sent after commit and can include the receipt. Azure support must enable the option. If the connection drops, the write may still be committed, and a retry can create a duplicate. The default path returns a transaction id at acceptance and the caller polls until `Committed`.
- A control-plane delete of the ledger resource is a hard delete. Data cannot be recovered. `AZURE_HARD_DELETE_RISK=OPEN`.
- No page documents a caller-defined compare-and-set of `seq_new = seq_current + 1`.
- The public price table did not render a numeric rate on 2026-11-17. `AZURE_PRICE=PRICE_NOT_VERIFIED`.
- The FAQ states that, after a ledger is created, Microsoft has no access to user management. That sentence is not a statement about resource deletion.

https://learn.microsoft.com/en-us/azure/confidential-ledger/faq

## Questions

Each answer needs a document or API reference, not a product summary. Until a reply arrives, every row stays `PROVIDER_REPLY=NOT_RECEIVED`.

### Q1 — Exact successor

Does any API accept one write only when the stored record for a caller-supplied `(namespace, incarnation)` is exactly `seq_current` with `predecessor_digest`, and commit `seq_current + 1` with the new digest, rejecting a concurrent writer?

If no such API exists, which documented primitive would implement it, and which race remains?

Evidence: the request and the error returned when the predecessor does not match. State whether the condition is evaluated inside the committed transaction.

### Q2 — Durable acknowledgement

When `waitForCommit=true` is enabled, what does the successful response guarantee after a node failure and a recovery of the ledger? Is "committed" a majority of enclaves, a disk flush, or a later receipt?

Evidence: the definition of `Committed` and the failure after which a returned receipt can still be absent.

### Q3 — Unknown outcome

The receipts page says a dropped connection can leave a committed write, and a retry can append a duplicate. Is there an idempotency key, supplied by the caller, that makes a retry return the original transaction instead of a second entry?

Evidence: the header or field name, and the result of a retry after a lost response.

### Q4 — Account-owner deletion

Can the subscription Owner, or a Microsoft operator, delete the ledger resource or a committed entry? Can a second administrator who is not in that subscription prevent the delete, or detect it before the history is gone?

Evidence: the control-plane operation and any lock that survives the Owner role.

### Q5 — Rollback and restore

Can the ledger be restored to an earlier transaction so that a previously receipted sequence disappears or becomes a lower sequence? Disaster-recovery replication is not itself an answer.

Evidence: whether a recovered ledger can omit a transaction that already had a receipt.

### Q6 — Independent administrator

Can an organization that does not administer BINTRANS own the subscription, while BINTRANS holds only a data-plane Contributor credential and cannot delete the resource, change users, or shorten any retention?

Evidence: the role list and the operations still available to the subscription Owner.

### Q7 — Verifiable evidence

Can the caller store the receipt outside Azure and later prove that exact entry against the ledger identity without asking the subscription Owner to approve the proof?

Evidence: the offline verification steps and which certificate must still be obtainable after the resource is deleted.

### Q8 — Availability and partition

What does the service return when it cannot commit? Is there a response that means "empty ledger, start at sequence zero"? What is the documented behavior under a zone failure?

Evidence: the error codes and the rule for an unavailable ledger.

### Q9 — Recovery of a continuous history

After a failure that does not delete the resource, can every receipted transaction be read back, in order, with the same digest?

Evidence: the read API and any gap the recovery is allowed to drop.

### Q10 — Pilot and cost

Publish, in USD, for one named region:

- price per ledger instance hour and per GB-month after the included 100 GB;
- minimum charge;
- request prices, if any;
- regions where the service is offered;
- how `2026-07-31-preview` and `waitForCommit=true` are enabled, and whether preview terms exclude a production guarantee;
- the price of a non-production ledger that production BINTRANS will not accept;
- the price of an independent audit log of control-plane deletes.

Evidence: the pricing page or a written quotation that cites the meter names. A range without meters is `PRICE_NOT_VERIFIED`.

## Microsoft-specific points that stay open

`AZURE_WAIT_FOR_COMMIT=DOCUMENTED_PREVIEW`. `AZURE_DURABLE_ACK=PARTIAL`. `AZURE_RETRY_IDEMPOTENCY=NOT_PROVEN`. `AZURE_EXACT_SUCCESSOR=NOT_DOCUMENTED`. `AZURE_HARD_DELETE_RISK=OPEN`. `AZURE_PRICE=PRICE_NOT_VERIFIED`.

A reply that confirms only `waitForCommit=true` does not qualify the service.
