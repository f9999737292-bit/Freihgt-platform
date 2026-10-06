# ADR-EDO-010: Attachment signature verification

## Status

Proposed — architecture only (EDO-0.3-I4A-R1). No implementation, migration, or API change is authorized by this record.

## Context

`AttachSignature` stores caller metadata and forces `verification_status` to `UNVERIFIED`. Migration `000095` names `VALID`, `INVALID`, and `PENDING`, then rejects every status except `UNVERIFIED`. The signature blob is not stored. No certificate, chain, revocation, timestamp, GOST, CryptoPro, or operator check exists on this path. A separate signing-session insert writes `VALID` and can move a document to `SIGNED` without a cryptographic check. That path is not evidence of a qualified signature.

Qualified recognition under 63-FZ article 11 is a server-derived result. It requires an accredited issuer, certificate and key validity at the article 11 time base, a positive signature and integrity check, and signature means that have a conformity confirmation. Caller metadata cannot produce `VALID`.

## Decision

1. Attaching a signature records claim A, metadata accepted. It does not record cryptographic validity, a trusted certificate, validity at signing time, or a legally trusted qualified signature.
2. `VALID` is written only by the server verifier. `UNVERIFIED` and `PENDING` are not trusted. `INVALID` is not trusted. Network and engine failure stay `PENDING`.
3. The first format is detached CAdES-BES over the stored attachment bytes. XMLDSig, embedded signatures, and PKCS#7 without the CAdES signed attributes are later work.
4. The application port is `VerifySignature(attachment, signature, format, policy)`. The engine is replaceable. Qualified `VALID` uses a conformity-confirmed means. A pure Go or OpenSSL-compatible implementation may not emit qualified `VALID`. CryptoPro is a candidate engine, not a legal requirement. An EDO operator is not the attachment verifier.
5. GOST R 34.10-2012 with GOST R 34.11-2012 is the qualified algorithm allowlist. GOST R 34.10-2001, MD5, and SHA-1 are rejected.
6. Article 11 does not require a trusted timestamp merely because a qualified signature exists. If reliable signing-time information exists, certificate and key validity are evaluated against that signing time. Otherwise the applicable check uses the verification date. A later timestamp or TSP rule may strengthen evidence. That rule is an architecture policy, not an extra statutory condition.
7. Attachment verification is fail-closed. Document trust is not. The legacy signing session can persist `VALID`, complete the session, and set `DocumentStatus=SIGNED` without the attachment verifier. That status must not be read as cryptographic verification, qualified verification, or a legally trusted qualified electronic signature until the legacy path is reconciled. `LEGACY_SIGNING_SESSION_REMEDIATION_REQUIRED=YES`. `QUALIFIED_DOCUMENT_TRUST_READY=NO`.
8. I4B stores the detached CAdES-BES blob, adds the evidence schema and the verifier port, and returns `PENDING` / `VERIFIER_UNAVAILABLE` when the production engine is missing. The client cannot set verification status. `I4B_CAN_PRODUCE_QUALIFIED_VALID=NO`. `I4B_CAN_MARK_DOCUMENT_TRUSTED_SIGNED=NO`. I4B does not change document state and does not remediate the legacy signing session.
9. `CRYPTOPRO_REQUIRED_BY_LAW=NO`. `CONFORMITY_CONFIRMED_ENGINE_REQUIRED_FOR_QUALIFIED_RESULT=YES`. The qualified engine stays replaceable.
10. `REGULATORY_REVALIDATION_BEFORE_PRODUCTION=YES`. Published FSB order 796 states effect until 2027-01-01. A 30.07.2026 FSB draft proposes removing that limit and is not treated as law. Recheck FSB and Ministry of Digital Development requirements before a production qualified rollout on or after that date.

Full design, failure codes, security limits, and source cards: [edo-0.3-i4a-signature-verification.md](../architecture/edo-0.3-i4a-signature-verification.md).

## Consequences

- `000095` cannot store a verification result until a later migration replaces the unverified-only trigger for evidence rows.
- Qualified `VALID` remains unavailable until a conformity-confirmed means and a reviewed legal edition are in place. CryptoPro is not legally mandated.
- `DocumentStatus=SIGNED` from the legacy signing session is not document-level trust closure.
- FSB order 796, as published, runs to 2027-01-01. A draft that would remove that date is not effective law.
- Private keys stay out of `document-service`.

## References

- ADR-EDO-008
- Federal Law 06.04.2011 No. 63-FZ, articles 8, 11, 12, and 13
- FSB order 27.12.2011 No. 796
- Ministry of Digital Development order 06.11.2020 No. 580
