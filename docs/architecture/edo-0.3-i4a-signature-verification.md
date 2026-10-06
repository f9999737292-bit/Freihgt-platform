# EDO 0.3 I4A — Attachment signature verification

## Status

```text
DOCUMENT_STATUS=ARCHITECTURE
PHASE=EDO-0.3-I4A-R1
BASE_SHA=23f6b626714a022b47a3948ebb727ce2b1061357
CHECKED_ON=2026-10-06
IMPLEMENTATION_AUTHORIZED=NO
COMPLIANCE_CLAIM=NONE
MIGRATION_CREATED=NO
PRODUCT_CODE_CHANGED=NO
```

This note designs server-side verification of a signature attached to an EDO file. It does not add a verifier, a migration, or an API change. It does not certify BINTRANS as an EDO operator or as a certified signature means.

Decision record: [ADR-EDO-010](../adr/ADR-EDO-010-attachment-signature-verification.md).

## 1. Current implementation

Proof is from `origin/main` at the baseline SHA.

### Attachment signature

`AttachmentService.AttachSignature` in `services/document-service/internal/service/attachment_service.go` accepts a finalized attachment and writes a row. The caller supplies `signature_format`, `signature_reference`, and optional certificate subject, issuer, serial, thumbprint, and signing time. The service forces `verification_status` to `UNVERIFIED`. Allowed format labels are `CAdES`, `PKCS7`, `XMLDSIG`, and `OTHER`. The reference is a trimmed string. The only content check is `RejectPrivateKeyMaterial`, which rejects a string containing private-key banners. The thumbprint, when present, must be 64 hex characters. Nothing parses ASN.1, XML, or a certificate.

`signature_reference` is stored in `documents.attachment_signatures.signature_reference` (`infrastructure/migrations/000095_edo_0_3_i2_signed_file_attachment.up.sql`). The repository insert does not read object storage. Integration tests pass values such as `objects/sig-1`. That string is not an object key and is not fetched.

There is no signature blob column, no signature object key, and no detached-or-embedded flag. The format label does not select a parser.

### Database gates already present

Migration `000095` defines `PENDING`, `VALID`, `INVALID`, and `UNVERIFIED`. Two triggers reject any insert or update whose status is not `UNVERIFIED`, on both `attachment_signatures` and `signature_verification_history`, with the exception text `cryptographic verifier is not available`. A further trigger makes the signature row immutable, including `verification_status`. History is append-only, and the unverified trigger still applies to new history rows. Audit event names `edo.signature.verified` and `edo.signature.verification_failed` exist in the check constraint. `AttachSignature` writes only `edo.signature.attached`.

`000095` therefore reserves the status vocabulary and blocks every status except `UNVERIFIED`. It cannot store a server verification result.

### Other signing path

`documents.signatures` is a different table, used by signing sessions. `signing_repository.go` inserts `verification_status = VALID` for `SIMPLE_ELECTRONIC`, `ENHANCED_UNQUALIFIED`, and `ENHANCED_QUALIFIED` with no cryptographic check. `ResolveSigningAfterSignature` moves the document to `SIGNED` when the completed signer count reaches the required count. `signature_payload_path` is an optional caller string. This path is outside the attachment verifier. I4A does not change it. A later trust policy must not treat that `VALID` or that `SIGNED` as cryptographic evidence.

### Capability matrix

| Capability | Result | Evidence |
|---|---|---|
| Signature metadata storage | YES | `attachment_signatures` columns and `AttachSignature` |
| Signature binary storage | NO | reference is unconstrained text; no object write |
| Cryptographic verification | NO | status forced to `UNVERIFIED`; no parser or signature library under `document-service` |
| Certificate chain validation | NO | certificate fields are caller text |
| Revocation check | NO | no CRL, OCSP, or registry client |
| Timestamp validation | NO | `signing_time` is a caller timestamp |
| GOST support | NO | no GOST implementation in the attachment path |
| CryptoPro, CSP, or CAdESCOM | NO | no integration in this repository |
| External operator verification | NO | ADR-EDO-008 keeps operator exchange as a future adapter; attachment attach does not call an operator |

`document-service` searches for `pkcs`, `cades`, `xmldsig`, `gost`, `ocsp`, `crl`, and `cryptopro` match only the format label and tests. OpenAPI text on the attach route states that no cryptographic verifier runs.

## 2. Trust model

Five claims stay separate:

| Claim | Meaning | Who may assert it |
|---|---|---|
| A. Metadata accepted | A finalized attachment has a stored signature record | Current `AttachSignature` |
| B. Cryptographically valid | The signature value matches the attachment bytes under an allowlisted algorithm | Server verifier |
| C. Trusted signer certificate | The signer certificate chains to a trust anchor accepted by the policy | Server verifier |
| D. Valid at signing time | The certificate and its key were in force at the signing instant the policy accepts | Server verifier, using the evidence defined below |
| E. Legally trusted qualified signature | B, C, and D hold, the issuer was an accredited certification center on the issue date, and the check used signature means that have a conformity confirmation | Server verifier under the qualified policy, after that means exists |

Attaching a signature is claim A. Claim A does not establish B, C, D, or E. Caller-supplied subject, issuer, serial, thumbprint, and signing time are claims, not observations. `VALID` on the attachment path is written only by the server from a completed positive verification. The current signing-session insert that stores `VALID` is not a model for this verifier.

```text
ATTACHMENT_VERIFICATION_FAIL_CLOSED=YES
DOCUMENT_TRUST_FAIL_CLOSED=NO
LEGACY_SIGNING_SESSION_BYPASS=YES
LEGACY_SIGNING_SESSION_REMEDIATION_REQUIRED=YES
QUALIFIED_DOCUMENT_TRUST_READY=NO
```

The attachment verifier is fail-closed. `UNVERIFIED`, `PENDING`, and `INVALID` cannot authorize a trusted signed claim. Document trust is not fail-closed. `AddSignature` persists `VALID`, can complete the signing session, and `ResolveSigningAfterSignature` can set `DocumentStatus` to `SIGNED` without the attachment verifier. `DocumentStatus=SIGNED` must not be read as cryptographic verification, qualified-signature verification, or a legally trusted qualified electronic signature until that legacy path is reconciled with server-derived verification. I4B does not fix that path and must not claim document-level trust closure.

## 3. First format

```text
FIRST_FORMAT=CAdES-BES detached CMS
DETACHED_OR_EMBEDDED=DETACHED
```

The attachment bytes already live in object storage, and the signature is a separate artifact. A detached CMS `SignedData` verifies those bytes without extracting a signature from PDF, XML, or another container.

CAdES-BES is the CMS profile that carries the signed attributes this check needs: `content-type`, `message-digest`, and `signing-certificate` or `signing-certificate-v2`. A bare PKCS#7 `SignedData` without those signed attributes does not bind the digest and the signer certificate strongly enough for the first policy. The existing `PKCS7` label can remain as a stored format name. The first verifier accepts it only when the blob is that CAdES-BES profile.

XMLDSig is a later format. It is the usual container for some tax XML documents, and it brings signature-wrapping and XXE risks that a CMS parser does not. Russian GOST is the algorithm family for a qualified signature. It is a requirement of the qualified policy, not a reason to accept every XML or embedded container in the first release.

This choice follows the attachment storage model and the qualified-signature check in 63-FZ article 11. It is not a choice of the smallest library.

## 4. Russian requirements

Sources opened on 2026-10-06. This note paraphrases duties. It does not reproduce statutory text. `COMPLIANCE_CLAIM=NONE`. The edition of 63-FZ in force on a future production date, including 04.08.2026 No. 315-FZ recorded in the EDO 0.3 regulatory register, still needs legal review before a qualified `VALID` is shown to customers.

### Legal requirements

Federal Law 06.04.2011 No. 63-FZ, Kremlin bank print [http://www.kremlin.ru/acts/bank/32938/print](http://www.kremlin.ru/acts/bank/32938/print):

- Article 11 recognizes a qualified signature only when its conditions hold together. The check this architecture relies on is:
  - positive verification that the signature belongs to the owner of the qualified certificate;
  - proof that the document was not altered after signing;
  - evaluation of certificate validity, and of the signature-key period named in the certificate;
  - use of signature verification means that have a conformity confirmation under the law, together with the signer's qualified certificate.
- The certificate must also have been issued by an accredited certification center whose accreditation was valid on the issue date.
- The time base for certificate and key validity is the statutory rule in the signing-time section below. A trusted timestamp is not required merely because a qualified signature exists.

```text
CRYPTOPRO_REQUIRED_BY_LAW=NO
CONFORMITY_CONFIRMED_ENGINE_REQUIRED_FOR_QUALIFIED_RESULT=YES
```

63-FZ does not name CryptoPro. A qualified result still requires some signature means that has a conformity confirmation. The engine remains replaceable.
- Article 12 states the functional duties of signature means. Part 5 says parts 2 and 3 do not apply to means used for automatic creation or automatic verification inside an information system. Part 5 does not remove the article 11 requirement that the qualified check use means with a conformity confirmation.
- Article 8 assigns the authorized federal body the head certification-center function and the publication of accredited-center lists.
- Article 13 requires a certification center to keep a registry of issued and revoked certificates and to give any person access to revocation information. The print records the 21.04.2025 No. 94-FZ amendment that the certificate ceases when the registry record is written, and that the registry must be updated within twelve hours.
- Article 2 defines a trusted time mark as reliable signing-time information created and checked by a trusted third party, a certification center, or an information-system operator, using means that passed conformity confirmation, under the procedure set by the authorized body.

### Signing time

Statutory rule, article 11: if reliable information about the signing moment exists, certificate validity and the signature-key period are evaluated against that signing time. If that information does not exist, the applicable validity check uses the verification date. Existence of a qualified signature does not by itself require a trusted timestamp.

Article 2 defines a trusted time mark as one form of that reliable information. Ministry of Digital Development order 06.11.2020 No. 580, registered by the Ministry of Justice on 28.12.2020 No. 61867, sets how that mark is created and checked. The Garant card is [https://base.garant.ru/400152022/](https://base.garant.ru/400152022/). The order allows the mark to be attached to the document or linked another way.

Architecture policy: a later timestamp or TSP rule may require a trusted time mark so that historical evidence survives after the certificate validity window. That is a BINTRANS policy choice. It is not an extra statutory condition on every qualified signature.

### Regulatory watch through 2027

```text
REGULATORY_REVALIDATION_BEFORE_PRODUCTION=YES
```

FSB order 27.12.2011 No. 796 approves requirements for signature means, under article 8 part 5. The published text, as amended, says the order is effective until 1 January 2027. Cards checked on 2026-10-06: [Garant](https://base.garant.ru/70139150/) and the [Alta-Soft publication](https://www.alta.ru/tamdoc/11a00796/). This discovery did not open a consolidated official publication on pravo.gov.ru, so the exact algorithm annex in force remains `LEGAL_VERIFICATION_REQUIRED` at implementation time.

A draft FSB order prepared on 30.07.2026, project ID 01/02/07-26/00169739, proposes deleting the words that limit order 796 to 1 January 2027. Draft card: [Garant](https://base.garant.ru/57061790/). The draft is not an effective order. This architecture does not treat it as law.

Before a production qualified-signature rollout on or after 2027-01-01, recheck the FSB and Ministry of Digital Development requirements then in force.

FSB order 04.12.2020 No. 556, Garant card [https://base.garant.ru/400163216/](https://base.garant.ru/400163216/), cites GOST R 34.10-2012 and GOST R 34.11-2012 as the signature and hash algorithms for the means it regulates, and records Rosstandart orders 07.08.2012 No. 215-st and No. 216-st. GOST R 34.10-2001 is not an acceptable algorithm for a new qualified check.

### Technical choices, not legal mandates

| Topic | Classification |
|---|---|
| Qualified certificate, accredited issuer, conformity-confirmed means, registry revocation status, and signing-time rule in article 11 | LEGAL_REQUIREMENT for claim E |
| CRL and OCSP as the wire protocols used to read that registry | TECHNICAL_IMPLEMENTATION_CHOICE |
| CAdES-BES as the first blob profile | TECHNICAL_IMPLEMENTATION_CHOICE consistent with detached CMS practice |
| CryptoPro CSP, ViPNet CSP, or another vendor | TECHNICAL_IMPLEMENTATION_CHOICE among conformity-confirmed means |
| A pure Go or OpenSSL implementation with no conformity confirmation | Insufficient for claim E |
| Trusted timestamp on every qualified signature | Not a statutory requirement. Article 11 uses signing time only when reliable signing-time information exists; otherwise it uses the verification date. A BINTRANS policy may still require a timestamp |
| Diadoc, SBIS, or another EDO operator | A separate exchange port under ADR-EDO-008, not the attachment verifier |

The head certification-center portal cited by accredited centers for root material is `https://e-trust.gosuslugi.ru`. This discovery did not download trust anchors. I4B must pin the official distribution channel at implementation time and must not trust a copied root from the application repository without a recorded source and digest.

CryptoPro is not named by 63-FZ. A qualified `VALID` waits until the deployed means has a conformity confirmation. Linux packages and per-instance licenses are vendor terms. This discovery did not open a CryptoPro license. Container deployment of a certified means is an operations task after the vendor and the license are chosen. Private keys are not required to verify.

## 5. Verifier contract

Application service owns the use case. The cryptographic engine is a port behind it. `AttachmentService` does not branch on CryptoPro, Selectel, or any other provider.

```text
VerifySignature(attachment, signature, format, policy) -> VerificationResult
```

`attachment` is the exact stored bytes, identified by the attachment SHA-256 already computed at upload. `signature` is the detached blob bytes, or a server object key that resolves to those bytes inside the tenant. A caller URL is not an input. `format` for the first policy is the CAdES-BES profile. `policy` identifies the rule set and its version.

`VerificationResult` contains:

- `status`: `VALID`, `INVALID`, or `PENDING`
- `reason_code`
- `verified_at`
- server-observed certificate subject, issuer, serial, thumbprint, `valid_from`, `valid_to`
- `signing_time` when the policy accepts a time source
- `chain_status`
- `revocation_status`
- `timestamp_status`
- `verifier_version` and `policy_version`

The result carries no private key, no signature key, and no caller-supplied certificate text presented as if the server had parsed it. Server-observed certificate fields stay distinct from the caller claims stored in `000095`.

`UNVERIFIED` remains the acceptance state of a signature record before the verifier has produced a result. It is not a `VerifySignature` success status.

## 6. Failure model

`INVALID` means the verifier completed a check and the material fails the policy. `PENDING` means the check did not finish. A timeout, a missing engine, or an unreachable registry stays `PENDING`.

| Reason | Status |
|---|---|
| `BAD_SIGNATURE` | INVALID |
| `CONTENT_HASH_MISMATCH` | INVALID |
| `CERT_EXPIRED` | INVALID at the policy time base |
| `CERT_NOT_YET_VALID` | INVALID at the policy time base |
| `CERT_REVOKED` | INVALID when the registry shows revocation at the policy time base |
| `UNKNOWN_CA` | INVALID when the chain ends outside the policy trust anchors |
| `CHAIN_BUILD_FAILED` | INVALID when the presented certificates are complete enough to show that no chain exists |
| `CRL_UNAVAILABLE` | PENDING |
| `OCSP_UNAVAILABLE` | PENDING |
| `TIMESTAMP_INVALID` | INVALID when a timestamp is present and fails; PENDING when the policy requires a timestamp and the stamp cannot be fetched |
| `UNSUPPORTED_ALGORITHM` | INVALID |
| `UNSUPPORTED_FORMAT` | INVALID |
| `VERIFIER_UNAVAILABLE` | PENDING |

`CERT_EXPIRED` uses the policy time base: signing time when a trusted time mark verifies, otherwise the verification day, matching article 11. A certificate that expires after a proven signing time is not `CERT_EXPIRED` for that historical check.

## 7. Security

- Signature blob limit: 1 MiB. Attachment bytes stay at the existing 10 MiB limit. Oversize input is rejected before parse.
- CMS parser limits: depth, element count, and certificate count. A certificate chain longer than 6 certificates, or more than 16 certificates in the blob, fails closed as `UNSUPPORTED_FORMAT` or `CHAIN_BUILD_FAILED` after the limits trip.
- ASN.1: reject indefinite constructs the parser does not bound, and reject trailing unexpected content in the signed attributes.
- XMLDSig, XML external entities, and signature wrapping are out of the first parser. No XML parser is added for I4B.
- Qualified algorithm allowlist: GOST R 34.10-2012 with GOST R 34.11-2012, 256-bit and 512-bit parameter sets. MD5, SHA-1, and GOST R 34.10-2001 are rejected with `UNSUPPORTED_ALGORITHM`.
- Local parse deadline: 5 seconds. Revocation and timestamp network deadline: 10 seconds. Expiry of the network deadline yields `PENDING`, not `INVALID`.
- CRL and OCSP URLs are taken from the certificate after parse. Requests go only to those URLs. Private, link-local, and metadata addresses are refused. Redirects are not followed. The verifier does not GET a caller-supplied `signature_reference`.
- Trust anchors are operator-provisioned material with a recorded digest. They are not embedded from an untrusted client and they are not logged.
- Verification evidence is append-only. Logs record reason codes and thumbprints, not signature bytes, certificate dumps, or secrets.
- The backend does not accept or store private keys. The existing private-key banner rejection stays.

## 8. Workflow

```text
ATTACH_SIGNATURE
  store detached blob under a server key
  record metadata as UNVERIFIED
  enqueue or run verifier
  PENDING while the check is unfinished
  VALID or INVALID when the check completes
  PENDING remains eligible for retry
```

The first implementation runs the verifier in the same request after the blob is stored, inside the deadlines above. The HTTP handler does not wait beyond those deadlines. A later worker may retry `PENDING` rows. Retry uses the same attachment bytes, signature bytes, and policy version. A completed `INVALID` or `VALID` is not recomputed unless the caller requests re-verification or the policy version changes. Each attempt appends evidence. Identical replay of the same idempotency key returns the stored result.

A later revocation does not rewrite an earlier evidence row. A new row may record that the certificate is revoked as of a later registry time. The historical row still says whether the signature satisfied the policy at its `verified_at` and time base.

Audit uses the existing names: `edo.signature.attached` on accept, `edo.signature.verified` when the result is `VALID`, `edo.signature.verification_failed` when the result is `INVALID`. `PENDING` writes an evidence row and does not emit the failed event, because the check did not fail.

## 9. Document state

```text
ATTACHMENT_VERIFICATION_FAIL_CLOSED=YES
DOCUMENT_TRUST_FAIL_CLOSED=NO
LEGACY_SIGNING_SESSION_BYPASS=YES
LEGACY_SIGNING_SESSION_REMEDIATION_REQUIRED=YES
QUALIFIED_DOCUMENT_TRUST_READY=NO
```

Attachment verification is fail-closed:

| Attachment verification | May authorize a trusted signed claim |
|---|---|
| `UNVERIFIED` | NO |
| `PENDING` | NO |
| `INVALID` | NO |
| `VALID` under the qualified policy | This is the only attachment result that can support that claim, and I4B does not produce it |

Document trust is not fail-closed. The legacy signing session can accept `AddSignature`, assign `VALID` in its persistence flow, complete the session, and transition the document to `SIGNED` without the attachment verifier. `DocumentStatus=SIGNED` must not be interpreted as cryptographically verified, qualified-signature verified, or a legally trusted qualified electronic signature until that path is reconciled with server-derived verification.

I4A and I4B do not change document transitions and must not claim document-level trust closure. `IsSignedEvidenceClass` must not be described as cryptographic trust. Closing the legacy bypass is a later gate, not an I4B task.

## 10. Data model gap

`000095` is not enough for I4B. A future migration, not created here, should add server evidence and the signature blob location.

Keep on the existing signature row, immutable: format, caller reference during the transition, caller certificate claims, caller signing time, and the original `UNVERIFIED` acceptance marker until a reviewed change says otherwise.

Add:

- server object key for the detached signature blob, unique, tenant-scoped, create-if-absent
- evidence table, append-only, tenant-scoped: status, reason code, verified_at, verifier version, policy id, policy version, server-observed certificate fields, chain fingerprint, chain status, revocation status, revocation evidence time, timestamp status, timestamp token object key when one is stored
- a narrow writer for evidence so application roles cannot insert `VALID`
- removal or replacement of `edo_i2_reject_unverified_claim` for evidence inserts only

Do not store private keys. Do not treat the caller thumbprint as the server thumbprint. The effective status for readers is the latest evidence row for the requested policy. While no evidence exists, the effective status stays `UNVERIFIED`.

## 11. Options

| | A. Pure Go or OpenSSL-compatible | B. CryptoPro or other certified CSP | C. External verification service or EDO operator | D. Hybrid |
|---|---|---|---|---|
| GOST | Possible as math, without conformity confirmation | Yes when the product implements GOST R 34.10-2012 | Depends on the provider | Qualified path uses B or a confirmed means; non-qualified experiments stay behind a different policy |
| Linux containers | Straightforward | Vendor packages and a license; not proven in this repo | No CSP in the application container | CSP or remote means stays outside the request path's domain code |
| Licensing | Go standard library is insufficient for GOST qualified checks | Commercial SKZI license | Provider contract | Pay only for the qualified engine |
| Operations | Low | License, updates, certified-build tracking | Network dependency and provider trust | Two adapters, one policy switch |
| Lock-in | Low, and legally incomplete for claim E | Vendor lock-in of the engine | Provider lock-in | Port isolates the vendor |
| Legal suitability for claim E | No | Yes if that exact build has conformity confirmation | Only if the provider's means satisfies article 11 and the evidence is retained locally | Yes on the qualified adapter |
| Offline | Yes | Yes for local CSP; revocation may be offline only with a fresh CRL cache | No | Local parse can run offline; revocation `PENDING` when the cache is stale |
| Audit | Full local evidence | Full local evidence if the application records the CSP result | Weaker unless the response and the input hashes are stored | Evidence schema is the same for every adapter |
| Scale | In process | Process or sidecar | Remote capacity | Same port, scale the adapter |

Recommendation: option D. The domain port and the evidence schema live in `document-service`. The qualified engine is a replaceable adapter. CryptoPro is an acceptable first engine candidate because it is a widely deployed certified means, not because the statute names it. A pure Go checker must not return qualified `VALID`. An EDO operator remains the exchange adapter from ADR-EDO-008 and is not required to verify an attachment the platform already stores.

## 12. Bounded I4B scope

I4B is a bounded foundation. It stores the detached blob, adds the evidence schema and the verifier port, and returns `PENDING` / `VERIFIER_UNAVAILABLE` when the production engine is absent. The client cannot set verification status. I4B produces no production `VALID` result and no document-state transition from the verifier.

```text
I4B_CAN_PRODUCE_QUALIFIED_VALID=NO
I4B_CAN_MARK_DOCUMENT_TRUSTED_SIGNED=NO
I4B_IMPLEMENTATION_SCOPE=
  detached CAdES-BES blob stored in the existing object store
  server-generated key, create-if-absent, 1 MiB limit
  verifier port and qualified policy identifier
  evidence migration described in section 10
  synchronous check with the deadlines in section 7
  missing engine returns PENDING / VERIFIER_UNAVAILABLE
  client cannot submit verification_status
  tests for the reason-code table and for UNVERIFIED/PENDING/INVALID
    not authorizing a trusted signed claim

OUT_OF_SCOPE=
  production GOST or CryptoPro adapter
  qualified VALID in a deployed environment
  document-state transition based on the I4B verifier
  legacy signing-session remediation
  XMLDSig, embedded PDF signatures, PKCS#7 without CAdES signed attributes
  CRL/OCSP network client until the engine task
  trusted-timestamp creation
  MChD
  EDO operator calls
  changes to signing-session VALID or document SIGNED transitions
  ListBucket or bucket-policy changes
```

`READY_FOR_I4B=YES` for that foundation. `READY_FOR_QUALIFIED_VALID=NO` and `QUALIFIED_DOCUMENT_TRUST_READY=NO` until a conformity-confirmed means is selected, the legacy signing-session path is reconciled, and the 63-FZ edition plus the FSB requirements in force are reviewed. On or after 2027-01-01 that review includes `REGULATORY_REVALIDATION_BEFORE_PRODUCTION`.

## References

- ADR-EDO-008, ADR-EDO-010
- `docs/architecture/edo-0.3-regulatory-source-register.md` LR-03-SIG-001
- 63-FZ Kremlin print, checked 2026-10-06
- FSB orders 796 and 556, MinTsifry order 580, via the cards cited above
