# EDO 0.3 Risk Register

## Status

```text
DOCUMENT_STATUS=DISCOVERY
IMPLEMENTATION_AUTHORIZED=NO
```

Risks are open. None are closed by this discovery.

| ID | Risk | Evidence | Mitigation in discovery | Residual |
|----|------|----------|-------------------------|----------|
| R-01 | Scope creep into TEDO, billing, or TMS | Final report names document-service only. Nearby docs mention operator, UPD, and legs | Variant A and the out-of-scope table | Controller must reject a wave that edits those owners |
| R-02 | `ArchiveManifest` treated as done because ADR-EDO-007 says EDO-0.3+ | Wording is "0.3 or later". Archive matrix puts WORM at EDO-0.4+ / INFRA | Recommended variant leaves the manifest out | A metadata-only table could be mistaken for a legal archive |
| R-03 | Single `document_status` kept forever | Code column contradicts the state-machine freeze | Recorded as variant D, not silently "fixed" | Clients keep a collapsed status until a later wave |
| R-04 | Signed bytes replaced via `AddFile` or `ON DELETE CASCADE` | `DocumentService.AddFile` has no status guard. Migration uses cascade | Immutability is in variant A for a future wave | Still true in the current code. This PR does not patch it |
| R-05 | Client-supplied `tenant_id` copied onto new routes | `document_handler.go` | Security note forbids copying it | Existing routes are unchanged |
| R-06 | MChD GUID stored without a check | No column today. Variant C would add one | Variant C deferred. `LEGAL_VERIFICATION_REQUIRED` | Authority can be implied by a later careless column |
| R-07 | Legal edition drift | 63-FZ has published amendment 04.08.2026 No. 315-FZ. Consolidated effect not reviewed | Register marks edition `LEGAL_VERIFICATION_REQUIRED` | A build wave must not cite this file as legal sign-off |
| R-08 | False operator or accreditation claim | No official source names this platform as an operator | Explicit non-claim | Marketing or UI copy outside this PR |
| R-09 | Selectel staging described as a WORM archive | EDO 0.2 already says object lock is unverified | Repeated as `EXTERNAL_INFRA_VERIFICATION_REQUIRED` | INFRA-0.1 still open |
| R-10 | Event names invented in code before catalog update | Package and relationship events are not in the accepted catalog | W0 does not add catalog entries or producers | A later wave must update the catalog in the same review if it emits events |
| R-11 | Personal data in payload logs | JSON payloads are unconstrained | Logging constraint recorded | No runtime log audit was performed (`NOT_RUN`) |
| R-12 | Discovery branch mistaken for an implementation start | Status enum includes `IMPLEMENTATION` | All new docs say `IMPLEMENTATION_AUTHORIZED=NO` | Reviewer must keep the PR draft |

## References

- [edo-0.3-gap-analysis.md](edo-0.3-gap-analysis.md)
- [edo-0.3-regulatory-source-register.md](edo-0.3-regulatory-source-register.md)
- [edo-0.2-final-report.md](edo-0.2-final-report.md)
