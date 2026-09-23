# EDO 0.3 Test and Acceptance Strategy

## Status

```text
DOCUMENT_STATUS=DISCOVERY
TESTS_ADDED_IN_THIS_CHANGE=NO
IMPLEMENTATION_AUTHORIZED=NO
BROWSER_ACCEPTANCE_REQUIRED_FOR_W0=NO
```

W0 has no product behavior to exercise. Acceptance of W0 is document review: scope matches the EDO 0.2 final report, links resolve, and status does not say implementation has started.

## Existing tests that remain the regression floor

Later waves must not weaken:

| Test | What it already locks |
|------|------------------------|
| `TestDocumentServiceCreateVersionOnlyDraftOrRejected` | No new version on signed or archived documents |
| `TestDocumentServiceCancelSignedDocument` | Signed document is not cancelled |
| `TestDocumentServiceArchiveOnlySignedOrAccepted` | Archive transition guard |
| `TestSigningServiceAddSignatureCompletesDocument` | Signature completion path |
| `TestPODUploadIntentIdempotency` | POD idempotency key |
| `TestPODCrossTenantShipmentRejected` | POD tenant boundary |

Source: `services/document-service/internal/service/document_service_test.go` and `internal/integration/podupload/pod_upload_integration_test.go`. Those tests were not re-run for this docs-only discovery (`NOT_RUN`).

## Tests required before an implementation wave can be accepted

Not written now. Names describe intent for a future Task Contract.

| Future check | Level | Proves |
|--------------|-------|--------|
| Package create and seal in one tenant | Unit | Seal freezes membership |
| Relationship append is idempotent per tenant and key | Unit | Replay does not duplicate the edge |
| Relationship to a foreign-tenant document | Unit | Rejected or not found, without revealing the other tenant |
| Signed revision payload update | Unit | Rejected |
| Add file after `SIGNED` | Unit | Rejected. Closes the current `AddFile` gap |
| Cascade or soft-delete of a signed revision | Integration | Signed bytes and signature rows remain |
| Gateway tenant mismatch | Integration | Body `tenant_id` that differs from the trusted tenant is rejected on any new route |
| Certificate evidence has no private-key column | Migration review | Schema inspection |
| OpenAPI matches new routes | Contract | Only if wave I3 exists |
| Browser flow | Not required for variant A | No new UI is in the recommended scope |

Operator acceptance against a real EDI operator, GIS EPD, or Selectel WORM bucket is out of EDO 0.3. Those checks belong to TEDO and INFRA and stay `NOT_RUN`.

## Evidence a later wave must attach

- Commands actually executed and their results (`PASS`, `FAIL`, `NOT_RUN`, `BLOCKED`).
- Confirmation the diff is limited to the wave's allowed paths.
- Confirmation no secret values appear in logs or fixtures.
- Controller review id for that wave.

## W0 checks for this discovery

| Check | Expected |
|-------|----------|
| Markdown links among the new docs and to ADR/event files | Resolve to files in the repo |
| Official legal URLs | Hosts are pravo.gov.ru, publication.pravo.gov.ru, nalog.gov.ru, or mintrans.gov.ru |
| `git diff --check` | Clean |
| Secret scan of the new docs | No credentials |
| Diff paths | `docs/**` only |
| Status wording | Discovery awaiting review. Implementation not authorized |

## References

- [edo-0.3-implementation-waves.md](edo-0.3-implementation-waves.md)
- [edo-0.3-security-tenant-boundary.md](edo-0.3-security-tenant-boundary.md)
