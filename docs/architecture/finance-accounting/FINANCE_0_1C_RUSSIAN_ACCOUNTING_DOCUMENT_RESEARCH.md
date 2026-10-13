# FINANCE-0.1C Russian accounting document research

```text
STAGE=FINANCE-0.1C-RUSSIAN-ACCOUNTING-DOCUMENT-RESEARCH
KIND=DOCS_ONLY_RESEARCH
BASE_SHA=e538528712f74c5f487e496bc4c33a2d91a2754f
RESEARCH_DATE=2026-10-13
IMPLEMENTATION_AUTHORIZED=NO
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
SECONDARY_SOURCE_ALLOWED_FOR_FREEZE=NO
CANONICAL_MONEY_TYPE=EXACT_DECIMAL
CANONICAL_API_MONEY_TYPE=DECIMAL_STRING
TAX_RATE_CHOSEN=NO
VAT_EXEMPTION_CHOSEN=NO
TAX_ROUNDING_ALGORITHM_FROZEN=NO
PRODUCT_CODE_CHANGED=NO
```

This stage records what official texts say, what BINTRANS may rely on, and what is still open. It does not implement a document, a tax decision, an XML file, or a signature.

A commercial database, vendor page, blog, or forum was not used to freeze a rule. Where a search result summarized a statute without the statute text, that summary was ignored.

`STATUS` means:

- `VERIFIED_PRIMARY_SOURCE`: the cited official text was read in this stage.
- `NEEDS_FURTHER_RESEARCH`: the question is in scope and the controlling sentence was not read.
- `NOT_APPLICABLE`: the reviewed primary sources do not make this object a statutory accounting or VAT document.
- `DEFERRED`: the requirement is acknowledged, and the artifact belongs to Agent B.

## 1. Normative-source matrix

| ID | Authority | Official act or order | Number | Date | Status reviewed | Official URL | What was read |
| --- | --- | --- | --- | --- | --- | --- | --- |
| SRC-402 | pravo.gov.ru IPS | Federal Law on accounting | 402-FZ | 06.12.2011; article 9 text includes the amendments cited in that article through 26.07.2019 No. 247-FZ | Consolidated article 9 read | http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=2cd1c64de5d2c2f028fbd68e725846eeae60fdd78e4a90d57b1f9e110477ba36 | Article 9 in full |
| SRC-1137 | pravo.gov.ru IPS | Government decree on VAT document forms | 1137 | 26.12.2011; IPS revision line includes amendments through 23.01.2026 No. 26 | Consolidated appendices 1 and 2 read | http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102153432&page=1&rdk=12 | Invoice form, filling rules, correction-invoice form and filling rules |
| SRC-248 | pravo.gov.ru IPS | Federal Law amending the Tax Code | 248-FZ | 23.07.2013 | Amending article read | http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=689a20c8e904e3ca05cb9f9c35c87201577a0d268ea08120ff83baea8d28dee5 | Article 1, item 9, adding Tax Code part one article 52 point 6 |
| SRC-UPD | nalog.gov.ru | FNS letter | MMV-20-3/96@ | 21.10.2013 | Official FNS publication of the letter's recommendation read; the letter appendix was not read line by line | https://www.nalog.gov.ru/rn77/about_fts/about_nalog/4319219/ and https://www.nalog.gov.ru/rn11/news/tax_doc_news/4345136/ | Recommendation, status 1 and status 2 |
| SRC-UKD | nalog.gov.ru | FNS letter | MMV-20-15/86@ | 17.10.2014 | Official FNS publication of the recommendation read; the letter appendix was not read line by line | https://www.nalog.gov.ru/rn62/news/tax_doc_news/4984071/ and https://www.nalog.gov.ru/rn77/news/tax_doc_news/4980752/ | Recommendation and non-mandatory character |
| SRC-970 | publication.pravo.gov.ru | FNS order | ED-7-26/970@ | 19.12.2023; registered 26.01.2024 No. 77001; published 29.01.2024 | Title and publication record read; XML schema not read | http://publication.pravo.gov.ru/document/0001202401290019 | Existence and document classes covered by the title |
| SRC-29 | publication.pravo.gov.ru | FNS order | ED-1-26/29@ | 19.01.2026; registered 17.03.2026 No. 85644; published 19.03.2026 | Title and publication record read; XML schema not read | http://publication.pravo.gov.ru/document/0001202603190004 | Existence of electronic correction-invoice and buyer consent/notification formats |
| SRC-NK2 | pravo.gov.ru IPS | Tax Code part two | 117-FZ | 05.08.2000 | Chapter 21 extract read from article 143 through article 162.2 only | http://pravo.gov.ru/proxy/ips/?docbody=&nd=102067058 | Articles 168 and 169 were not in the retrieved extract |

`PRIMARY_SOURCE_COUNT=8` official acts or official publications were opened. Articles 168 and 169 of the Tax Code were not retrieved as article text, so they are not counted as reviewed provisions.

## 2. Document research

### 2.1 Commercial invoice / счёт на оплату

```text
AUTHORITY=NONE_FOUND
OFFICIAL_ACT_OR_ORDER=NONE
DOCUMENT_NUMBER=
DATE=
CURRENT_EFFECTIVE_STATUS=NOT_FOUND_IN_REVIEWED_ACTS
OFFICIAL_URL=
PROVISION_OR_SECTION=
BINTRANS_INTERPRETATION=A payment invoice is not a statutory primary document under 402-FZ article 9 and is not the VAT invoice regulated by decree 1137. BINTRANS must not treat a browser payment invoice as accounting truth or as a VAT invoice.
STATUS=NOT_APPLICABLE
```

`INVOICE_RULES_RESEARCHED=YES` for this negative finding. Whether a particular carriage contract requires a payment invoice remains a contract fact, not a statutory form.

### 2.2 Act / акт

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Federal Law on accounting
DOCUMENT_NUMBER=402-FZ
DATE=06.12.2011
CURRENT_EFFECTIVE_STATUS=ARTICLE_9_TEXT_READ
OFFICIAL_URL=http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=2cd1c64de5d2c2f028fbd68e725846eeae60fdd78e4a90d57b1f9e110477ba36
PROVISION_OR_SECTION=Article 9, parts 1, 2, 3, 4, 5 and 7
BINTRANS_INTERPRETATION=An act of services or works is a primary accounting document only when it records a real fact and carries the seven statutory requisites. No federal form named "акт" was found. The entity approves the form. The document date is the compilation date, at the time of the fact or immediately after it. A later correction of a primary document must show the correction date and the signatures of the persons who compiled the corrected document, unless another federal law forbids that method. A VAT invoice is such another regime.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`ACT_RULES_RESEARCHED=YES`. The seven requisites are: document name; compilation date; name of the economic entity that compiled it; content of the fact; natural and/or monetary measure with units; position of the responsible person; signatures with surname and initials or other identifying requisites.

### 2.3 VAT invoice / счёт-фактура

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Government decree 1137, adopted under Tax Code article 169
DOCUMENT_NUMBER=1137
DATE=26.12.2011
CURRENT_EFFECTIVE_STATUS=IPS_TEXT_INCLUDES_AMENDMENT_23.01.2026_NO_26
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102153432&page=1&rdk=12
PROVISION_OR_SECTION=Appendix 1, form and section II
BINTRANS_INTERPRETATION=The operative filling instrument for a VAT invoice is decree 1137. The invoice is a line document, not a register header. Seller and buyer name, EGRUL or EGRIP address, and INN/KPP are form fields. For works, services, and property rights the consignor and consignee lines may be dashed. Cost columns are in currency units and subunits, including rubles and kopecks. An unlabeled company tax_id is not this INN field.
STATUS=VERIFIED_PRIMARY_SOURCE
```

The Tax Code article 169 field list itself was not extracted. Decree 1137 states that it is issued under that article. Until article 169 is read, BINTRANS uses the decree as the verified form and does not claim that every decree row was re-derived from the Code.

`VAT_INVOICE_RULES_RESEARCHED=YES` for the decree. `TAX_CODE_ARTICLE_169_TEXT_READ=NO`.

### 2.4 UPD / УПД

```text
AUTHORITY=nalog.gov.ru
OFFICIAL_ACT_OR_ORDER=FNS letter
DOCUMENT_NUMBER=MMV-20-3/96@
DATE=21.10.2013
CURRENT_EFFECTIVE_STATUS=RECOMMENDED_FORM_NOT_A_MANDATORY_BLANK
OFFICIAL_URL=https://www.nalog.gov.ru/rn11/news/tax_doc_news/4345136/
PROVISION_OR_SECTION=FNS publication of the letter: status 1 combines the invoice and the transfer document; status 2 is a primary document only
BINTRANS_INTERPRETATION=UPD is optional. Status 1 must satisfy both the VAT-invoice regime and the 402-FZ requisite list. Status 2 must not be treated as a VAT invoice. The paper appendix was not transcribed in this stage. The electronic format is a separate FNS order and belongs to Agent B.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`UPD_RULES_RESEARCHED=YES` for status and legal character. Field-by-field transcription of the 2013 appendix remains open.

### 2.5 UKD / correction UPD

```text
AUTHORITY=nalog.gov.ru
OFFICIAL_ACT_OR_ORDER=FNS letter
DOCUMENT_NUMBER=MMV-20-15/86@
DATE=17.10.2014
CURRENT_EFFECTIVE_STATUS=RECOMMENDED_FORM_NOT_A_MANDATORY_BLANK
OFFICIAL_URL=https://www.nalog.gov.ru/rn62/news/tax_doc_news/4984071/
PROVISION_OR_SECTION=FNS publication of the letter
BINTRANS_INTERPRETATION=UKD is a recommended way to combine a correction VAT invoice with a primary document about a cost change. Refusal to use UKD is not, on this letter, a reason to deny the cost change. The taxpayer may still use decree 1137's correction invoice and a 402-FZ primary document. The letter does not authorize rewriting the original invoice.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`UKD_RULES_RESEARCHED=YES` for that boundary. The filling appendix was not transcribed.

### 2.6 Correction VAT invoice

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Government decree 1137
DOCUMENT_NUMBER=1137
DATE=26.12.2011
CURRENT_EFFECTIVE_STATUS=IPS_TEXT_INCLUDES_AMENDMENT_23.01.2026_NO_26
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102153432&page=1&rdk=12
PROVISION_OR_SECTION=Appendix 1, section II, point 7; appendix 2, section II
BINTRANS_INTERPRETATION=Two mechanisms exist and must not be collapsed. An error copy keeps the original line-1 number and date and adds a correction number and date. A correction invoice is a new document for a later change of price and/or quantity, with its own number and date, a reference to the original invoice, and before/after/increase/decrease lines. Neither mechanism rewrites the issued original in place.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`CORRECTION_INVOICE_RULES_RESEARCHED=YES`.

### 2.7 Tax basis and VAT treatment

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Tax Code part two, chapter 21 extract
DOCUMENT_NUMBER=117-FZ
DATE=05.08.2000
CURRENT_EFFECTIVE_STATUS=ARTICLES_143_THROUGH_162_2_ONLY
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?docbody=&nd=102067058
PROVISION_OR_SECTION=Articles 143-162.2 were present; articles 164, 168 and 169 were not in the extract
BINTRANS_INTERPRETATION=No VAT rate, freight exemption, or statutory tax formula is frozen. The 0.1B rule stands: a missing treatment is not zero, and a client or register rate is not authoritative. Article 149's heading confirms that exemption operations exist, but this stage did not read which operations they are.
STATUS=NEEDS_FURTHER_RESEARCH
```

`TAX_BASIS_RESEARCHED=PARTIAL`. `VAT_TREATMENT_RESEARCHED=PARTIAL`.

### 2.8 Rounding

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=248-FZ and decree 1137
DOCUMENT_NUMBER=248-FZ; 1137
DATE=23.07.2013; 26.12.2011
CURRENT_EFFECTIVE_STATUS=AMENDMENT_TEXT_READ; CONSOLIDATED_ARTICLE_52_NOT_RE-READ
OFFICIAL_URL=http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=689a20c8e904e3ca05cb9f9c35c87201577a0d268ea08120ff83baea8d28dee5
PROVISION_OR_SECTION=248-FZ article 1 item 9; decree 1137 appendix 1 section II point 3
BINTRANS_INTERPRETATION=These are different scales. Decree 1137 requires invoice cost indicators in columns 4-6, 8 and 9 to be stated in rubles and kopecks, or the equivalent subunits of another currency. The 2013 amendment tells the tax amount under Tax Code article 52 to be computed in full rubles, dropping less than 50 kopecks and rounding 50 kopecks upward. That full-ruble rule is not the invoice-column rule. This stage does not adopt either rule as the line-VAT algorithm.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`TAX_ROUNDING_POLICY_PROPOSED=INVOICE_KOPECK_SCALE_SEPARATE_FROM_FULL_RUBLE_TAX_AMOUNT; LINE_AND_DOCUMENT_ALGORITHM_UNRESOLVED`.

### 2.9 Numbering and dates

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Government decree 1137; Federal Law 402-FZ
DOCUMENT_NUMBER=1137; 402-FZ
DATE=26.12.2011; 06.12.2011
CURRENT_EFFECTIVE_STATUS=PROVISIONS_READ
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102153432&page=1&rdk=12
PROVISION_OR_SECTION=Decree 1137 appendix 1 section II point 1(a) and point 7; appendix 2 section II point 1(a) and 1(c); 402-FZ article 9 parts 2 and 3
BINTRANS_INTERPRETATION=A VAT invoice carries a serial number and a compilation date. A separate subdivision appends a slash and the tax-policy index. Correction invoices and invoices share one chronology. An error copy must not change line 1. A correction invoice has its own number and date and cites the original number and date. A primary document's date is its compilation date.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`DOCUMENT_NUMBERING_POLICY_PROPOSED=SHARED_CHRONOLOGY_FOR_INVOICE_AND_CORRECTION_INVOICE; ORIGINAL_NUMBER_IMMUTABLE_ON_ERROR_COPY`.

`DOCUMENT_DATE_POLICY_PROPOSED=COMPILATION_DATE; ORIGINAL_LINE_1_DATE_IMMUTABLE_ON_ERROR_COPY; CORRECTION_INVOICE_HAS_ITS_OWN_DATE`.

### 2.10 Party requisites and lines

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Government decree 1137; Federal Law 402-FZ
DOCUMENT_NUMBER=1137; 402-FZ
DATE=26.12.2011; 06.12.2011
CURRENT_EFFECTIVE_STATUS=PROVISIONS_READ
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102153432&page=1&rdk=12
PROVISION_OR_SECTION=Decree 1137 appendix 1 lines 2, 2a, 2b, 6, 6a, 6b and columns 1a and 3-9; point 2.3; 402-FZ article 9 part 2
BINTRANS_INTERPRETATION=VAT-invoice parties require legal name, registry address, INN and KPP. A generic tax_id must not be copied into INN. KPP is separate. Primary documents require the compiling entity's name and do not, in article 9, require INN. Invoice lines require the description and the cost and tax columns. Quantity, unit and price may be dashed when those indicators are absent. A freight service therefore is not forced by this decree to invent a quantity.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`PARTY_REQUISITES_RESEARCHED=YES`. `MANDATORY_LINES_RESEARCHED=YES`.

### 2.11 Signature and Finance to EDO

```text
AUTHORITY=pravo.gov.ru and publication.pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=402-FZ; decree 1137; FNS orders ED-7-26/970@ and ED-1-26/29@
DOCUMENT_NUMBER=402-FZ; 1137; ED-7-26/970@; ED-1-26/29@
DATE=06.12.2011; 26.12.2011; 19.12.2023; 19.01.2026
CURRENT_EFFECTIVE_STATUS=PAPER_AND_PRIMARY_SIGNATURE_READ; XML_AND_CERTIFICATE_RULES_NOT_READ
OFFICIAL_URL=http://publication.pravo.gov.ru/document/0001202401290019
PROVISION_OR_SECTION=402-FZ article 9 part 5; decree 1137 appendix 1 form signature block and points 7 and 8
BINTRANS_INTERPRETATION=A paper VAT invoice is signed by the head and chief accountant, or by authorised persons, or by an individual entrepreneur with OGRNIP. An electronic invoice does not form the chief-accountant indicator. 402-FZ requires an electronic signature on an electronic primary document and does not, in the sentence read, say "qualified". The qualified-signature sentence of Tax Code article 169 was not read. XML generation, operator exchange, certificate checks, EDO state and artifact storage stay with Agent B.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`SIGNATURE_REQUIREMENTS_RESEARCHED=PARTIAL`. `FINANCE_EDO_BOUNDARY_CONFIRMED=YES`.

## 3. Document-field matrix

`REQUIRED` here means required by the source named, not by an unimplemented BINTRANS schema.

| Field | Payment invoice | Act / primary document | VAT invoice | UPD status 1 | UPD status 2 | Correction invoice / UKD |
| --- | --- | --- | --- | --- | --- | --- |
| Statutory object | Not found | 402-FZ article 9 | Decree 1137 appendix 1 | Letter MMV-20-3/96@ plus both regimes | Letter MMV-20-3/96@ plus 402-FZ | Decree 1137 appendix 2; UKD letter is optional |
| Document name | Not statutory | Required | Form title | Required for the primary part | Required | Form title |
| Own number | Not statutory | Not in the seven requisites | Serial number, line 1 | Inherits invoice numbering when it is an invoice | Not an invoice number | Own serial number in the shared chronology |
| Compilation date | Not statutory | Required | Line 1 | Both regimes | Required | Own date, line 1 |
| Predecessor number and date | None | Not the VAT mechanism | Unchanged on an error copy | Same split | Same split | Line 1b |
| Seller and buyer legal name | Not statutory | Compiling entity is required; counterparty follows from the fact description | Lines 2 and 6 | Required | Compiling entity required | Copied from the original invoice |
| Registry address | Not statutory | Not in article 9 | Lines 2a and 6a | Required for the invoice part | Not an invoice field | Copied from the original |
| INN and KPP | Not statutory | Not in article 9 | Lines 2b and 6b | Required for the invoice part | Not established by article 9 | Copied from the original |
| Generic `tax_id` | Must not fill INN | Must not be relabeled INN | Must not fill INN | Must not fill INN | Must not fill INN | Must not fill INN |
| Currency | Not statutory | Required if the measure is monetary | Line 7 | Required for the invoice part | Required when the fact is monetary | Copied from the original |
| Service or goods lines | Not statutory | Natural and/or monetary measure | Column 1a description plus cost and tax columns | Both | Measure required | Before, after, increase, decrease |
| Quantity, unit, price | Not statutory | Unit required when a natural measure is used | May be dashed when absent | Same | Same | Before and after when price or quantity changes |
| VAT rate and VAT amount | Not a VAT document | Not required by article 9 | Columns 7 and 8 when the invoice states tax | Required | Must not be treated as a VAT invoice | Before, after, and delta |
| Consignor and consignee | Not statutory | Not in article 9 | May be dashed for works and services | Same for the invoice part | Not an invoice field | Not the correction-invoice identity |
| Paper signatory | Not statutory | Position, signature, identifying requisites | Head and chief accountant, or authorised persons, or entrepreneur | Both regimes if paper | 402-FZ | Same signature block as the invoice form |
| Legal XML | None | Agent B if electronic | Agent B | Agent B | Agent B | Agent B, order ED-1-26/29@ |

## 4. TaxDecision requirements

The 0.1B gate remains. Unknown treatment does not become zero, a register rate, a browser rate, or today's unpublished policy. This research adds the facts a future decision must be able to receive and return. It does not name a rate.

Inputs, all server-owned:

- tenant, commercial relationship, and execution reference already required by 0.1B
- relationship side, so a forwarder customer chain and supply chain do not share one decision
- customer party and provider party as transaction-scoped roles
- legal name, registry address, INN, and KPP of the seller and buyer roles, held as those legal identifiers
- currency
- line description of the work, service, or goods
- quantity, unit, and price only when those indicators exist; absence is not zero quantity
- exact-decimal monetary basis
- operation class needed to apply form rules already verified, at least goods versus works/services
- policy identity and version, still opaque until a later stage names legal treatments
- compilation context time captured with the decision

Outputs:

- treatment code, or `UNRESOLVED`
- exact-decimal tax amount only when the treatment is resolved
- the scale at which that amount was produced
- provenance of the policy source
- no output that silently fills INN from `tax_id`

A resolved decision is still required before a tax-bearing VAT invoice, UPD status 1, or correction invoice states a rate or a tax amount. An act that only records the commercial fact may exist as a primary document, but 0.1B still blocks a tax-inclusive accounting close while treatment is unresolved. This stage does not split those gates further.

## 5. VAT-treatment matrix

| Treatment question | Result of this stage | Status |
| --- | --- | --- |
| Client VAT rate | Not authoritative. Unchanged from 0.1B. | VERIFIED_PRIMARY_SOURCE as an architecture rule; no Code rate was read |
| Register VAT rate | Not authoritative. Unchanged from 0.1B. | VERIFIED_PRIMARY_SOURCE as an architecture rule |
| Missing rate means 0% | Forbidden. Decree 1137 uses an explicit "без НДС" entry for operations listed in Tax Code article 168 point 5. That list was not read, so "без НДС" cannot be selected by default. | NEEDS_FURTHER_RESEARCH |
| Statutory rates | Not chosen. Articles 164 and 168 were not in the retrieved extract. | NEEDS_FURTHER_RESEARCH |
| Freight exemption | Not chosen. Article 149 was not read beyond its heading. | NEEDS_FURTHER_RESEARCH |
| Place of supply for transport | Article 148 is in the extract and was not interpreted for BINTRANS lanes. | NEEDS_FURTHER_RESEARCH |
| Advance invoice | Decree 1137 point 4 allows dashes in specified lines for an advance invoice. When freight must issue one was not read in article 168. | NEEDS_FURTHER_RESEARCH |
| Generic `tax_id` equals INN | No. The decree asks for the taxpayer identification number and the registration reason code. | VERIFIED_PRIMARY_SOURCE |

## 6. Rounding decision table

| Boundary | Proposed rule | Status |
| --- | --- | --- |
| In-process money | Exact decimal. Unchanged from 0.1B. | VERIFIED architecture, not a tax statute |
| API money | Decimal string. Unchanged from 0.1B. | VERIFIED architecture, not a tax statute |
| Generic persistence | Half away from zero, scale 2, persistence boundary only. This is not a statutory VAT rule. | 0.1B architecture; not a tax statute |
| Invoice columns 4-6, 8 and 9 | State the amount in rubles and kopecks, or the subunits of the invoice currency. | VERIFIED_PRIMARY_SOURCE |
| Line VAT rounding algorithm | Not chosen. | NEEDS_FURTHER_RESEARCH |
| Tax on a rate subtotal | Not chosen. | NEEDS_FURTHER_RESEARCH |
| Document VAT versus sum of line VAT | Not chosen. | NEEDS_FURTHER_RESEARCH |
| Final document total | The form has a "Всего к оплате" row. The decree says that row is filled for the purchase and sales books. It does not state a second rounding algorithm. | NEEDS_FURTHER_RESEARCH |
| Tax amount under Tax Code article 52 | The 2013 amendment computes that tax amount in full rubles: below 50 kopecks is dropped; 50 kopecks and above rounds up to the ruble. | VERIFIED_PRIMARY_SOURCE as the amending sentence |
| Use of the full-ruble rule on an invoice line | Not adopted. It contradicts the kopeck invoice columns if applied there. | BINTRANS interpretation; consolidated article 52 was not re-read |

`TAX_ROUNDING_POLICY_FROZEN=NO` remains correct for the statutory algorithm.

## 7. Correction-document model

Issued financial facts stay immutable. A correction is a successor.

| Event | Successor | Identity | Original number and date | Quantity, price, tax | Semantics |
| --- | --- | --- | --- | --- | --- |
| Error that prevents identification of seller, buyer, name, cost, rate, or tax | New copy of the same invoice | Line 1a correction number and correction date | Line 1 unchanged | The new copy states the corrected indicators | Replacement copy, not a delta and not an in-place edit |
| Error that does not prevent that identification | No new invoice copy | None | Unchanged | Unchanged | Decree 1137 appendix 1 point 7 |
| Price and/or quantity change after the invoice | Correction invoice | Own line-1 number and date, shared chronology | Cited in line 1b | Lines A before, B after, V increase, G decrease | Delta against the predecessor, plus the resulting values |
| Same cost change recorded as UKD | Optional combined document | Same correction-invoice identity plus a primary document | Same predecessor reference | Same delta | Does not replace the decree 1137 document |
| Primary-document mistake that is not a VAT invoice | Correction under 402-FZ article 9 part 7 | Correction date and signatures on the primary document | Not a VAT line-1 rule | As the corrected fact states | Allowed only where no federal law requires another method |
| Electronic artifact | Agent B stores the new legal XML | Finance stores the accounting successor identity | Finance retains the predecessor reference | Finance supplies the monetary delta or replacement facts | Agent B does not calculate them |

The 19.01.2026 FNS order ED-1-26/29@ approves electronic formats for the correction invoice and for a buyer consent or notification document. Its XML was not read. Whether buyer consent is a legal precondition for every freight cost change remains `NEEDS_FURTHER_RESEARCH`.

## 8. Finance versus EDO

| Responsibility | Owner | This stage |
| --- | --- | --- |
| Monetary truth and exact-decimal amounts | Agent G | Confirmed |
| Accounting facts and primary-document requisite requirements | Agent G | Confirmed from 402-FZ |
| TaxDecision inputs and the unknown-tax gate | Agent G | Extended, not implemented |
| VAT-invoice and correction-invoice accounting fields | Agent G | Taken from decree 1137 |
| Correction monetary semantics, delta versus replacement | Agent G | Confirmed |
| Legal XML and format generation | Agent B | Orders 970@ and ED-1-26/29@ exist; schema not copied here |
| EDO operator exchange, certificate verification, signature material, EDO state, artifact storage | Agent B | Not moved into Finance |
| Binding | Both | Finance must be able to cite the Agent B artifact. The artifact must reproduce Finance amounts and must not recompute them. |

`FINANCE_EDO_BOUNDARY_CONFIRMED=YES`.

## 9. Unresolved legal questions

```text
UNRESOLVED_LEGAL_QUESTIONS=12
```

1. The current consolidated text of Tax Code article 169, including its mandatory invoice fields and any qualified-signature sentence.
2. The current consolidated text of Tax Code article 168, including the tax formula and the operations that are marked "без НДС".
3. Whether Tax Code article 52 point 6 still has the 2013 wording, and the official boundary between that full-ruble tax amount and invoice kopecks.
4. The kopeck algorithm for one invoice line's VAT.
5. Whether document VAT is the sum of line VAT amounts or a tax computed on the document base.
6. Statutory VAT rates.
7. Whether any freight, forwarding, or international-transport operation is exempt or zero-rated, including Tax Code articles 148 and 149.
8. When an advance VAT invoice is required for freight.
9. Whether buyer consent is legally required before a correction invoice, as distinct from the existence of an electronic consent format.
10. The field appendix of FNS letters MMV-20-3/96@ and MMV-20-15/86@ beyond the status and recommendation already read.
11. The XML element dictionaries of FNS orders ED-7-26/970@ and ED-1-26/29@. This is Agent B work, and Finance still needs the resulting field map before implementation.
12. Which electronic-signature kind satisfies an electronic VAT invoice, given that 402-FZ article 9 part 5 says "электронной подписью" and the Code sentence was not read.

## 10. Recommendation for the next architecture freeze

Do not start implementation. Do not treat this research as a VAT-rate freeze.

The next safe architecture stage can freeze only the verified boundaries:

- a payment invoice is not a statutory accounting or VAT document
- an act follows 402-FZ article 9 and has no federal blank in the sources read
- a VAT invoice follows decree 1137 and is a line document
- UPD and UKD are optional recommended combinations, not mandatory blanks
- error correction is a replacement copy with an immutable original number and date
- a post-shipment price or quantity change is a delta successor
- INN and KPP are legal party identifiers and are not `tax_id`
- invoice money is stated in kopecks; the full-ruble article 52 rule is not the invoice-column rule
- Agent B keeps XML, signature material, and EDO state

Rates, exemptions, the kopeck algorithm, and article 169's unread sentences stay open. FINANCE-0.2A may later implement exact decimal and the ban on client totals only if it does not select a tax rate or a statutory rounding algorithm. That implementation is not authorized here.

```text
VERIFIED_PRIMARY_SOURCE_RULE_COUNT=11
RECOMMENDED_NEXT_ARCHITECTURE_STAGE=FINANCE-0.1D-VERIFIED-DOCUMENT-RULE-FREEZE
IMPLEMENTATION_AUTHORIZED=NO
```

The eleven verified rule groups are: 402-FZ primary requisites; decree 1137 invoice identity and parties; decree 1137 line and dash rules; decree 1137 kopeck cost columns; decree 1137 error-copy rule; decree 1137 correction-invoice delta; shared numbering chronology; UPD recommendation and statuses; UKD recommendation; article 52 full-ruble amendment as a separate rule; Finance versus EDO ownership of the 2023 and 2026 format orders.
