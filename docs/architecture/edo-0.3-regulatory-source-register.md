# EDO 0.3 Regulatory Source Register

## Status

```text
DOCUMENT_STATUS=DISCOVERY
LEGAL_VERIFICATION_STATUS=OPEN
COMPLIANCE_CLAIM=NONE
CHECKED_ON=2026-09-23
IMPLEMENTATION_AUTHORIZED=NO
```

This register cites primary official publications so later waves can be reviewed. It does not reproduce statutory text. It does not mark any EDO 0.2 registry row as `VERIFIED`. BINTRANS is not described as a licensed or accredited operator.

Items that an engineer cannot close are marked `LEGAL_VERIFICATION_REQUIRED`.

## How to read a row

| Field | Meaning |
|-------|---------|
| Document | Official title |
| Number and date | Act identifier |
| Edition checked | What was opened on 2026-09-23, not a lawyer's consolidation |
| URL | Official host: pravo.gov.ru, publication.pravo.gov.ru, nalog.gov.ru, or mintrans.gov.ru |
| Architecture effect | Constraint on EDO 0.3 design |
| Legal review | What remains outside engineering discovery |

## Sources

### LR-03-SIG-001 — electronic signature

| Field | Value |
|-------|--------|
| Document | Federal Law "On electronic signature" |
| Number and date | 06.04.2011 No. 63-FZ |
| Edition checked | IPS card lists amending acts through 31.07.2025 No. 304-FZ. A later amending act, 04.08.2026 No. 315-FZ, is published separately. This discovery did not merge those texts into one edition. |
| URL | [IPS card](http://pravo.gov.ru/proxy/ips/?docbody=&nd=102146610); [315-FZ publication](http://publication.pravo.gov.ru/document/0001202608040021) |
| Architecture effect | Signature evidence must be able to record signature class. Variant A stores metadata only. Private keys stay out of `document-service`. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for the edition in force on a production date, for which document types require a qualified signature, and for the effect of 315-FZ. |

### LR-03-ACC-001 — accounting records

| Field | Value |
|-------|--------|
| Document | Federal Law "On accounting" |
| Number and date | 06.12.2011 No. 402-FZ |
| Edition checked | Official publication of the original act (published 07.12.2011). The consolidated edition, including retention article text as of 2026-09-23, was not opened as a single current text. |
| URL | [publication.pravo.gov.ru](http://publication.pravo.gov.ru/document/0001201112070017) |
| Architecture effect | Accounting primary documents are a retention concern for archive design. EDO 0.3 variant A does not implement retention enforcement. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for the current retention period, electronic-document rules, and which BINTRANS document types are accounting primary documents. |

### LR-03-PD-001 — personal data

| Field | Value |
|-------|--------|
| Document | Federal Law "On personal data" |
| Number and date | 27.07.2006 No. 152-FZ |
| Edition checked | IPS card title "О персональных данных" opened. Amending publications through 28.02.2025 No. 23-FZ were located. Consolidated text was not reviewed line by line. |
| URL | [IPS card](http://pravo.gov.ru/proxy/ips/?docbody=&nd=102108261); [23-FZ](http://publication.pravo.gov.ru/document/0001202502280034); [519-FZ](http://publication.pravo.gov.ru/document/0001202412280021) |
| Architecture effect | Document payload and files may contain personal data. Later logs and errors must not dump payloads. Storage residency is an INFRA question, not an EDO 0.3 schema claim. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for legal basis, retention, and transfer limits. |

### LR-03-ETRN-001 — exchange of electronic transport documents

| Field | Value |
|-------|--------|
| Document | Rules for exchange of electronic transport documents and their data, including submission to the state system |
| Number and date | Government Decree 21.05.2022 No. 931 |
| Edition checked | Official publication record opened (published 25.05.2022). Later amendments were not consolidated in this note. |
| URL | [publication.pravo.gov.ru](http://publication.pravo.gov.ru/Document/View/0001202205250011) |
| Architecture effect | Operator exchange and GIS EPD belong to TEDO, not to recommended EDO 0.3. `GIS_EPD_CONNECTED` stays `NO`. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` whether BINTRANS must be, or must use, an operator of an electronic transport-document information system. No accreditation is claimed. |

### LR-03-ETRN-002 — road electronic waybill exchange rules

| Field | Value |
|-------|--------|
| Document | Rules for exchange of electronic carriage contracts, electronic waybills, electronic road lists, and electronic cargo-acceptance receipts, and for sending them to the state system |
| Number and date | Government Decree 01.06.2024 No. 753 |
| Edition checked | Official publication record opened (published 01.06.2024). |
| URL | [publication.pravo.gov.ru](http://publication.pravo.gov.ru/document/0001202406010006) |
| Architecture effect | Confirms e-waybill lifecycle is a transport-document problem. TEDO-0.3 remains the roadmap phase. EDO 0.3 does not model titles. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for the current edition and for which parties must sign which title. |

### LR-03-GIS-001 — GIS EPD technical requirements

| Field | Value |
|-------|--------|
| Document | Rules for submitting information to the state electronic transport-document system and technical requirements for those information systems |
| Number and date | Government Decree 03.03.2022 No. 281 |
| Edition checked | Official publication record. Mintrans pages also point at a 28.12.2024 amendment of related technical rules. This note does not merge the texts. |
| URL | [281 publication](http://publication.pravo.gov.ru/Document/View/0001202203050020); [Mintrans GIS EPD page](https://mintrans.gov.ru/eye/activities/94/7/365) |
| Architecture effect | Adapter and accreditation work stay outside EDO 0.3. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for operator inclusion rules and the current technical edition. |

### LR-03-ETRN-FMT-001 — electronic waybill format

| Field | Value |
|-------|--------|
| Document | FNS order on formats of the electronic waybill, electronic accompanying list, and electronic work order |
| Number and date | 09.12.2021 No. ЕД-7-26/1065@ |
| Edition checked | Cited on the official Mintrans GIS EPD page and in the Mintrans interaction regulation PDF. The order text and any 2026 amendments were not opened as a consolidated format. |
| URL | [Mintrans GIS EPD page](https://mintrans.gov.ru/eye/activities/94/7/365) |
| Architecture effect | `ETRN` as a `document_type` label is not a format implementation. Format binding belongs with FormatVersion and TEDO, both outside recommended EDO 0.3. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for the XSD edition in force, including any later FNS orders listed by Mintrans. |

### LR-03-UPD-001 — invoice and UPD format

| Field | Value |
|-------|--------|
| Document | FNS order on electronic invoice and universal transfer document formats |
| Number and date | 19.12.2023 No. ЕД-7-26/970@ |
| Edition checked | FNS news and the FNS page for amending order 15.11.2024 No. ЕД-7-26/1032@. FNS news states that UPD format 5.03 of order 970@ is the format to use after formal TORG-12 and act formats end, and that order 20.01.2025 No. ЕД-7-26/28@ takes effect 01.01.2026. |
| URL | [FNS news on 970@](https://www.nalog.gov.ru/rn77/news/activities_fts/14422827/); [order 1032@](https://www.nalog.gov.ru/rn77/about_fts/docs/15556529/); [order 28@](https://www.nalog.gov.ru/rn77/about_fts/docs/16587608/); [FNS news on format end](https://www.nalog.gov.ru/rn77/news/activities_fts/16543959/) |
| Architecture effect | UPD XML and format versions are not in recommended EDO 0.3. Billing remains the commercial projection (ADR-EDO-002). |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` before any wave stores or validates UPD XML. Correction (UKD) format was not opened and stays unverified. |

### LR-03-MCHD-001 — machine-readable power of attorney

| Field | Value |
|-------|--------|
| Document | FNS orders and FNS notices on electronic powers of attorney; FNS notice that a Ministry of Digital Development unified format is accepted for a stated use |
| Number and date | FNS order 16.10.2024 No. ЕД-7-26/858@ (format version 5.03, accepted by the tax authority from 10.01.2025 per FNS news). Unified format version 003 is described by FNS as a Ministry of Digital Development format accepted by tax authorities from 01.03.2025. |
| Edition checked | FNS news pages. The Ministry of Digital Development schema file itself was not retrieved from digital.gov.ru during this discovery. |
| URL | [FNS news 5.03](https://www.nalog.gov.ru/rn77/news/activities_fts/15582370/); [FNS news on unified format](https://www.nalog.gov.ru/rn03/news/activities_fts/15998874/) |
| Architecture effect | Variant A does not store or check MChD. A GUID column without a verification rule is rejected as the default. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` for which format applies to counterparty EDI versus tax filing, and for the official schema URL. |

### LR-03-OP-001 — operator status

| Field | Value |
|-------|--------|
| Document | No source consulted here names this platform as an operator |
| Number and date | Not applicable |
| Edition checked | 2026-09-23 |
| URL | Not applicable |
| Architecture effect | ADR-EDO-008 remains `EXTERNAL_OPERATOR_MODE=YES` and `OWN_IS_EPD_OPERATOR_MODE=NO`. |
| Legal review | `LEGAL_VERIFICATION_REQUIRED` before any accreditation, license, or "we are an operator" statement. |

## Registry disposition

EDO 0.2 placeholder rows in [edo-0.2-legal-requirements-registry.md](edo-0.2-legal-requirements-registry.md) stay `EXTERNAL_LEGAL_VERIFICATION_REQUIRED`. This file does not flip them to `VERIFIED`.

## References

- [edo-0.3-architecture-options.md](edo-0.3-architecture-options.md)
- [edo-0.2-legal-requirements-registry.md](edo-0.2-legal-requirements-registry.md)
- [ADR-EDO-008](../adr/ADR-EDO-008-external-operator-first-own-operator-ready.md)
