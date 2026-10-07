# EDO 0.3 I4C-A — Production GOST verifier qualification

## Status

```text
DOCUMENT_STATUS=ARCHITECTURE
PHASE=EDO-0.3-I4C-A
BASE_SHA=226f9eff20743111d50c9e9a1df16b9bc2cacb51
CHECKED_ON=2026-10-07
IMPLEMENTATION_AUTHORIZED=NO
COMPLIANCE_CLAIM=NONE
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
QUALIFIED_VALID_AVAILABLE_NOW=NO
QUALIFIED_DOCUMENT_TRUST_READY=NO
LEGACY_SIGNING_SESSION_BYPASS_REMAINS=YES
ENGINE_SELECTION=BLOCKED
READY_FOR_I4C_B_IMPLEMENTATION=NO
```

This note qualifies a production verification engine for a future qualified-signature path. It does not install a CSP, change runtime code, create migration `000097`, permit `VALID` or `INVALID` in the I4B evidence gate, deploy to staging, or store a private key.

Decision record: [ADR-EDO-011](../adr/ADR-EDO-011-production-gost-verifier.md).

`QUALIFIED_VALID_AVAILABLE_NOW=NO`. `QUALIFIED_DOCUMENT_TRUST_READY=NO`. `LEGACY_SIGNING_SESSION_BYPASS_REMAINS=YES`.

## 1. I4B foundation confirmed on this baseline

Confirmed from `origin/main` `226f9eff20743111d50c9e9a1df16b9bc2cacb51`. These foundations stay. This discovery did not find a blocking incompatibility in them.

```text
SIGNATURE_BINARY_STORAGE=YES
VERIFIER_PORT=YES
DEFAULT_VERIFIER=UnavailableSignatureVerifier
DEFAULT_RESULT=PENDING / VERIFIER_UNAVAILABLE
EVIDENCE_TABLE=documents.signature_verification_evidence
POLICY_SCOPED_EFFECTIVE_STATUS=YES
BINARY_IDEMPOTENCY_REQUIRED=YES
EVIDENCE_REQUIRES_SERVER_BLOB=YES
DB_VALID_WRITE_BLOCKED=YES
LEGACY_SIGNING_SESSION_BYPASS_REMAINS=YES
QUALIFIED_DOCUMENT_TRUST_READY=NO
```

Migration `000096` stores detached signature bytes only by object key. Evidence checks allow `PENDING` and `VERIFIER_UNAVAILABLE` only. Evidence references the blob row. `attachment_signatures` stays the immutable `UNVERIFIED` acceptance row. Effective status is the latest evidence row for policy `QUALIFIED_CADES_BES` / `v1`. `UnavailableSignatureVerifier` returns that pending result and does not invent certificates. The legacy signing-session path can still write `VALID` and set `DocumentStatus=SIGNED`. That path is not this verifier.

## 2. Question this phase had to answer

A production verifier for a future qualified result needs one concrete engine, one exact build, public evidence of its capabilities, a conformity confirmation that covers that verification means, an integration boundary, trust-anchor and revocation handling, and license and operations prerequisites.

Those facts are not established. `ENGINE_SELECTION=BLOCKED`. The sections below record what was checked, what is unknown, and the architecture that stays frozen while the engine remains unselected.

## 3. Source rules

Regulatory claims use official legislation, FSB publications, Ministry of Digital Development publications, or the head certification-center channel. Engine claims use vendor documentation or the vendor's own certificate index. A vendor sentence that a product is certified is not treated as the certificate text.

Blogs, forums, reseller pages, and SEO articles are not evidence. In particular, a reseller page that attributes CAdES-BES to ViPNet CSP, a forum thread about `cryptcp`, and `description_csp_r3.pdf` on cryptopro.ru were not used: that PDF is the implementation description of КриптоПро CSP 3.6.1 (ЖТЯИ.00050-03), which cites GOST R 34.10-2001 and GOST R 34.11-94. It is not evidence for CSP 5.0 R3.

For each major claim:

```text
SOURCE=
SOURCE_TYPE=
CHECKED_AT=
SUPPORTED_CLAIM=
```

`SOURCE_TYPE` is one of `EFFECTIVE_LAW`, `EFFECTIVE_ORDER`, `DRAFT`, `VENDOR_DOCUMENT`, `VENDOR_CERTIFICATE_INDEX`, `LEGAL_DATABASE`, `BINTRANS_POLICY`.

## 4. Legal revalidation

Checked 2026-10-07. This note paraphrases duties. It does not reproduce statutory text. `COMPLIANCE_CLAIM=NONE`.

```text
CRYPTOPRO_REQUIRED_BY_LAW=NO
CONFORMITY_CONFIRMED_ENGINE_REQUIRED_FOR_QUALIFIED_RESULT=YES
REGULATORY_REVALIDATION_BEFORE_PRODUCTION=YES
LEGAL_REVIEW_REQUIRED_BEFORE_PRODUCTION=YES
```

### 63-FZ

```text
SOURCE=http://www.kremlin.ru/acts/bank/32938/print
SOURCE_TYPE=EFFECTIVE_LAW
CHECKED_AT=2026-10-06
SUPPORTED_CLAIM=Article 11 recognizes a qualified signature only when a positive ownership check, proof the document was not altered, certificate and signature-key validity evaluation, and conformity-confirmed signature means hold together. The law does not name CryptoPro. Article 11 uses signing time only when reliable signing-time information exists; otherwise the verification date. A trusted timestamp is not a universal statutory condition.
```

Carried from I4A. This pass did not open a newer edition that names a vendor. The edition in force on a production date, including amendments recorded in the EDO 0.3 regulatory register, still needs legal review before a qualified `VALID` is shown.

### FSB order 796

```text
SOURCE=https://base.garant.ru/70139150/
SOURCE_TYPE=LEGAL_DATABASE
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=The card reproduces order 27.12.2011 No. 796, as amended, and point 2 states that the order is effective until 1 January 2027.
```

```text
SOURCE=https://www.alta.ru/tamdoc/11a00796/
SOURCE_TYPE=LEGAL_DATABASE
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=Alta records the order as effective, in force from 27.04.2012, content amended from 01.09.2022 by FSB order 13.04.2022 No. 179, and ending 01.01.2027.
```

This pass did not open a consolidated text on pravo.gov.ru. The algorithm annex in force remains subject to legal review at implementation time. Order 796 is the published requirements instrument for signature means. It is not a draft.

```text
SOURCE=https://base.garant.ru/57061790/
SOURCE_TYPE=DRAFT
CHECKED_AT=2026-10-06
SUPPORTED_CLAIM=A draft FSB order prepared 30.07.2026, project 01/02/07-26/00169739, proposes deleting the 1 January 2027 limit. It is not an effective order.
```

### Qualified algorithms

```text
SOURCE=https://base.garant.ru/400163216/
SOURCE_TYPE=LEGAL_DATABASE
CHECKED_AT=2026-10-06
SUPPORTED_CLAIM=FSB order 04.12.2020 No. 556, as cited in I4A, names GOST R 34.10-2012 and GOST R 34.11-2012 for the means it regulates. GOST R 34.10-2001 is not the algorithm for a new qualified check.
```

This pass did not re-open the algorithm annex. `GOST_2001_ALLOWED=NO` is BINTRANS policy aligned with that I4A reading, not a new statutory quotation.

### What the law does not do

No source opened in I4A or this pass requires КриптоПро by name. A qualified result still requires some signature means that has a conformity confirmation. The engine stays replaceable. Before any production qualified rollout, and again before 2027-01-01, recheck the FSB and Ministry of Digital Development instruments then in force and obtain legal review.

## 5. Candidates

Ordinary verification must not require the signer's private key. Every candidate below is held to `PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO`. None of the vendor pages opened say that verifying a signature requires the signer's private key. A candidate that later does is rejected.

Unknown cells are `UNKNOWN`. They are not inferred.

### A. Local CSP / CAdES engine — КриптоПро CSP 5.0 with КриптоПро ЭЦП SDK

```text
PRODUCT=КриптоПро CSP 5.0 R3, with КриптоПро ЭЦП SDK asserted by the vendor to be certified inside that CSP
VENDOR=ООО КРИПТО-ПРО
EXACT_VERSION_OR_BUILD=UNKNOWN
LINUX_SUPPORT=VENDOR_CLAIMED
SERVER_SUPPORT=VENDOR_CLAIMED
CONTAINER_SUPPORT=UNKNOWN
CAdES_BES_VERIFY=UNVERIFIED
DETACHED_CADES_VERIFY=UNVERIFIED
GOST_34_10_2012=VENDOR_CLAIMED
GOST_34_11_2012=VENDOR_CLAIMED
GOST_256=UNKNOWN
GOST_512=UNKNOWN
CERT_CHAIN_VERIFY=UNKNOWN
CRL_SUPPORT=UNKNOWN
OCSP_SUPPORT=SEPARATE_PRODUCT_NAMED
TIMESTAMP_VERIFY=SEPARATE_PRODUCT_NAMED
CONFORMITY_CONFIRMATION=UNVERIFIED
CONFORMITY_DOCUMENT_REFERENCE=see certificate index below; certificate PDF not opened
CONFORMITY_SCOPE=UNKNOWN
CONFORMITY_VALIDITY_PERIOD=see dates below; scope of those dates is the named SKZI object
LICENSE_REQUIRED=YES
LICENSE_MODEL=UNKNOWN
PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO
OFFLINE_VERIFY=UNKNOWN
NETWORK_DEPENDENCIES=UNKNOWN
```

```text
SOURCE=https://cryptopro.ru/products/csp
SOURCE_TYPE=VENDOR_DOCUMENT
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=The CSP 5.0 product page says the product forms and checks an electronic signature, lists GOST R 34.10-2012 and GOST R 34.11-2012, lists Linux among operating systems, and lists Hyper-V, VMware, VirtualBox, and RHEV as virtual environments. CryptoPro Tools on that page creates and checks a PKCS#7 signature. The page also lists SHA-1 and SHA-2. It does not state bit length 256 or 512, detached CAdES-BES, an exact build, or container support. A footnote mentions build 5.0.13003 only for particular token models.
```

PKCS#7 on that page is not detached CAdES-BES. SHA-1 support on the same page is a reason the BINTRANS allowlist must reject algorithms the engine is able to process.

```text
SOURCE=https://cryptopro.ru/products/cades/sdk
SOURCE_TYPE=VENDOR_DOCUMENT
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=The vendor states that КриптоПро ЭЦП SDK is certified by the FSB as part of SKZI КриптоПро CSP version 5.0 R3. The same page says the API is for creating and checking CAdES messages (ETSI TS 101 733) and that the interface currently supports creating CAdES-BES, CAdES-T, and CAdES-X Long Type 1. Linux (LSB 3.1 and newer) is listed. The page does not give an SDK version, a certificate number, a formulary, or a statement that detached CAdES-BES verification is inside the certified boundary.
```

```text
SOURCE=https://cryptopro.ru/certificates/cryptopro-csp-50-ks1-ispolnenie-1-base-1
SOURCE_TYPE=VENDOR_CERTIFICATE_INDEX
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=Vendor index entry: FSB certificate SF/114-4864, issued 01 May 2024, valid until 01 May 2027, formulary ЖТЯИ.00101-01 30 01 with change notice ЖТЯИ.00101-01.1-2024, for КриптоПро CSP 5.0 KS1 execution 1-Base. The public card names the object as that CSP. It does not quote order 796 or a CAdES profile.
```

```text
SOURCE=https://cryptopro.ru/en/certificates/cryptopro-csp-50-r3-ks2-ispolnenie-2-base-2
SOURCE_TYPE=VENDOR_CERTIFICATE_INDEX
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=Vendor index entry: FSB certificate SF/124-5311, issued 25 November 2025, valid until 25 November 2028, formulary ЖТЯИ.00102-03 30 01 with change notices ЖТЯИ.00102-03.1-2024 and ЖТЯИ.00102-03.2-2024, for КриптоПро CSP 5.0 R3 KS2 execution 2-Base.
```

The Russian certificate index at `https://cryptopro.ru/certificates?pid=1417`, checked via search snippet the same day, uses the same SF/124-5311 record and names the object СКЗИ «КриптоПро CSP» версия 5.0 R3 КС2. A sibling index entry for KS3 is SF/124-5312, 25.11.2025 through 25.11.2028, formulary ЖТЯИ.00103-03 30 01. This pass did not open the certificate PDF for any of these numbers. Rosatom's public certificate list titles the CSP 5.0 files as certificates for СКЗИ, which is a third-party mirror and is not used as the conformity scope.

`CONFORMITY_CONFIRMATION=UNVERIFIED` because the vendor SDK sentence and the SKZI certificate cards do not show that the exact means — detached CAdES-BES verification — is what the certificate covers.

OCSP and TSP appear in vendor SDK usage text as КриптоПро OCSP Client and КриптоПро TSP Client, separate from the CSP page. That is why `OCSP_SUPPORT` and `TIMESTAMP_VERIFY` are `SEPARATE_PRODUCT_NAMED`, not `YES`.

### B. Remote verification service — КриптоПро SVS

```text
PRODUCT=КриптоПро SVS
VENDOR=ООО КРИПТО-ПРО
EXACT_VERSION_OR_BUILD=2.0 guide header cites 2.0.2714; not a qualified pin
LINUX_SUPPORT=NO in the opened guide
SERVER_SUPPORT=Windows Server only in the opened guide
CONTAINER_SUPPORT=UNKNOWN
CAdES_BES_VERIFY=VENDOR_DOCUMENTED_FOR_REVOCATION_BEHAVIOR
DETACHED_CADES_VERIFY=UNKNOWN
GOST_34_10_2012=UNKNOWN
GOST_34_11_2012=UNKNOWN
GOST_256=UNKNOWN
GOST_512=UNKNOWN
CERT_CHAIN_VERIFY=VENDOR_DOCUMENTED
CRL_SUPPORT=YES in the opened guide
OCSP_SUPPORT=YES in the opened online mode
TIMESTAMP_VERIFY=UNKNOWN
CONFORMITY_CONFIRMATION=UNVERIFIED
CONFORMITY_DOCUMENT_REFERENCE=NONE_OPENED
CONFORMITY_SCOPE=UNKNOWN
CONFORMITY_VALIDITY_PERIOD=UNKNOWN
LICENSE_REQUIRED=YES
LICENSE_MODEL=UNKNOWN
PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO
OFFLINE_VERIFY=CRL_CACHE_MODE_DOCUMENTED
NETWORK_DEPENDENCIES=CRL_OR_OCSP_WHEN_ONLINE
```

```text
SOURCE=https://www.cryptopro.ru/sites/default/files/products/svs/2/2828/cryptopro_svs_admin_guide.pdf
SOURCE_TYPE=VENDOR_DOCUMENT
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=The opened administrator guide is for КриптоПро SVS version 2.0.2714, formulary series ЖТЯИ.00094-01. It requires Windows Server 2008 R2 SP1, 2012, 2012 R2, or 2016, and names SKZI КриптоПро CSP version 4.0 executions 2-Base and 3-Base as the crypto provider. It documents a license step, local trust stores, and CRL revocation modes. It does not document Linux or CSP 5.0 R3.
```

```text
SOURCE=https://dss.cryptopro.ru/docs/svs/adminguide/set/certstatus.html
SOURCE_TYPE=VENDOR_DOCUMENT
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=Current SVS documentation describes offline CRL and online OCSP-or-CRL status checks, including a network CRL fetch for CAdES-BES when a CDP extension is present and no local CRL is installed. CAdES-X Long Type 1 does not contact OCSP or CRL during status check.
```

```text
SOURCE=https://dss.cryptopro.ru/docs/svs/devguide/endpoints.html
SOURCE_TYPE=VENDOR_DOCUMENT
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=SVS exposes REST endpoints that accept a signed document and return a verification result. Calling a remote SVS sends signature bytes to that service.
```

No FSB certificate number for SVS was opened. The guide's CSP generation is 4.0, not the 5.0 R3 build named on the SDK page. SVS is not the selected engine.

### C. EDO / operator verification service

```text
PRODUCT=UNKNOWN
VENDOR=UNKNOWN
EXACT_VERSION_OR_BUILD=UNKNOWN
LINUX_SUPPORT=UNKNOWN
SERVER_SUPPORT=UNKNOWN
CONTAINER_SUPPORT=UNKNOWN
CAdES_BES_VERIFY=UNKNOWN
DETACHED_CADES_VERIFY=UNKNOWN
GOST_34_10_2012=UNKNOWN
GOST_34_11_2012=UNKNOWN
GOST_256=UNKNOWN
GOST_512=UNKNOWN
CERT_CHAIN_VERIFY=UNKNOWN
CRL_SUPPORT=UNKNOWN
OCSP_SUPPORT=UNKNOWN
TIMESTAMP_VERIFY=UNKNOWN
CONFORMITY_CONFIRMATION=UNVERIFIED
CONFORMITY_DOCUMENT_REFERENCE=NONE_OPENED
CONFORMITY_SCOPE=UNKNOWN
CONFORMITY_VALIDITY_PERIOD=UNKNOWN
LICENSE_REQUIRED=UNKNOWN
LICENSE_MODEL=UNKNOWN
PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO
OFFLINE_VERIFY=UNKNOWN
NETWORK_DEPENDENCIES=UNKNOWN
```

ADR-EDO-008 and ADR-EDO-010 keep operator exchange on a separate port. This pass did not open an operator conformity certificate for a verification API. An operator result is that operator's claim. It is not BINTRANS operating a conformity-confirmed means. This candidate is not the attachment verifier.

### D. Hybrid adapter

The integration shape is an adapter in front of one engine. A hybrid that parses CMS in Go and asks a CSP only for arithmetic would duplicate cryptographic interpretation. That split is rejected. The adapter may enforce policy on identifiers the engine reports. It does not re-verify the signature math.

### E. Alternate local engine — ViPNet CSP 4.4.8

Recorded so CryptoPro is not the default by familiarity. Not selected.

```text
PRODUCT=ViPNet CSP 4.4.8
VENDOR=АО ИнфоТеКС
EXACT_VERSION_OR_BUILD=4.4.8 named in the press release; Linux build UNKNOWN
LINUX_SUPPORT=UNKNOWN_FOR_THE_NAMED_CERTIFICATE
SERVER_SUPPORT=UNKNOWN
CONTAINER_SUPPORT=UNKNOWN
CAdES_BES_VERIFY=UNKNOWN
DETACHED_CADES_VERIFY=UNKNOWN
GOST_34_10_2012=UNKNOWN
GOST_34_11_2012=UNKNOWN
GOST_256=UNKNOWN
GOST_512=UNKNOWN
CERT_CHAIN_VERIFY=UNKNOWN
CRL_SUPPORT=UNKNOWN
OCSP_SUPPORT=UNKNOWN
TIMESTAMP_VERIFY=UNKNOWN
CONFORMITY_CONFIRMATION=UNVERIFIED
CONFORMITY_DOCUMENT_REFERENCE=vendor press release cites SF/124-4702; certificate PDF not opened
CONFORMITY_SCOPE=VENDOR_PRESS_CLAIMS_SKZI_AND_ORDER_796
CONFORMITY_VALIDITY_PERIOD=vendor press: through 28 December 2026
LICENSE_REQUIRED=UNKNOWN
LICENSE_MODEL=UNKNOWN
PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO
OFFLINE_VERIFY=UNKNOWN
NETWORK_DEPENDENCIES=UNKNOWN
```

```text
SOURCE=https://infotecs.ru/press-center/publications/poluchen-sertifikat-sootvetstviya-fsb-rossii-na-vipnet-csp-4/
SOURCE_TYPE=VENDOR_DOCUMENT
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=InfoTeCS states that FSB certificate SF/124-4702 of 28.12.2023 says ViPNet CSP 4.4.8 meets SKZI requirements for KS1, KS2, and KS3 and the signature-means requirements of FSB order 796 for those classes, valid until 28.12.2026. The same page says the certified 4.4.8 build linked there is for Windows. The page does not describe detached CAdES-BES, GOST bit lengths, CRL, OCSP, or a Linux execution of that certificate.
```

The press release is stronger order-796 language than the CryptoPro certificate cards. It is still not the certificate PDF, and it does not prove the verification function this policy needs. Reseller pages that mention CAdES-BES were not used.

## 6. Selection

```text
ENGINE_SELECTION=BLOCKED
PRIMARY_ENGINE=
VENDOR=
EXACT_VERSION_OR_BUILD=
WHY_SELECTED=
WHY_NOT_ALTERNATIVES=No candidate has a conformity document, read in this pass, whose scope is detached CAdES-BES verification on a pinned Linux server build with GOST R 34.10-2012 and GOST R 34.11-2012 at both 256 and 512 bits.
READY_FOR_I4C_B_IMPLEMENTATION=NO
```

CryptoPro was not selected because it is widely deployed. ViPNet was not selected because a press release names order 796. SVS was not selected because it is a verification product. Each one fails the evidence bar in section 5.

### Evidence still required before any selection

1. The FSB certificate PDF, or an official FSB registry extract, for one exact build, showing that the certified object includes signature verification under the signature-means requirements, not only an SKZI name on a vendor card.
2. The formulary or certified manual for that same build stating detached CAdES-BES verification, GOST R 34.10-2012 at 256 and 512 bits, GOST R 34.11-2012, certificate chain building, and CRL or OCSP.
3. The package or image version string operations can pin, and the certificate number that covers that string.
4. Whether that Linux server execution is the certified execution, and whether a container is inside the certified environment or needs a separate influence evaluation.
5. The license model for a server-side verifier, including whether a license secret is required, and written confirmation that verification does not take the signer's private key or require BINTRANS to hold a signing key.
6. For SVS, a current certificate, a Linux build, and a CSP generation that matches the certified build. The opened 2.0.2714 guide is Windows Server and CSP 4.0.
7. For ViPNet CSP 4.4.8, the certificate PDF, the Linux execution covered by it, and detached CAdES-BES verification in that execution's manual.

Until those exist, I4C-B does not implement a verifier and does not open the evidence gate to `VALID` or `INVALID`.

## 7. Topology

The boundary is frozen even though the engine is not:

```text
document-service
        |
SignatureVerifier port
        |
qualified verifier adapter
        |
selected engine
```

Vendor types, command lines, and certificate-store handles stay inside the adapter. Domain code keeps calling `VerifySignature` and maps the port result. `UnavailableSignatureVerifier` remains the default until a qualified adapter is deployed.

```text
VERIFIER_TOPOLOGY=LOCAL_DAEMON
PROCESS_ISOLATION=YES
IPC_PROTOCOL=UNKNOWN
HEALTHCHECK=UNKNOWN
LICENSE_LOCATION=UNKNOWN
PACKAGE_UPDATE_MODEL=UNKNOWN
SIGNATURE_BYTES_LEAVE_BINTRANS=NO
```

`LOCAL_DAEMON` is the proposed shape: a separate process on the same host or pod network namespace, so a CSP or SVS-like library is not linked into the Go service, and signature bytes are not sent to an external operator. IPC, health, license path, and package update stay `UNKNOWN` until an engine is pinned. Inventing gRPC, a socket path, or a container image would pretend a product was selected.

`IN_PROCESS` is not the proposal. It puts the vendor API in the service process. `REMOTE_SERVICE` is not the proposal. The only remote product opened here is Windows-only, has no opened conformity certificate, and would send signature bytes outside the service. An operator service remains the exchange port, not this daemon.

When a daemon is eventually specified, these rules already apply:

```text
TLS=loopback or pod-local only; no public listener
AUTH=mutual authentication between document-service and the daemon
REQUEST_INTEGRITY=the adapter sends the stored attachment digest and the blob digest with the bytes
RESPONSE_AUTHENTICITY=the daemon response is authenticated; an unsigned local response is not a result
TIMEOUT=LOCAL_VERIFY_TIMEOUT
DATA_RESIDENCY=signature bytes remain on the BINTRANS host
```

## 8. Private keys

```text
PRIVATE_KEY_REQUIRED_FOR_VERIFY=NO
PRIVATE_KEYS_STORED_BY_BINTRANS=NO
```

The adapter refuses an engine configuration that asks for a signer private key, a server signing key, or a key container used to verify. Verification uses the public key in the signer certificate and the trust anchors provisioned for the policy. I4B's private-key banner rejection stays.

## 9. First format

Unchanged from I4A and I4B.

```text
FIRST_FORMAT=detached CAdES-BES
GOST_256=YES
GOST_512=YES
GOST_2001_ALLOWED=NO
```

The qualified policy allowlist is GOST R 34.10-2012 and GOST R 34.11-2012, both 256-bit and 512-bit parameter sets. The first policy rejects GOST R 34.10-2001, MD5, SHA-1, XMLDSig, an embedded PDF signature, and a bare PKCS#7 profile that lacks the CAdES-BES signed attributes. Scope does not grow in I4C-B.

`GOST_256=YES` and `GOST_512=YES` are policy requirements. They are not a claim that a candidate engine implements both. Candidate cells in section 5 stay `UNKNOWN` where the vendor page did not say the bit length.

## 10. Responsibility

One authority per capability. The adapter does not repeat the engine's cryptographic decision.

| Capability | Authority |
|---|---|
| CMS parsing | ENGINE |
| CAdES-BES profile check | ENGINE |
| Signed attributes | ENGINE |
| message-digest | ENGINE, over the bytes BINTRANS supplies |
| signing-certificate-v2 | ENGINE |
| Signature math | ENGINE |
| Algorithm allowlist | BINTRANS_ADAPTER, using algorithm identifiers the engine reports |
| Certificate extraction | ENGINE |
| Chain building | ENGINE |
| Certificate validity | ENGINE |
| Revocation | ENGINE |
| Timestamp validation | ENGINE |
| Trust-anchor validation | ENGINE, against the provisioned set |

BINTRANS keeps input size limits, tenant isolation, stored-object integrity, policy selection, deadlines, result mapping, and audit persistence. Stored-object integrity means the bytes verified are the bytes at the server object key, checked by the SHA-256 already stored in `attachment_signature_blobs`. The adapter does not hash a second interpretation of the CMS content when the profile is detached: the content is the attachment object.

If the selected engine cannot report algorithm identifiers, chain status, revocation status, and timestamp status as data, it cannot fill this matrix. That gap is another selection blocker. It is not a reason to parse CMS in Go.

## 11. Trust anchors

```text
APPLICATION_REPO_CONTAINS_ROOTS=NO
TRUST_ANCHOR_SOURCE=head certification-center publication channel https://e-trust.gosuslugi.ru
TRUST_ANCHOR_PROVISIONING=out of band, onto the verifier host trust store, by operations
TRUST_ANCHOR_UPDATE_PROCESS=replace the store from a new official publication, record the digest, restart or reload the daemon, append new evidence under the same policy only for new verifications
TRUST_ANCHOR_DIGEST_RECORDING=SHA-256 of the provisioned bundle in the operations inventory, not in git
TRUST_ANCHOR_ROLLBACK=restore the previous bundle whose digest is in the inventory
```

```text
SOURCE=https://e-trust.gosuslugi.ru
SOURCE_TYPE=BINTRANS_POLICY
CHECKED_AT=2026-10-07
SUPPORTED_CLAIM=I4A identified this portal as the head certification-center channel cited for accredited-center material. This pass did not download roots, TSL files, or certificates. Secondary copies and open-data mirrors are not the provisioning source.
```

The application repository does not gain a `certs/` tree. A copied root without a recorded source and digest is not a trust anchor.

## 12. Revocation

```text
REVOCATION_PRIMARY=CRL
REVOCATION_FALLBACK=OCSP
CACHE=YES
CACHE_MAX_AGE=the CRL nextUpdate, and never longer than 12 hours
NETWORK_TIMEOUT=10s
STALE_CACHE_BEHAVIOR=PENDING
INFRA_FAILURE_TO_INVALID=NO
```

CRL is primary because article 13 requires a certification center to expose revocation information and I4A classifies CRL and OCSP as the technical way to read it. OCSP is the fallback when the engine supports it and the certificate carries an OCSP responder. A vendor trust service is not a third authority unless it is inside the later-selected certified means.

Mappings:

| Condition | Result |
|---|---|
| CRL unavailable | PENDING / `CRL_UNAVAILABLE` |
| OCSP unavailable | PENDING / `OCSP_UNAVAILABLE` |
| Network timeout | PENDING / `NETWORK_TIMEOUT` |
| Confirmed revocation at the policy time base | INVALID / `CERT_REVOKED` |

A stale cache is unavailable evidence. It is not a successful "not revoked" answer. `CACHE_MAX_AGE` also respects the twelve-hour registry update duty noted from 63-FZ article 13 in I4A. This pass did not re-quote that amendment.

## 13. Network and SSRF

Certificate CDP and OCSP URLs may be contacted only after the engine or adapter has parsed them from the certificate, and only when the address is public. Deny loopback, RFC 1918 and other private ranges, link-local, metadata addresses, and local service networks. Redirects are denied. A caller-supplied URL is never fetched.

```text
CALLER_URL_FETCHED=NO
PRIVATE_NETWORK_FETCH=NO
REDIRECT_POLICY=DENY
```

The I4B opaque reference `bintrans:attachment-signature:<uuid>` is resolved inside the tenant object store. It is not an HTTP URL.

## 14. Signing time

| Source | Trusted as signing time |
|---|---|
| `CALLER_SIGNING_TIME` | NO |
| `CMS_SIGNED_ATTRIBUTE` | NO by itself; the signer asserted it |
| `TRUSTED_TIMESTAMP` | YES when the engine validates the timestamp token |
| `VERIFICATION_TIME` | YES as the article 11 fallback when no reliable signing time exists |

```text
TRUSTED_SIGNING_TIME=NO
```

for the caller field alone.

The selected engine's timestamp behavior is `UNKNOWN` (section 5). The qualified policy does not require a trusted timestamp on every signature. That matches article 11 as recorded in I4A: use reliable signing-time information when it exists, otherwise the verification date. A timestamp token that the engine proves invalid is `TIMESTAMP_INVALID`. A missing token is not `TIMESTAMP_INVALID`. A token the engine cannot fetch stays `PENDING`.

`CERT_EXPIRED` uses that policy time base. A certificate that expires after a proven signing time is not expired for that historical check.

## 15. Result model

Target vocabulary for a future I4C-B, after the evidence gate is widened by a later migration. I4C-A does not enable it.

| Engine or policy outcome | Status |
|---|---|
| `BAD_SIGNATURE` | INVALID |
| `CONTENT_HASH_MISMATCH` | INVALID |
| `CERT_EXPIRED` | INVALID at the policy time base |
| `CERT_NOT_YET_VALID` | INVALID |
| `CERT_REVOKED` | INVALID |
| `UNKNOWN_CA` | INVALID |
| `CHAIN_BUILD_FAILED` | INVALID only when the chain failure is conclusive |
| `UNSUPPORTED_ALGORITHM` | INVALID |
| `UNSUPPORTED_FORMAT` | INVALID |
| `TIMESTAMP_INVALID` | INVALID when the token is conclusively invalid |
| `VERIFIER_UNAVAILABLE` | PENDING |
| `CRL_UNAVAILABLE` | PENDING |
| `OCSP_UNAVAILABLE` | PENDING |
| `NETWORK_TIMEOUT` | PENDING |
| `ENGINE_TIMEOUT` | PENDING |
| `UNKNOWN_ENGINE_ERROR` | PENDING unless the engine proves a cryptographic failure |

```text
VALID_SUPPORTED_BY_SELECTED_ENGINE=NO
INVALID_SUPPORTED=NO
PENDING_SUPPORTED=YES
INFRA_FAILURE_TO_INVALID=NO
```

`PENDING_SUPPORTED=YES` is the I4B behavior of `UnavailableSignatureVerifier`, not a production engine. `VALID` and `INVALID` stay unsupported until an engine is qualified and a later migration allows the server to write them.

A certificate-count or chain-length limit trip is `UNSUPPORTED_FORMAT`, not `CHAIN_BUILD_FAILED`. An oversize signature never reaches the engine: I4B already rejects it at upload.

## 16. Evidence schema

`000096` already has `certificate_subject`, `certificate_issuer`, `certificate_serial`, `certificate_thumbprint`, `certificate_valid_from`, `certificate_valid_to`, `chain_fingerprint`, `chain_status`, `revocation_status`, `revocation_evidence_time`, `timestamp_status`, `timestamp_token_object_key`, `verifier_version`, `policy_id`, and `policy_version`.

No engine is selected, so those columns cannot be matched to a real engine payload. The policy in this note also needs facts the table cannot store:

- observed signature algorithm and hash algorithm, so the allowlist is auditable;
- signing-time source and the policy time base;
- revocation source (`CRL` or `OCSP`);
- engine product and build, because `verifier_version VARCHAR(64)` cannot hold a formulary, certificate number, and package version together;
- trust-anchor bundle digest used for that attempt.

```text
EVIDENCE_SCHEMA_SUFFICIENT=NO
I4C_B_MIGRATION_REQUIRED=YES
```

A future migration, not this phase, would add `signature_algorithm`, `hash_algorithm`, `signing_time_source`, `policy_time`, `revocation_source`, `engine_product`, `engine_build`, and `trust_anchor_digest`, and would replace the I4B status and reason checks. This phase does not create `000097` or edit `000096`.

## 17. Future DB gate

I4C-B may, in a migration after the engine is qualified, allow the server writer to insert:

- `verification_status` of `VALID`, `INVALID`, or `PENDING`;
- the reason codes in section 15.

Requirements that migration must keep:

- the client cannot write evidence;
- `attachment_signatures` stays immutable and `UNVERIFIED`;
- the legacy signing-session acceptance row stays `UNVERIFIED` on the attachment table;
- evidence stays append-only;
- effective status stays the latest row for one policy id and policy version;
- a pre-commit failure may delete an uncommitted object; an unknown commit outcome must not delete it.

No SQL changes in I4C-A. Until that migration, the database still rejects every status except `PENDING` and every reason except `VERIFIER_UNAVAILABLE`.

## 18. Retry

```text
PENDING=retryable
VALID=no automatic recompute under the same policy
INVALID=no automatic recompute under the same policy
EXPLICIT_REVERIFY=appends a new row
NEW_POLICY_VERSION=new evidence row
HISTORICAL_EVIDENCE=never overwrite
```

Retry uses the same stored attachment bytes, signature bytes, and policy version. A later revocation does not rewrite an earlier row. It may append a new row.

## 19. Bounds

No selected engine, so the I4A bounds stay.

```text
SIGNATURE_MAX_BYTES=1048576
ATTACHMENT_MAX_BYTES=10MiB
LOCAL_VERIFY_TIMEOUT=5s
REVOCATION_NETWORK_TIMEOUT=10s
MAX_CERTIFICATES=16
MAX_CHAIN_LENGTH=6
```

I4B already enforces the signature size in `000096` and in the binary upload. Changing a bound needs engine evidence. There is none.

## 20. Observability

Metrics for the future adapter:

- `verification_attempts_total`
- `verification_valid_total`
- `verification_invalid_total{reason}`
- `verification_pending_total{reason}`
- `verification_duration_seconds`
- `revocation_duration_seconds`
- `verifier_unavailable_total`
- `verification_timeout_total`

Logs may include `signature_id`, `policy_id`, `policy_version`, `reason_code`, `certificate_thumbprint`, `engine_version`, and duration. Logs must not include signature bytes, attachment bytes, private keys, full certificate dumps, credentials, or authorization headers.

## 21. Operations prerequisites

Not deployable. Values that depend on the missing engine stay unknown.

```text
PACKAGE_OR_IMAGE=UNKNOWN
VERSION_PIN=UNKNOWN
LICENSE_REQUIRED=YES
LICENSE_MODEL=UNKNOWN
LICENSE_SECRET_TYPE=UNKNOWN
TRUST_STORE=host store provisioned from the head certification-center channel
OUTBOUND_NETWORK_REQUIRED=YES
OUTBOUND_DESTINATIONS=public CRL and OCSP endpoints named by the signer certificate, after SSRF checks
DNS_REQUIRED=YES
CPU_REQUIREMENT=UNKNOWN
MEMORY_REQUIREMENT=UNKNOWN
HEALTHCHECK=UNKNOWN
READINESS_CHECK=daemon responds and the pinned build's license and trust-store digest match the inventory
UPGRADE_PROCEDURE=pin the new certified build, record its certificate number and package digest, run the staging vectors, then switch the daemon
ROLLBACK_PROCEDURE=restore the previous pinned build and the previous trust-anchor bundle digest
```

`OUTBOUND_NETWORK_REQUIRED=YES` is the revocation policy, not a claim that the blocked engine has been measured. Do not install a package, pull an image, or apply a license under this phase.

## 22. Staging vectors for a later phase

Design only. No staging change in I4C-A.

| Vector | Expected | Material |
|---|---|---|
| `VALID_GOST_256` | INVALID until an engine is qualified; then VALID | Test-only key, labelled not conformity evidence |
| `VALID_GOST_512` | same | Test-only key, labelled not conformity evidence |
| `BAD_SIGNATURE` | INVALID | Tampered test signature |
| `MODIFIED_DOCUMENT` | INVALID / `CONTENT_HASH_MISMATCH` | Stored attachment bytes changed after signing, in a test fixture |
| `EXPIRED_CERT` | INVALID / `CERT_EXPIRED` at the policy time base | Test certificate |
| `NOT_YET_VALID_CERT` | INVALID / `CERT_NOT_YET_VALID` | Test certificate |
| `REVOKED_CERT` | INVALID / `CERT_REVOKED` | Test CA fixture with a CRL this harness serves |
| `UNKNOWN_CA` | INVALID / `UNKNOWN_CA` | Test chain outside the provisioned anchors |
| `UNSUPPORTED_GOST_2001` | INVALID / `UNSUPPORTED_ALGORITHM` | Test fixture using the rejected algorithm |
| `MALFORMED_CMS` | INVALID / `UNSUPPORTED_FORMAT` | Truncated or indefinite ASN.1 fixture |
| `OVERSIZE` | upload rejection, no evidence row | Bytes above 1 MiB |
| `REVOCATION_NETWORK_DOWN` | PENDING | Fixture whose CRL host is blackholed |
| `VERIFIER_DOWN` | PENDING / `VERIFIER_UNAVAILABLE` | Daemon stopped |
| `TIMESTAMP_VALID` | depends on engine support, currently UNKNOWN | Not a conformance claim |
| `TIMESTAMP_INVALID` | INVALID only when a token is present and conclusively invalid | Test token fixture |

```text
TEST_VECTOR_PLAN=DESIGN_ONLY
REAL_PRIVATE_KEYS_IN_REPO=NO
STAGING_MUTATED=NO
```

Test-only keys, if a later phase generates them, are for mechanical tests. They are not evidence that the means has a conformity confirmation. They are not committed as production key material. This phase generates none.

## 23. What I4C-B is not

I4C-B does not start from this note as an implementation ticket. It starts only after the missing evidence in section 6 is attached to a revised selection and this block is lifted. Until then the production path remains `UnavailableSignatureVerifier`, and the database continues to refuse `VALID` and `INVALID`.
