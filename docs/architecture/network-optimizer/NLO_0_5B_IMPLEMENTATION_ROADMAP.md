# NLO-0.5B implementation roadmap

NLO-0.5B0 is this policy freeze. Backhaul runtime is not authorized.

```text
NLO_0_5B0=PROPOSED_PENDING_CONTROLLER_REVIEW
NLO_0_5B_IMPLEMENTATION_AUTHORIZED=NO
BACKHAUL_RUNTIME_IMPLEMENTED=NO
```

| Wave | Content | Uses |
| --- | --- | --- |
| NLO-0.5B1 | Stop `ListMarketplaceLoads` at 1,000 rows in the existing order `created_at DESC, id`, after the current visibility predicate. Do not add a provider call in this wave. | Current marketplace query |
| NLO-0.5B2 | Send at most 25 prefilter survivors to the routing port. Stop at 4 matrix calls and 2 route calls. Honor the 5 second call timeout and the 30 second search watchdog. | Existing `BatchMatrix` and 2GIS adapter |
| NLO-0.5B3 | One-load corridor and four-leg ellipse feasibility, including service duration from the NLO-0.4 policy when a plan is built. | NLO-0.5A scope |
| NLO-0.5B4 | Score and persist that bounded run with the existing search record and score profile. | BNO-0.1C2 ranking |
| NLO-0.5B5 | Public exposure beyond the current next-load response, including Control Tower. | Not part of the first runtime authorization |

B1 through B4 do not add a search mode, a migration, or a freight-cost ledger. An accepted plan still uses the NLO-0.4 activation handshake. NLO-0.6, NLO-0.9, and fleet assignment stay out.
