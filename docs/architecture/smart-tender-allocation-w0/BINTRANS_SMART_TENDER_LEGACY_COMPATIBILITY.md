# BINTRANS Smart Tender — Legacy Compatibility (Spec v0.9)

**Main:** `892361db87985f35132aef86550a025909af648a`  
**Decisions:** D1 parallel campaign types; D2 OPTION_A; D7 no PARTIALLY_AWARDED for allocation  

---

## Immutable legacy contracts

| Contract | Evidence | Rule |
|---|---|---|
| One award per event | `000038` `uq_rfx_award_event UNIQUE (rfx_event_id)` | **KEEP** — never multi-row awards for multi-carrier |
| AwardResponse → AWARDED | `evaluation_service.go` | Unchanged API |
| Second award conflict | repository + enterprise tests | Unchanged |
| Multi-lot TO from one award | `award_conversion.go`, `000039` | Unchanged for legacy |
| Conversion requires AWARDED | not PARTIALLY_AWARDED | Unchanged |
| Questionnaire versioning | `version_lifecycle.go`, `000068` | ≠ commercial rounds |
| Late questionnaire | `000072`, late_submission_* | Unchanged E7 |
| Gateway buyer/carrier RFx | `rfxrbac/policies.go` | Additive policies only under allocations namespace |

---

## Parallel graph (OPTION_A)

```
LEGACY:  RfxEvent → RfxAward (1:1) → AwardTO scopes → TransportOrder
NEW:     RfxEvent → ALLOCATION_CAMPAIGN → Rounds/Snapshots/Proposal/Decision
              → Commitment → Appendix/EDO(mock) → Ledger → TransportOrder
```

No synthetic legacy Award for each carrier. Reporting uses allocation tables for campaigns.

---

## Regression gates (every allocation PR)

- T-E2E-01 / legacy single-winner path  
- Award→TO integration  
- Late submission + version lifecycle suites  
- Gateway RFx RBAC parity  
- Must not modify `uq_rfx_award_event`  

---

## PARTIALLY_AWARDED

Legacy FSM may retain the string/command.  
`ALLOCATION_CAMPAIGN`: use Allocation Decision FSM; do not map multi-carrier shortfall to event `PARTIALLY_AWARDED`.
