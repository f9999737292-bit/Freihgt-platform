# ADR-EDO-011: Production GOST verifier

## Status

Proposed — discovery only (EDO-0.3-I4C-A). No implementation, migration, package install, or staging change is authorized by this record.

```text
QUALIFIED_VALID_AVAILABLE_NOW=NO
QUALIFIED_DOCUMENT_TRUST_READY=NO
LEGACY_SIGNING_SESSION_BYPASS_REMAINS=YES
ENGINE_SELECTION=BLOCKED
READY_FOR_I4C_B_IMPLEMENTATION=NO
COMPLIANCE_CLAIM=NONE
```

## Context

I4B on `226f9eff` stores a detached signature blob and append-only evidence, and it returns `PENDING` / `VERIFIER_UNAVAILABLE` from `UnavailableSignatureVerifier`. The evidence check constraint rejects every other status and reason. Effective status is the latest evidence row for `QUALIFIED_CADES_BES` / `v1`. The legacy signing session can still store `VALID` and set `DocumentStatus=SIGNED` without this verifier.

ADR-EDO-010 requires a conformity-confirmed signature means before a qualified `VALID`. It does not name a product. CryptoPro is not required by 63-FZ. A pure Go or OpenSSL-compatible implementation may not emit a qualified result.

This phase had to name the engine, the exact build, and the public evidence that the build can verify detached CAdES-BES under GOST R 34.10-2012 and GOST R 34.11-2012 at 256 and 512 bits, on a Linux server, without a signer private key. That evidence is not in hand.

## Decision

1. `ENGINE_SELECTION=BLOCKED`. No primary engine, vendor, or build is chosen. CryptoPro CSP 5.0 R3, КриптоПро ЭЦП SDK, КриптоПро SVS, ViPNet CSP 4.4.8, and an EDO operator API were examined and none is selected. Familiarity is not a selection criterion.
2. `CONFORMITY_CONFIRMATION=UNVERIFIED` for every candidate. Vendor pages and certificate-index cards are not a substitute for the certificate scope of the exact verification means. Where only vendor marketing or a press release supports a conformity claim, the claim stays unverified.
3. `READY_FOR_I4C_B_IMPLEMENTATION=NO`. I4C-B does not implement an adapter, open `VALID` or `INVALID`, or create a migration until the missing evidence listed in the architecture note is attached and this block is revised.
4. The I4B foundation stays: blob storage, the verifier port, `UnavailableSignatureVerifier`, policy-scoped evidence, required binary idempotency, evidence bound to the server blob, the database gate that allows only `PENDING` / `VERIFIER_UNAVAILABLE`, and the legacy signing-session bypass. This discovery found no blocking incompatibility in that foundation.
5. The integration boundary is frozen:

```text
document-service
        |
SignatureVerifier port
        |
qualified verifier adapter
        |
selected engine
```

`VERIFIER_TOPOLOGY=LOCAL_DAEMON`. Vendor APIs stay behind the adapter. Signature bytes do not leave BINTRANS. IPC, health, license path, and package update remain unknown until a build is pinned.

6. `PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO`. `PRIVATE_KEYS_STORED_BY_BINTRANS=NO`. A candidate that needs the signer's private key to verify is rejected.
7. The first format stays detached CAdES-BES. The qualified allowlist stays GOST R 34.10-2012 and GOST R 34.11-2012, both 256 and 512. GOST R 34.10-2001, MD5, SHA-1, XMLDSig, embedded PDF signatures, and bare PKCS#7 are rejected. Those allowlist flags are policy. They are not a claim that an examined product implements both bit lengths.
8. Cryptographic interpretation belongs to the engine: CMS, CAdES-BES profile, signed attributes, message digest, signing-certificate-v2, signature math, certificate extraction, chain building, certificate validity, revocation, timestamp validation, and trust-anchor path building. The adapter owns the algorithm allowlist, using identifiers the engine reports, plus size limits, tenant isolation, stored-object integrity, policy selection, deadlines, result mapping, and audit persistence.
9. Trust anchors come from the head certification-center channel `https://e-trust.gosuslugi.ru`, provisioned out of band. `APPLICATION_REPO_CONTAINS_ROOTS=NO`. This phase did not download roots.
10. Revocation failure, OCSP failure, network timeout, engine timeout, and an unknown engine error stay `PENDING`. Confirmed revocation is `INVALID` / `CERT_REVOKED`. `INFRA_FAILURE_TO_INVALID=NO`. Caller URLs are never fetched. Private and metadata addresses are denied. Redirects are denied.
11. Caller signing time is not trusted. A trusted timestamp is not mandatory for every qualified signature. Article 11, as recorded in ADR-EDO-010, uses reliable signing-time information when it exists and otherwise uses the verification date.
12. `000096` is not sufficient for the future policy. A later migration must widen the status and reason checks and add algorithm, signing-time source, revocation source, engine build, and trust-anchor digest fields. That migration is not created here.
13. `CRYPTOPRO_REQUIRED_BY_LAW=NO`. `CONFORMITY_CONFIRMED_ENGINE_REQUIRED_FOR_QUALIFIED_RESULT=YES`. `REGULATORY_REVALIDATION_BEFORE_PRODUCTION=YES`. `LEGAL_REVIEW_REQUIRED_BEFORE_PRODUCTION=YES`. Published order 796, on the Garant and Alta cards checked 2026-10-07, runs until 2027-01-01. The 30.07.2026 FSB draft is not law.

Full source cards, candidate matrices, and the missing-evidence list: [edo-0.3-i4c-gost-verifier.md](../architecture/edo-0.3-i4c-gost-verifier.md).

## Consequences

- Qualified `VALID` stays unavailable. The live writer remains `UnavailableSignatureVerifier`.
- `DocumentStatus=SIGNED` from the legacy signing session is still not document-level trust closure.
- I4C-B cannot lawfully treat this record as permission to install КриптоПро, ViPNet, or SVS.
- A later selection needs the certificate scope, the exact build, detached CAdES-BES verification, both GOST 2012 lengths, Linux server viability, and a license that does not require signer private keys. Until that revision, the daemon's IPC and package pin stay unknown on purpose.
- `000096` continues to refuse `VALID` and `INVALID`.

## References

- ADR-EDO-010
- ADR-EDO-008
- Federal Law 06.04.2011 No. 63-FZ, article 11
- FSB order 27.12.2011 No. 796
- FSB order 04.12.2020 No. 556
- Ministry of Digital Development order 06.11.2020 No. 580
