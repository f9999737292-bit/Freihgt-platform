# FINANCE-0.1C Russian accounting document research

```text
STAGE=FINANCE-0.1C-RUSSIAN-ACCOUNTING-DOCUMENT-RESEARCH
R1_STAGE=FINANCE-0.1C-R1-RUSSIAN-ACCOUNTING-DOCUMENT-LEGAL-SOURCE-COMPLETION
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
TAX_DECISION_IMPLEMENTED=NO
VAT_RATE_RUNTIME_IMPLEMENTED=NO
VAT_EXEMPTION_RUNTIME_IMPLEMENTED=NO
GLOBAL_BINTRANS_VAT_RATE=NOT_SET
PRODUCT_CODE_CHANGED=NO
```

This stage records what official texts say, what BINTRANS may rely on, and what is still open. It does not implement a document, a tax decision, an XML file, or a signature.

A commercial database, vendor page, blog, or forum was not used to freeze a rule. Where a search result summarized a statute without the statute text, that summary was ignored.

R1 reads the 2026 primary texts that 0.1C had not retrieved. A statutory catalog fact is not a runtime rate. TaxDecision selects a treatment from transaction facts. A carrier being in BINTRANS does not determine the VAT rate.

`STATUS` means:

- `VERIFIED_PRIMARY_SOURCE`: the cited official text was read in this stage.
- `CONDITIONAL`: the statute states a treatment, and it applies only when the stated conditions and evidence are present.
- `NEEDS_FURTHER_RESEARCH`: the question is in scope and the controlling sentence was not read.
- `NOT_APPLICABLE`: the reviewed primary sources do not make this object the freight treatment under discussion.
- `DEFERRED`: the requirement is acknowledged, and the artifact belongs to Agent B.

## 1. Normative-source matrix

| ID | Authority | Official act or order | Number | Date | Status reviewed | Official URL | What was read |
| --- | --- | --- | --- | --- | --- | --- | --- |
| SRC-402 | pravo.gov.ru IPS | Federal Law on accounting | 402-FZ | 06.12.2011; article 9 text includes the amendments cited in that article through 26.07.2019 No. 247-FZ | Consolidated article 9 read | http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=2cd1c64de5d2c2f028fbd68e725846eeae60fdd78e4a90d57b1f9e110477ba36 | Article 9 in full |
| SRC-1137 | pravo.gov.ru IPS | Government decree on VAT document forms | 1137 | 26.12.2011; IPS revision line includes amendments through 23.01.2026 No. 26 | Consolidated appendices 1 and 2 read in 0.1C | http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102153432&page=1&rdk=12 | Invoice form, filling rules, correction-invoice form and filling rules |
| SRC-248 | pravo.gov.ru IPS | Federal Law amending the Tax Code | 248-FZ | 23.07.2013 | Amending article read | http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=689a20c8e904e3ca05cb9f9c35c87201577a0d268ea08120ff83baea8d28dee5 | Article 1, item 9, adding Tax Code part one article 52 point 6 |
| SRC-NK1 | pravo.gov.ru IPS | Tax Code part one | 146-FZ | 31.07.1998; IPS edition dated 01.10.2026; amending-act list includes 28.11.2025 No. 425-FZ and later acts through 04.08.2026 No. 292-FZ | Article 52 point 6 re-read | http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=b113c2e08341853ef53a8dad4585b513d96f85e0f3d0d246a25ecf52e40608db | Current article 52 point 6 |
| SRC-UPD | nalog.gov.ru | FNS letter | MMV-20-3/96@ | 21.10.2013 | Official FNS publication of the letter's recommendation read; the letter appendix was not read line by line | https://www.nalog.gov.ru/rn77/about_fts/about_nalog/4319219/ and https://www.nalog.gov.ru/rn11/news/tax_doc_news/4345136/ | Recommendation, status 1 and status 2 |
| SRC-UKD | nalog.gov.ru | FNS letter | MMV-20-15/86@ | 17.10.2014 | Official FNS publication of the recommendation read; the letter appendix was not read line by line | https://www.nalog.gov.ru/rn62/news/tax_doc_news/4984071/ and https://www.nalog.gov.ru/rn77/news/tax_doc_news/4980752/ | Recommendation and non-mandatory character |
| SRC-970 | publication.pravo.gov.ru | FNS order | ED-7-26/970@ | 19.12.2023; registered 26.01.2024 No. 77001; published 29.01.2024 | Title and publication record read; XML schema not read | http://publication.pravo.gov.ru/document/0001202401290019 | Existence and document classes covered by the title |
| SRC-29 | publication.pravo.gov.ru | FNS order | ED-1-26/29@ | 19.01.2026; registered 17.03.2026 No. 85644; published 19.03.2026 | Title and publication record read; XML schema not read | http://publication.pravo.gov.ru/document/0001202603190004 | Existence of electronic correction-invoice and buyer consent/notification formats |
| SRC-NK2 | pravo.gov.ru IPS | Tax Code part two | 117-FZ | 05.08.2000; amending-act list through 04.08.2026 No. 292-FZ, including 28.11.2025 No. 425-FZ | Articles 148, 149 exemption headings relevant to carriage, 163, 164, 168, 169, and 172 point 10 read on 2026-10-13 | http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102067058&fulltext=1 | Current consolidated wording. The list's last federal law is 04.08.2026 No. 292-FZ |
| SRC-425 | publication.pravo.gov.ru official text | Federal Law amending the Tax Code | 425-FZ | 28.11.2025; publication number 0001202511280017 | Official text read, not an FNS summary | http://publication.pravo.gov.ru/document/0001202511280017 | Article 2 points amending articles 164 and 169; article 25 force clauses; article 36 form transition. No amendment of article 168 |
| SRC-PP26 | publication.pravo.gov.ru official text | Government decree amending decree 1137 | 26 | 23.01.2026; publication number 0001202601230017; IPS redaction id 486774 | Official text read | http://publication.pravo.gov.ru/document/0001202601230017 | Force clause and the restated invoice, correction invoice, journal, purchase book, and sales book |

`PRIMARY_SOURCE_COUNT=11`.

`SRC_425_REVIEWED=YES`. `PP26_REVIEWED=YES`. `TAX_CODE_ARTICLE_148_REVIEWED=YES`. `TAX_CODE_ARTICLE_149_REVIEWED=YES` for the carriage-exemption subpoints that were located. `TAX_CODE_ARTICLE_164_REVIEWED=YES`. `TAX_CODE_ARTICLE_168_TEXT_READ=YES`. `TAX_CODE_ARTICLE_169_TEXT_READ=YES`.

The IPS consolidation of part two is the current text as listed through Federal Law 04.08.2026 No. 292-FZ. 425-FZ is in that list. A federal law adopted after 04.08.2026 and before the research date is not in the list that was read.

## 2. Federal Law 28.11.2025 No. 425-FZ

Official publication: 28.11.2025, publication number 0001202511280017. The body was read from the official-publication text.

Article 2 of 425-FZ, for BINTRANS:

- Article 164 point 3: the words "20 процентов" are replaced with "22 процента".
- Article 164 point 1: a new subpoint 6.3 on ores and concentrates for refining. This is not a freight rule.
- Article 164 point 7: a taxpayer may elect not to apply the point 1 zero rate to the listed export and transport operations, and may then apply the rates in points 2, 3, and 8, for at least 12 months, by a statement filed before the tax period.
- Article 164 point 8: one added paragraph on simplified-tax concessionaires for listed utility systems. The 5 percent and 7 percent rates themselves are not created by 425-FZ.
- Article 164 point 9: a first-time user of a point 8 rate may refuse it within four consecutive tax periods. The minimum application period otherwise remains 12 consecutive tax periods.
- Article 165 point 9: consequential replacement of 20 percent with 22 percent, and references to the rates in points 3 and 8 of article 164.
- Article 169 point 5 subpoint 4: the shipment invoice must also state the serial number and date of the invoice issued on receipt of the advance that is being offset.
- Article 169 point 6: an individual entrepreneur's invoice states the primary state registration number of the entrepreneur and the date that number was assigned, in place of the state-registration certificate.
- Article 169 point 8: the words that the Government establishes the forms are replaced with approval by the federal executive body authorised for tax control and supervision.

425-FZ does not amend article 168. The plain text contains no amendment of that article.

Force, from article 25 of 425-FZ:

- Subpoints "а" and "б" of point 10 of article 2, which are the article 169 advance-invoice reference and the entrepreneur registration number, enter into force one month after official publication and not earlier than the first day of the next tax period for the relevant tax. Publication is 28.11.2025. Article 163, read in the current code, sets the VAT tax period as a quarter. The resulting date for those VAT amendments is 01.01.2026.
- Point 3 of article 164, in the wording of 425-FZ, applies to goods, works, services, and property rights shipped, performed, rendered, or transferred from 01.01.2026. That is article 25 point 13.
- Subpoint "в" of point 10 of article 2, the transfer of form approval away from the Government, enters into force on 01.10.2027. Article 36 says that until the tax authority approves the forms under the amended point 8, the forms established by the Government under the previous point 8 remain in force.

On the research date the Government forms are still the operative forms. Decree 1137, as amended by decree 26, is that operative instrument. The later transfer of approval power is not collapsed into the 2026 form.

## 3. Government decree 23.01.2026 No. 26

Official publication: 23.01.2026, publication number 0001202601230017. The decree amends decree 1137. It enters into force two months after official publication and not earlier than the first day of the next VAT tax period.

Two months after 23.01.2026 is 23.03.2026. The VAT tax period is a quarter. The first day of the next VAT tax period on or after that date is 01.04.2026. The official-text redaction date stored for this decree is 2026-04-01.

```text
CURRENT_1137_FORM_EFFECTIVE_DATE=2026-04-01
CURRENT_2026_INVOICE_FORM_VERIFIED=YES
CURRENT_2026_NEW_INVOICE_FIELDS_CAPTURED=YES
```

On 2026-10-13 the decree 26 form is the current form. The 0.1C matrix, taken from the consolidated decree, did not record the fields decree 26 adds or restates as 2026 additions. Those fields are:

- Invoice line 5b: serial number and date of the invoice issued on receipt of an advance, partial payment, or other payment toward future supplies, and the number and date of the last correction of that invoice. Several such invoices are separated by a semicolon. The seller fills line 5b when the shipment invoice is issued against that advance. This is the decree line that corresponds to the 425-FZ addition in article 169 point 5 subpoint 4. Line 5 remains the payment document. The Code puts both references in one subpoint. The decree keeps them on separate lines. They are not one field.
- Invoice and correction-invoice signature block for an individual entrepreneur: primary state registration number and the date it was assigned. The same replacement is made in the journal, purchase book, and sales book.
- Purchase book column 7a, and the same column on the additional sheet: number and date of the shipment invoice issued against an earlier advance, filled when column 3 records the advance invoice from which the seller deducts tax under article 172 point 6.
- Sales book column 11a, and the same column on the additional sheet: number and date of the earlier advance invoice shown in line 5b of the shipment invoice.
- Sales book rate columns, in the form restated by decree 26: 22 percent, 20 or 18 percent, 10 percent, 7 percent, 5 percent, and 0 percent. The filling rule replaces the words "16,67 процента" with "18,03 процента" and replaces "20 процентов" with "22 процентов" in the stated sales-book subpoints. That 18.03 percent label is a book-column label. It is not a statutory line-rounding algorithm.
- Traceability cost in the invoice, correction invoice, journal, and books: rubles and kopecks, where the previous wording said rubles.

Article 169 is the statutory requisite list. Decree 1137, as amended by decree 26, is the operative form and filling rule. The Code does not draw line 5b. The decree does. The decree's book columns are not additional statutory invoice requisites.

## 4. Document research

### 4.1 Commercial invoice / счёт на оплату

```text
AUTHORITY=NONE_FOUND_IN_REVIEWED_SOURCES
OFFICIAL_ACT_OR_ORDER=NONE
DOCUMENT_NUMBER=
DATE=
CURRENT_EFFECTIVE_STATUS=NO_GENERAL_MANDATORY_STATUTORY_FORM_IDENTIFIED_IN_REVIEWED_SOURCES
OFFICIAL_URL=
PROVISION_OR_SECTION=
BINTRANS_INTERPRETATION=The reviewed primary sources do not identify a general mandatory statutory form for a payment invoice. That is a statement about the sources read. It is not a proof that no statute or contract can ever require one. A payment invoice is not the VAT invoice regulated by article 169 and decree 1137, and it is not, by itself, the primary accounting document whose seven requisites are set by 402-FZ article 9. BINTRANS must not treat a browser payment invoice as accounting truth or as a VAT invoice. A particular contract may still require a payment invoice.
STATUS=NOT_APPLICABLE
```

`INVOICE_RULES_RESEARCHED=YES` for this source-bounded finding. `PAYMENT_INVOICE_NEGATIVE_CLAIM_NARROWED=YES`.

### 4.2 Act / акт

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Federal Law on accounting
DOCUMENT_NUMBER=402-FZ
DATE=06.12.2011
CURRENT_EFFECTIVE_STATUS=ARTICLE_9_TEXT_READ
OFFICIAL_URL=http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=2cd1c64de5d2c2f028fbd68e725846eeae60fdd78e4a90d57b1f9e110477ba36
PROVISION_OR_SECTION=Article 9, parts 1, 2, 3, 4, 5 and 7
BINTRANS_INTERPRETATION=402-FZ article 9 does not prescribe a unified act form. An act of services or works is a primary accounting document when it records a real fact and carries the seven statutory requisites. The economic entity approves the form. The document date is the compilation date, at the time of the fact or immediately after it. A later correction of a primary document must show the correction date and the signatures of the persons who compiled the corrected document, unless another federal law forbids that method. A VAT invoice is such another regime.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`ACT_RULES_RESEARCHED=YES`. `ACT_NEGATIVE_CLAIM_NARROWED=YES`. The seven requisites are: document name; compilation date; name of the economic entity that compiled it; content of the fact; natural and/or monetary measure with units; position of the responsible person; signatures with surname and initials or other identifying requisites.

### 4.3 VAT invoice / счёт-фактура

```text
AUTHORITY=pravo.gov.ru and publication.pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Tax Code article 169 and Government decree 1137 as amended by decree 26
DOCUMENT_NUMBER=169; 1137; 26
DATE=current consolidated article 169; decree 26 dated 23.01.2026
CURRENT_EFFECTIVE_STATUS=CODE_REQUISITES_READ; FORM_EFFECTIVE_2026-04-01
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102067058&fulltext=1
PROVISION_OR_SECTION=Article 169 points 5, 5.1, 5.2, 6 and 8; decree 26 appendix 1
BINTRANS_INTERPRETATION=The Tax Code states the statutory requisites. Decree 1137 states the operative lines and filling rules. Seller and buyer name, address, and identification numbers are statutory. For works and services the decree allows dashes in the consignor and consignee lines. Cost columns are in currency units and subunits, including rubles and kopecks. An unlabeled company tax_id is not the identification number. From 01.04.2026 the form has line 5b for the advance invoice being offset. The entrepreneur signs with the primary state registration number and its assignment date.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`VAT_INVOICE_RULES_RESEARCHED=YES`. `TAX_CODE_ARTICLE_169_TEXT_READ=YES`.

Statutory invoice requisites in article 169 point 5, after the 425-FZ amendment already present in the consolidated text, include: serial number and date; name, address, and identification numbers of the taxpayer and the buyer; consignor and consignee name and address; payment-document number when an advance was received, the serial number and date of the advance invoice being offset, and the requisites that identify the shipment document; line number, name or description, and unit when it can be stated; quantity when it can be stated; currency name; state-contract identifier when one exists; price per unit when it can be stated; cost without tax; excise; tax rate; tax amount presented to the buyer; cost with tax; country of origin; goods-declaration registration number; EAEU commodity code where the subpoint requires it; and the traceability number, unit, quantity, and cost where those subpoints apply.

An advance invoice under point 5.1 states number and date, names, addresses and identification numbers, payment-document number, description, currency, state-contract identifier when one exists, advance amount, tax rate, and tax amount.

A correction invoice under point 5.2 states its own name, number and date, the predecessor invoice number and date, party names and identification numbers, the changed description and unit, quantity and price before and after, currency, cost and tax before and after, the difference, and the origin and traceability indicators where required. The taxpayer may issue one correction invoice for two or more earlier invoices.

Article 169 point 6: a paper invoice is signed by the head and the chief accountant, or by persons authorised by an organisational act or a power of attorney. An individual entrepreneur, or a person authorised by the entrepreneur's power of attorney, signs and states the entrepreneur's primary state registration number and the date it was assigned. An invoice composed in electronic form is signed with an enhanced qualified electronic signature of the head, of another person authorised by an organisational act or a power of attorney, or of the individual entrepreneur.

Article 169 point 8, in the consolidated text current on the research date, still says the Government establishes the form and the filling procedure. The 425-FZ replacement of that sentence takes effect on 01.10.2027. Until the tax authority approves new forms, article 36 of 425-FZ keeps the Government forms in force.

### 4.4 UPD / УПД

```text
AUTHORITY=nalog.gov.ru and publication.pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=FNS letter MMV-20-3/96@; FNS order ED-7-26/970@
DOCUMENT_NUMBER=MMV-20-3/96@; ED-7-26/970@
DATE=21.10.2013; 19.12.2023
CURRENT_EFFECTIVE_STATUS=LETTER_REMAINS_A_RECOMMENDATION; ELECTRONIC_FORMAT_IS_THE_2023_ORDER
OFFICIAL_URL=https://www.nalog.gov.ru/rn11/news/tax_doc_news/4345136/
PROVISION_OR_SECTION=FNS publication of the letter; publication record of order ED-7-26/970@
BINTRANS_INTERPRETATION=The 2013 letter recommends UPD. Status 1 combines the invoice and the transfer document. Status 2 is a primary document only. The letter does not make UPD a mandatory blank. The statutory accounting facts of a status 1 document are the article 169 requisites plus the 402-FZ requisites. The statutory facts of a status 2 document are the 402-FZ requisites, and it is not a VAT invoice. Order ED-7-26/970@ is the electronic format whose title covers the electronic invoice and the electronic primary document. Its XML, schema, file generation, and operator transport remain with Agent B. Finance states the accounting facts that artifact must reproduce. The 2013 appendix was not transcribed line by line.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`UPD_RULES_RESEARCHED=YES`. `UPD_2026_STATUS_RESEARCHED=YES`. No later order repealing ED-7-26/970@ was read in this stage. The 2026 order that was read, ED-1-26/29@, is the correction-invoice and buyer-notification format, not a replacement of the 2013 letter.

### 4.5 UKD / correction UPD

```text
AUTHORITY=nalog.gov.ru and publication.pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=FNS letter MMV-20-15/86@; FNS order ED-1-26/29@
DOCUMENT_NUMBER=MMV-20-15/86@; ED-1-26/29@
DATE=17.10.2014; 19.01.2026
CURRENT_EFFECTIVE_STATUS=LETTER_REMAINS_A_RECOMMENDATION; 2026_ELECTRONIC_CORRECTION_FORMAT_EXISTS
OFFICIAL_URL=https://www.nalog.gov.ru/rn62/news/tax_doc_news/4984071/
PROVISION_OR_SECTION=FNS publication of the letter; publication record of order ED-1-26/29@
BINTRANS_INTERPRETATION=UKD is a recommended way to combine a correction VAT invoice with a primary document about a cost change. Refusal to use UKD is not, on this letter, a reason to deny the cost change. The taxpayer may use decree 1137's correction invoice and a 402-FZ primary document. The 2026 electronic format for the correction invoice and for a buyer consent or notification document is order ED-1-26/29@. Agent B owns that XML. Finance owns the monetary delta, the predecessor identity, and the fact that article 172 point 10 requires a document of consent or notification.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`UKD_RULES_RESEARCHED=YES`. `UKD_2026_STATUS_RESEARCHED=YES`. The 2014 filling appendix was not transcribed.

### 4.6 Correction VAT invoice

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Tax Code articles 168, 169 and 172; Government decree 1137
DOCUMENT_NUMBER=168; 169; 172; 1137
DATE=current consolidated text; decree 26 effective 01.04.2026
CURRENT_EFFECTIVE_STATUS=PROVISIONS_READ
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102067058&fulltext=1
PROVISION_OR_SECTION=Article 168 point 3; article 169 point 5.2; article 172 point 10; decree 1137 appendix 2
BINTRANS_INTERPRETATION=Two mechanisms exist and must not be collapsed. An error copy keeps the original line-1 number and date and adds a correction number and date. A correction invoice is a new document for a later change of price and/or quantity, with its own number and date, a reference to the original invoice, and before/after/increase/decrease lines. Article 168 point 3 requires the seller to issue the correction invoice no later than five calendar days from the date of the documents named in article 172 point 10. Article 172 point 10 allows the deduction of the difference only when a contract, agreement, or other primary document confirms the buyer's consent, or the fact of notification, to the cost change. The electronic shape of that document is Agent B's format.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`CORRECTION_INVOICE_RULES_RESEARCHED=YES`.

### 4.7 Article 168

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Tax Code part two
DOCUMENT_NUMBER=117-FZ
DATE=05.08.2000; consolidated text whose amending list runs through 04.08.2026 No. 292-FZ
CURRENT_EFFECTIVE_STATUS=ARTICLE_168_READ
OFFICIAL_URL=http://pravo.gov.ru/proxy/ips/?doc_itself=&nd=102067058&fulltext=1
PROVISION_OR_SECTION=Article 168 points 1 through 7
BINTRANS_INTERPRETATION=The seller must present the tax to the buyer in addition to the price. On an advance toward supplies realised in the Russian Federation, the seller presents tax computed by the calculated method in article 164 point 4. The presented amount is computed for each kind of goods, works, services, or property rights as the percentage share of the price that corresponds to the tax rate. The invoice is issued no later than five calendar days from shipment, performance, rendering, transfer, or receipt of the advance. Settlement documents, primary documents, and invoices state the tax as a separate line. The inscription "Без налога (НДС)" belongs to taxpayers released from taxpayer duties under article 145 point 1, who issue invoices without a tax amount. It is not a default freight marking. Taxpayers on the agricultural regime or the simplified regime who are released from VAT duties do not issue invoices.
STATUS=VERIFIED_PRIMARY_SOURCE
```

`TAX_CODE_ARTICLE_168_TEXT_READ=YES`. The earlier statement that article 168 was unavailable is withdrawn.

### 4.8 Rounding

```text
AUTHORITY=pravo.gov.ru
OFFICIAL_ACT_OR_ORDER=Tax Code part one article 52; Tax Code article 168; decree 1137 as amended by decree 26
DOCUMENT_NUMBER=146-FZ; 117-FZ; 1137; 26
DATE=article 52 edition 01.10.2026; decree 26 effective 01.04.2026
CURRENT_EFFECTIVE_STATUS=CURRENT_ARTICLE_52_POINT_6_READ
OFFICIAL_URL=http://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=b113c2e08341853ef53a8dad4585b513d96f85e0f3d0d246a25ecf52e40608db
PROVISION_OR_SECTION=Article 52 point 6; article 168 point 2; decree 1137 appendix 1 section II point 3; decree 26 sales-book labels
BINTRANS_INTERPRETATION=Article 52 is the order for computing the tax. Point 6 says the amount of tax and levy is computed in full rubles: a tax amount below 50 kopecks is dropped, and 50 kopecks or more rounds up to the ruble. The 31.07.2023 No. 389-FZ revision is already in that sentence; the full-ruble rule remains. That sentence governs the tax amount computed under article 52. It is not the invoice-column rule. Article 168 point 2 computes the tax presented to the buyer as a percentage share of the price for each kind of supply and states no full-ruble step. Decree 1137 requires invoice columns 4-6, 8 and 9 in rubles and kopecks. No reviewed primary source states a line-by-line kopeck rounding algorithm. An official FNS or Ministry of Finance letter that separately explains this distinction was not retrieved in this review. The distinction used here is the text of article 52 point 6 against article 168 point 2 and decree 1137.
STATUS=VERIFIED_PRIMARY_SOURCE
```

```text
BUDGET_TAX_ROUNDING=ARTICLE_52_POINT_6_FULL_RUBLE_FOR_THE_COMPUTED_TAX_AMOUNT
INVOICE_VAT_SCALE_RULE=RUBLES_AND_KOPECKS_ON_THE_INVOICE_COLUMNS
LINE_VAT_ROUNDING_ALGORITHM=NOT_STATUTORILY_IDENTIFIED_IN_REVIEWED_PRIMARY_SOURCES
```

Internal money stays exact decimal. The full-ruble article 52 rule is not applied to an invoice line. This stage does not invent a kopeck algorithm.

### 4.9 Numbering and dates

A VAT invoice carries a serial number and a compilation date. A separate subdivision appends a slash and the tax-policy index. Correction invoices and invoices share one chronology. An error copy must not change line 1. A correction invoice has its own number and date and cites the original number and date. A primary document's date is its compilation date. Line 5b cites the advance invoice's number and date and does not replace line 1.

`DOCUMENT_NUMBERING_POLICY_PROPOSED=SHARED_CHRONOLOGY_FOR_INVOICE_AND_CORRECTION_INVOICE; ORIGINAL_NUMBER_IMMUTABLE_ON_ERROR_COPY`.

`DOCUMENT_DATE_POLICY_PROPOSED=COMPILATION_DATE; ORIGINAL_LINE_1_DATE_IMMUTABLE_ON_ERROR_COPY; CORRECTION_INVOICE_HAS_ITS_OWN_DATE`.

### 4.10 Party requisites and lines

VAT-invoice parties require legal name, address, and identification numbers. A generic `tax_id` must not be copied into that identification number. The registration-reason code is separate. Primary documents require the compiling entity's name and do not, in article 9, require an identification number. Quantity, unit, and price may be dashed when those indicators are absent. A freight service is not forced by the decree to invent a quantity. From 01.04.2026 the entrepreneur's signature line states the primary state registration number and the date of assignment.

`PARTY_REQUISITES_RESEARCHED=YES`. `MANDATORY_LINES_RESEARCHED=YES`.

### 4.11 Signature and Finance to EDO

A paper VAT invoice is signed by the head and chief accountant, or by authorised persons, or by an individual entrepreneur with the primary state registration number and its assignment date. An electronic VAT invoice is signed with an enhanced qualified electronic signature under article 169 point 6. 402-FZ article 9 part 5 requires an electronic signature on an electronic primary document and, in the sentence read, does not say "qualified". Those two sentences are not the same rule.

XML generation, operator exchange, certificate checks, EDO state, and artifact storage stay with Agent B. Finance states the accounting facts the artifact must reproduce: parties, identification numbers, dates, predecessor references, line 5b when an advance is offset, currency, exact-decimal amounts, the tax treatment the server decided, and the consent-or-notification fact required by article 172 point 10.

`SIGNATURE_REQUIREMENTS_RESEARCHED=YES` for the VAT invoice. `FINANCE_EDO_BOUNDARY_CONFIRMED=YES`.

## 5. Document-field matrix

`REQUIRED` here means required by the source named, not by an unimplemented BINTRANS schema.

| Field | Payment invoice | Act / primary document | VAT invoice | UPD status 1 | UPD status 2 | Correction invoice / UKD |
| --- | --- | --- | --- | --- | --- | --- |
| Statutory object | No general mandatory form identified in the sources read | 402-FZ article 9 | Article 169 and decree 1137 appendix 1 | Letter MMV-20-3/96@ plus both regimes | Letter MMV-20-3/96@ plus 402-FZ | Article 169 point 5.2 and decree 1137 appendix 2; UKD letter is optional |
| Document name | Not identified as a statutory blank | Required | Form title | Required for the primary part | Required | Form title, and article 169 point 5.2 requires the name "корректировочный счет-фактура" |
| Own number | Not identified as a statutory blank | Not in the seven requisites | Serial number, line 1 | Inherits invoice numbering when it is an invoice | Not an invoice number | Own serial number in the shared chronology |
| Compilation date | Not identified as a statutory blank | Required | Line 1 | Both regimes | Required | Own date, line 1 |
| Predecessor number and date | None | Not the VAT mechanism | Unchanged on an error copy | Same split | Same split | Line 1b and article 169 point 5.2 subpoint 2 |
| Advance invoice being offset | None | Not an article 9 field | Line 5b from 01.04.2026; article 169 point 5 subpoint 4 | Required for the invoice part when an advance is offset | Not an invoice field | Not the correction-invoice identity |
| Seller and buyer legal name | Not identified as a statutory blank | Compiling entity is required; counterparty follows from the fact description | Lines 2 and 6; article 169 point 5 subpoint 2 | Required | Compiling entity required | Copied from the original invoice |
| Registry address | Not identified as a statutory blank | Not in article 9 | Lines 2a and 6a | Required for the invoice part | Not an invoice field | Copied from the original |
| Identification number and registration-reason code | Not identified as a statutory blank | Not in article 9 | Lines 2b and 6b | Required for the invoice part | Not established by article 9 | Copied from the original |
| Generic `tax_id` | Must not fill the identification number | Must not be relabeled as that number | Must not fill the identification number | Must not fill the identification number | Must not fill the identification number | Must not fill the identification number |
| Entrepreneur registration | Not identified as a statutory blank | Not in article 9 | Primary state registration number and assignment date | Same when the invoice part is signed by an entrepreneur | 402-FZ signature requisites | Same signature block |
| Currency | Not identified as a statutory blank | Required if the measure is monetary | Line 7; article 169 point 7 allows a foreign-currency obligation to be stated in that currency | Required for the invoice part | Required when the fact is monetary | Copied from the original |
| Service or goods lines | Not identified as a statutory blank | Natural and/or monetary measure | Column 1a description plus cost and tax columns | Both | Measure required | Before, after, increase, decrease |
| Quantity, unit, price | Not identified as a statutory blank | Unit required when a natural measure is used | May be dashed when absent; article 169 says "when it can be stated" | Same | Same | Before and after when price or quantity changes |
| VAT rate and VAT amount | Not a VAT document | Not required by article 9 | Columns 7 and 8; article 169 point 5 subpoints 10 and 11 | Required | Must not be treated as a VAT invoice | Before, after, and the difference |
| Consignor and consignee | Not identified as a statutory blank | Not in article 9 | May be dashed for works and services | Same for the invoice part | Not an invoice field | Not the correction-invoice identity |
| Paper signatory | Not identified as a statutory blank | Position, signature, identifying requisites | Head and chief accountant, or authorised persons, or entrepreneur | Both regimes if paper | 402-FZ | Same signature block as the invoice form |
| Electronic signature | None identified for this object | 402-FZ says "электронной подписью" | Article 169 point 6: enhanced qualified electronic signature | Qualified signature for the invoice part | The 402-FZ sentence | Article 169 point 6 |
| Legal XML | None | Agent B if electronic | Agent B, order ED-7-26/970@ | Agent B | Agent B | Agent B, order ED-1-26/29@ |

Sales-book columns 14 through 18b and purchase-book column 7a are book fields under decree 26. They are not extra invoice requisites. Finance must be able to supply the advance-invoice reference and the rate class that those columns record.

## 6. Article 164 rate catalog

The current consolidated article 164 was read. 425-FZ supplies the 22 percent replacement and its application date. The catalog is not a BINTRANS rate.

```text
STANDARD_RATE=22_PERCENT_ARTICLE_164_POINT_3
REDUCED_RATE=10_PERCENT_ARTICLE_164_POINT_2_FOR_THE_LISTED_GOODS
ZERO_RATE=ARTICLE_164_POINT_1_FOR_THE_LISTED_OPERATIONS
USN_SPECIAL_RATES=5_PERCENT_OR_7_PERCENT_ARTICLE_164_POINT_8
CURRENT_2026_STANDARD_VAT_RATE=22_PERCENT_FOR_CASES_NOT_LISTED_IN_POINTS_1_2_AND_4_ON_SUPPLIES_FROM_2026-01-01
CURRENT_2026_REDUCED_RATE_CATALOG=10_PERCENT_ARTICLE_164_POINT_2
CURRENT_2026_ZERO_RATE_PRESENT=YES
CURRENT_2026_USN_SPECIAL_RATE_CATALOG=5_PERCENT_OR_7_PERCENT_UNDER_ARTICLE_164_POINT_8
GLOBAL_BINTRANS_VAT_RATE=NOT_SET
```

Point 3: taxation is at 22 percent in cases not stated in points 1, 2, and 4. The revision note cites 425-FZ. Article 25 point 13 of 425-FZ applies that wording to supplies from 01.01.2026.

Point 2: taxation is at 10 percent for the goods listed in that point. The list is a goods list. It was not read as a freight-service rate.

Point 4: where the Code requires the calculated method, including an advance, the rate is the percentage relation of the rate in point 2, 3, or 8 to a tax base taken as 100 and increased by that rate. The Code states the relation. It does not state a line-rounding algorithm. Decree 26 labels a sales-book column "18,03 процента" in place of "16,67 процента". That label is not adopted here as the line algorithm.

Point 8: an organisation or entrepreneur on the simplified system may tax the relevant supplies at 5 percent or 7 percent when the stated income conditions are met. The Code states income magnitudes of 250 million rubles and 450 million rubles. Point 9 says those magnitudes are indexed in the manner of article 346.12 point 2. The act that sets the indexed 2026 magnitudes was not read. A supplier on point 8 does not apply points 2 and 3, and does not apply point 1 except for the operations listed in point 9. The 12-period commitment and the four-period first-time refusal are in point 9, as amended by 425-FZ.

Point 7: for the export and transport operations it lists, including subpoints 2.1 through 2.5 and 2.7 through 2.8-1 of point 1, the taxpayer may elect the rates in points 2, 3, and 8 instead of the zero rate. The election covers all such operations of that taxpayer and lasts at least 12 months.

`TAX_DECISION_IMPLEMENTED=NO`. `VAT_RATE_RUNTIME_IMPLEMENTED=NO`. `VAT_EXEMPTION_RUNTIME_IMPLEMENTED=NO`. The catalog is a legal fact. The runtime must not choose 22 percent because the carrier is in BINTRANS.

## 7. Freight VAT treatment matrix

Place of supply is decided before a rate. Article 148 point 1 subpoint 4.1: carriage and transportation, and services or works directly connected with them, are supplied in the Russian Federation when a Russian organisation or entrepreneur performs them and the point of departure or the point of destination is in the Russian Federation, or when a foreign person who is not registered as a taxpayer performs them and both points are in the Russian Federation. Subpoint 4.2 covers services directly connected with carriage of goods under customs transit from the place of arrival to the place of departure, when the performer's place of activity is the Russian Federation. Point 1.1 subpoint 5: carriage and directly connected services that are not listed in point 1 subpoints 4.1 through 4.3 are not supplied in the Russian Federation. A service that is not supplied in the Russian Federation is not a zero-rated Russian VAT supply. It is outside that charging rule.

Article 149, in the subpoints located by a carriage search, exempts stated passenger services: urban public passenger transport other than taxis, suburban carriage on stated modes under unified tariffs, and regular passenger and baggage carriage by road and urban electric transport at regulated tariffs under a state or municipal contract. Those subpoints are not a goods-freight exemption. No goods-freight or freight-forwarding exemption was located in the article 149 text that was searched. That search does not prove that no other sentence of article 149 could ever touch a transport service. It does establish that the passenger exemptions above are not the BINTRANS goods-freight rule.

Article 164 point 1 subpoint 2.1 taxes international carriage of goods at 0 percent. International carriage, for that article, means carriage by sea, river, mixed river-sea, air, rail, or road where the point of departure or the point of destination is outside the Russian Federation. The same subpoint includes stated transport-forwarding services under a freight-forwarding contract when they organise that international carriage, and it lists the kinds of services that count. Federal Law 22.04.2024 No. 92-FZ, already in the consolidated text, excludes a carrier or forwarder who only carries between points inside the Russian Federation and is not named as a carrier in the transport or shipping documents that confirm export or import. Subpoint 2.1 also does not apply to the Russian rail carriers described in subpoint 9. Confirmation documents are the subject of article 165. That article's document list was not transcribed. Point 7 allows a taxpayer to refuse this zero rate for the listed operations.

| OPERATION_CLASS | DOMESTIC_OR_INTERNATIONAL | LEGAL_BASIS | POTENTIAL_TREATMENT | REQUIRED_EVIDENCE | TAX_DECISION_INPUT | STATUS |
| --- | --- | --- | --- | --- | --- | --- |
| Carriage of goods with both ends in the Russian Federation, and not within a zero-rate subpoint | Domestic | Article 148 point 1 subpoint 4.1; article 164 point 3; article 164 points 7, 8 and 9 | 22 percent only when the supply is in the Russian Federation, the seller is on the general rate, and no zero-rate subpoint, exemption, or point 8 election applies | Route ends, seller tax status, absence of an export or transit document set, policy date | Seller status, operation class, freight versus forwarding, both route countries, service date, policy version | CONDITIONAL |
| International carriage of goods as defined in article 164 point 1 subpoint 2.1 | International | Article 164 point 1 subpoint 2.1; article 164 point 7; article 165 is the confirmation article and was not transcribed | 0 percent only when the statutory definition is met, the 92-FZ exclusion does not apply, the taxpayer has not elected out under point 7, and the article 165 documents are present | Departure or destination outside the Russian Federation, mode, carrier named in the transport documents, election status, article 165 documents | Route ends, mode, carrier identity on the transport document, border fact, election, evidence set, policy date | CONDITIONAL |
| Freight forwarding that organises an international carriage and is inside the subpoint 2.1 list | International | Article 164 point 1 subpoint 2.1, forwarding paragraph; 92-FZ exclusion | 0 percent only on the same conditions as the carriage it organises, and only for services the subpoint lists | Forwarding contract, the organised international carriage, the service kind, the 92-FZ test | Freight versus forwarding, contract type, route, service kind, documents | CONDITIONAL |
| Forwarder or carrier who only moves between Russian points and is not named in the export or import transport documents | Domestic leg of an international shipment | Article 164 point 1 subpoint 2.1, 92-FZ exclusion | Subpoint 2.1 does not apply. The treatment returns to the domestic row or to another subpoint that fits | Proof the movement is only between Russian points and that the person is not named as a carrier | Route ends, named-carrier fact | CONDITIONAL |
| Railway forwarding, or provision of rail rolling stock or containers, for export goods where both ends are in the Russian Federation | Domestic route, export-related | Article 164 point 1 subpoint 2.7 | 0 percent for the Russian organisations and entrepreneurs and the conditions stated in that subpoint | Export or re-export character, both ends in the Russian Federation, rail mode, exclusion of subpoints 9 and 2.1 | Mode, route, export fact, performer | CONDITIONAL |
| Inland-water or sea leg inside the Russian Federation that the statute treats as part of a further export | Domestic leg, export-related | Article 164 point 1 subpoints 2.8 and 2.8-1 | 0 percent only for the works and services those subpoints describe | Mode, export procedure, transshipment point | Mode, route, customs procedure | CONDITIONAL |
| Services directly connected with customs transit from arrival to departure | Transit | Article 164 point 1 subpoint 3; article 148 point 1 subpoint 4.2 | 0 percent when the transit conditions are met. Place of supply can be the Russian Federation when the performer is established there | Customs transit, arrival and departure offices, performer established in the Russian Federation | Transit fact, performer place of activity | CONDITIONAL |
| Carriage or forwarding that article 148 point 1.1 subpoint 5 does not place in the Russian Federation | Outside Russian place of supply | Article 148 point 1.1 subpoint 5 | Not a Russian VAT rate, including not an inferred 0 percent | Facts showing the service is outside subpoints 4.1 through 4.3 | Place-of-supply facts | CONDITIONAL |
| Passenger services exempted by the located article 149 subpoints | Domestic passenger | Article 149 point 2 subpoints 7, 7.1 and the suburban passenger paragraph | Exemption for those passenger services | The tariff, mode, and contract conditions of those subpoints | Not a goods-freight input | NOT_APPLICABLE |
| Advance toward a freight supply realised in the Russian Federation | Follows the underlying supply | Article 168 points 1 and 3; article 164 point 4 | Calculated method on the rate that applies to the underlying supply, and an invoice within five calendar days of receipt | Advance fact, underlying treatment, receipt date | Advance fact, underlying operation class, currency, receipt date | CONDITIONAL |
| Supplier on article 164 point 8 | Either | Article 164 points 8 and 9 | 5 percent or 7 percent instead of points 2 and 3, with the point 1 exceptions stated in point 9 | Seller regime, income test, election period, indexed threshold | Seller taxation regime, policy date | CONDITIONAL |

`DOMESTIC_FREIGHT_TREATMENT_RESEARCHED=YES`. `INTERNATIONAL_FREIGHT_TREATMENT_RESEARCHED=YES`. `FORWARDING_TREATMENT_RESEARCHED=YES`. `PLACE_OF_SUPPLY_RESEARCHED=YES`.

International carriage is not recorded as 0 percent without the conditions in the matrix. Article 165's confirmation documents remain unread, so a zero-rate decision cannot be closed from route facts alone.

## 8. TaxDecision input model

All of the following are server-owned. No browser or client field is authoritative tax truth. Unknown treatment still blocks a tax-bearing document and an accounting close. It is not zero, and it is not a register rate.

Assessed inputs:

| Input | Why it is required | Source of the need |
| --- | --- | --- |
| Seller legal and tax status | Point 3, point 8, and article 145 produce different treatments for the same carriage | Articles 164 and 168 |
| Buyer role | Forwarder chains stay separate; the buyer is also an invoice party | 0.1B and article 169 |
| Supplier taxation regime | Simplified-system point 8 rates, and the article 145 release, are seller facts | Article 164 points 8 and 9; article 168 point 5 |
| Operation class | Goods, works, and services are not one rate list | Article 164 |
| Goods versus service | Point 2 is a goods list; freight here is a service | Article 164 point 2 |
| Freight versus forwarding | Subpoint 2.1 states them separately, and the forwarding list is closed by that subpoint | Article 164 point 1 subpoint 2.1 |
| Domestic or international route | The zero-rate definition uses departure or destination outside the Russian Federation | Article 164 point 1 subpoint 2.1 |
| Origin country and destination country | Place of supply and the international-carriage definition both use the ends of the route | Articles 148 and 164 |
| Border or cross-border fact | Export, transit, and the 92-FZ exclusion depend on it | Article 164 point 1 |
| Relevant transport documents | The 92-FZ exclusion asks whether the person is named as a carrier; article 165 documents were not transcribed and remain required evidence | Article 164; article 165 still open |
| Service date and shipment or execution date | The 22 percent wording applies to supplies from 01.01.2026; the invoice date runs from shipment or from the advance | 425-FZ article 25 point 13; article 168 point 3 |
| Advance-payment fact | Changes the method to article 164 point 4 and requires line 5b on the later shipment invoice | Articles 164 and 168; decree 26 |
| Currency | Article 169 point 7 and the decree currency line | Article 169 |
| Tax-policy effective date | Rate wording, form wording, and indexed thresholds are date-dependent | 425-FZ and decree 26 |
| Exemption or zero-rate evidence | A zero-rate subpoint is conditional | Article 164 point 1 |
| Policy version | The server must be able to show which text it applied | Architecture |

Also retained from 0.1B: tenant, commercial relationship, execution reference, relationship side, exact-decimal monetary basis, and compilation context time. Quantity, unit, and price are inputs only when those indicators exist.

Outputs remain a treatment code or `UNRESOLVED`, an exact-decimal tax amount only when the treatment is resolved, the scale used, and the policy provenance. No output fills the identification number from `tax_id`.

`TAXDECISION_INPUT_REQUIREMENTS_UPDATED=YES`.

## 9. Rounding decision table

| Boundary | Rule | Status |
| --- | --- | --- |
| In-process money | Exact decimal. Unchanged from 0.1B | Architecture, not a tax statute |
| API money | Decimal string. Unchanged from 0.1B | Architecture, not a tax statute |
| Generic persistence | Half away from zero, scale 2, persistence boundary only. This is not a statutory VAT rule | 0.1B architecture |
| Budget or declaration tax amount | Article 52 point 6: full rubles, below 50 kopecks dropped, 50 kopecks and above rounded up | VERIFIED_PRIMARY_SOURCE |
| Invoice columns 4-6, 8 and 9 | Rubles and kopecks, or the subunits of the invoice currency | VERIFIED_PRIMARY_SOURCE |
| Tax presented to the buyer | Article 168 point 2: for each kind of supply, the percentage share of the price. No full-ruble step and no kopeck algorithm | VERIFIED_PRIMARY_SOURCE for the percentage rule |
| Line VAT rounding algorithm | Not identified in the reviewed primary sources | NEEDS_FURTHER_RESEARCH |
| Document VAT versus the sum of per-kind amounts | Article 168 point 2 states the per-kind computation. It does not say whether the document tax is the sum of those amounts or a second computation on a document base | NEEDS_FURTHER_RESEARCH |
| Sales-book label 18.03 percent | Decree 26 replaces the 16.67 percent book label. It is a book label, not a line algorithm | VERIFIED_PRIMARY_SOURCE as a label |
| Use of the full-ruble rule on an invoice line | Not adopted | The two norms were both read |

`TAX_ROUNDING_POLICY_FROZEN=NO` remains correct for the line algorithm.

## 10. Correction-document model

Issued financial facts stay immutable. A correction is a successor.

| Event | Successor | Identity | Original number and date | Quantity, price, tax | Semantics |
| --- | --- | --- | --- | --- | --- |
| Error that prevents identification of seller, buyer, name, cost, rate, or tax | New copy of the same invoice | Line 1a correction number and correction date | Line 1 unchanged | The new copy states the corrected indicators | Replacement copy, not a delta and not an in-place edit |
| Error that does not prevent that identification | No new invoice copy | None | Unchanged | Unchanged | Decree 1137 appendix 1 point 7; article 169 point 2 |
| Price and/or quantity change after the invoice | Correction invoice | Own line-1 number and date, shared chronology | Cited in line 1b and in article 169 point 5.2 | Lines before, after, increase, decrease | Delta against the predecessor. Issuance is within five calendar days of the article 172 point 10 documents |
| Buyer consent or notification | Primary document named by article 172 point 10 | Not an invoice number | It supports the correction invoice | It does not itself compute the delta | Condition of the deduction, and the date that starts the five-day period. Electronic format is Agent B |
| Same cost change recorded as UKD | Optional combined document | Same correction-invoice identity plus a primary document | Same predecessor reference | Same delta | Does not replace the decree 1137 document |
| Primary-document mistake that is not a VAT invoice | Correction under 402-FZ article 9 part 7 | Correction date and signatures on the primary document | Not a VAT line-1 rule | As the corrected fact states | Allowed only where no federal law requires another method |
| Electronic artifact | Agent B stores the new legal XML | Finance stores the accounting successor identity | Finance retains the predecessor reference | Finance supplies the monetary delta or replacement facts | Agent B does not calculate them |

## 11. Finance versus EDO

| Responsibility | Owner | This stage |
| --- | --- | --- |
| Monetary truth and exact-decimal amounts | Agent G | Confirmed |
| Accounting facts and primary-document requisite requirements | Agent G | Confirmed from 402-FZ |
| TaxDecision inputs and the unknown-tax gate | Agent G | Extended from the freight catalog, not implemented |
| VAT-invoice and correction-invoice accounting fields | Agent G | Article 169 plus decree 1137 as amended by decree 26 |
| Which accounting facts the electronic artifact must reproduce | Agent G | Parties, identification numbers, dates, line 5b, currency, amounts, treatment, predecessor, consent-or-notification fact |
| Legal XML, schemas, file generation | Agent B | Orders 970@ and ED-1-26/29@ exist; schema not copied here |
| EDO operator exchange, certificate verification, signature material, EDO state, artifact storage | Agent B | Not moved into Finance |
| Binding | Both | Finance cites the Agent B artifact. The artifact reproduces Finance amounts and does not recompute them |

`FINANCE_EDO_BOUNDARY_CONFIRMED=YES`.

## 12. Disposition of the previous unresolved questions

```text
UNRESOLVED_LEGAL_QUESTIONS=5
```

The count is the number of items that remain `STILL_OPEN`. Closed, deferred, and architecture items are listed and are not added to that count.

| Previous question | Disposition | Reason and source |
| --- | --- | --- |
| 1. Current article 169, including requisites and the signature sentence | CLOSED_PRIMARY_SOURCE | Consolidated article 169 read. 425-FZ points 10(a), 10(b), and 10(c), with force dates 01.01.2026, 01.01.2026, and 01.10.2027. Electronic invoice: enhanced qualified electronic signature, article 169 point 6 |
| 2. Current article 168, including the formula and "без НДС" | CLOSED_PRIMARY_SOURCE | Consolidated article 168 read. 425-FZ does not amend it. "Без налога (НДС)" is the article 145 inscription in point 5 |
| 3. Whether article 52 point 6 still has the full-ruble wording, and the boundary with invoice kopecks | CLOSED_PRIMARY_SOURCE | Current part-one article 52 point 6 still computes the tax amount in full rubles. Article 168 point 2 and decree 1137 keep invoice tax at a percentage share stated in kopecks. An explanatory FNS or Ministry of Finance letter was not retrieved; the two norms were |
| 4. Kopeck algorithm for one invoice line | STILL_OPEN | No reviewed primary source states a line-by-line kopeck algorithm. Absence is not converted into a Finance rule |
| 5. Document VAT versus the sum of line VAT | STILL_OPEN | Article 168 point 2 computes tax for each kind of supply. It does not state whether the document tax is the sum of those amounts |
| 6. Statutory VAT rates | CLOSED_PRIMARY_SOURCE | Article 164 points 1, 2, 3, 4 and 8 read, including the 425-FZ 22 percent application from 01.01.2026. The catalog is not a BINTRANS rate |
| 7. Freight, forwarding, and international transport | CLOSED_PRIMARY_SOURCE for the classes that were read | Articles 148, 149 carriage subpoints, and 164 point 1 were read. Application of a class to a shipment stays conditional. The article 165 document list is a separate still-open item below |
| 8. When an advance VAT invoice is required | CLOSED_PRIMARY_SOURCE | Article 168 points 1 and 3: on receipt of an advance toward supplies realised in the Russian Federation, present tax under article 164 point 4 and issue the invoice within five calendar days |
| 9. Whether buyer consent is a legal precondition | CLOSED_PRIMARY_SOURCE for the legal condition | Article 172 point 10 requires a contract, agreement, or other primary document confirming consent or notification, as the basis of the deduction and as the document named by article 168 point 3. The XML of that document is deferred to Agent B |
| 10. Field appendices of the 2013 and 2014 FNS letters | STILL_OPEN | The recommendation and the status split were read. The appendices were not transcribed. Statutory facts now come from article 169 and decree 1137, so the unread appendices are not the statutory requisite list |
| 11. XML dictionaries of orders ED-7-26/970@ and ED-1-26/29@ | DEFERRED_TO_AGENT_B | Titles and publication records were read. Schemas, generation, transport, and signature verification stay with Agent B |
| 12. Which signature satisfies an electronic VAT invoice | CLOSED_PRIMARY_SOURCE | Article 169 point 6: enhanced qualified electronic signature. The 402-FZ primary-document sentence remains a different, less specific rule |

Additional item that the rate catalog exposed:

| Question | Disposition | Reason |
| --- | --- | --- |
| Article 165 confirmation documents for a zero-rated transport operation | STILL_OPEN | Article 165 is the confirmation article. Its document list was not transcribed, so route facts alone do not close a zero-rate decision |
| Indexed 2026 ruble magnitudes for article 164 point 8 | STILL_OPEN | Point 9 requires indexation under article 346.12 point 2. The Code states 250 million and 450 million rubles. The act that sets the indexed magnitudes was not read |

Architecture decisions, not legal gaps:

- `GLOBAL_BINTRANS_VAT_RATE` stays unset. TaxDecision selects the treatment.
- `NO_GENERAL_MANDATORY_STATUTORY_FORM_IDENTIFIED_IN_REVIEWED_SOURCES` for a payment invoice. A contract may still require one.
- `402-FZ_ARTICLE_9_DOES_NOT_PRESCRIBE_A_UNIFIED_ACT_FORM`. The seven requisites remain.
- No browser field is tax truth.

## 13. Verified rule groups

```text
VERIFIED_PRIMARY_SOURCE_RULE_COUNT=17
```

1. 402-FZ article 9 primary requisites, and the absence of a unified act form in that article.
2. Decree 1137 invoice identity and parties.
3. Decree 1137 line and dash rules.
4. Decree 1137 kopeck cost columns.
5. Decree 1137 error-copy rule.
6. Decree 1137 correction-invoice delta.
7. Shared numbering chronology.
8. UPD recommendation and statuses, distinct from the electronic format order.
9. UKD recommendation, distinct from order ED-1-26/29@.
10. Article 52 point 6 full-ruble tax amount, re-read in the current part-one text, and kept off the invoice line.
11. Finance versus EDO ownership.
12. Article 164 rate catalog: 22 percent, 10 percent, 0 percent, the calculated method, and simplified-system 5 percent or 7 percent, with no BINTRANS rate.
13. Article 168 presentation duty, per-kind percentage, five-day invoice, advance method, and the article 145 inscription.
14. Article 169 requisites, the 2026 advance-invoice reference, the entrepreneur registration number, and the enhanced qualified electronic signature.
15. Decree 26 form, including line 5b and book columns 7a and 11a, effective 01.04.2026, kept separate from the Code.
16. Article 148 place of supply and the conditional article 164 transport and forwarding classes.
17. Article 172 point 10 consent or notification document as the deduction condition and the five-day trigger.

## 14. Recommendation for the next architecture review

Do not start implementation. Do not treat the 22 percent catalog fact as a runtime choice.

The next review can freeze only the closed boundaries: the payment-invoice wording above, the article 9 act rule, the article 169 and decree 26 invoice fields, the separation of budget rounding from invoice kopecks, the conditional freight classes, and the Agent B boundary. It cannot freeze a line-rounding algorithm, an article 165 evidence checklist, or the indexed simplified-system thresholds.

```text
RECOMMENDED_NEXT_ARCHITECTURE_STAGE=FINANCE-0.1D-VERIFIED-DOCUMENT-RULE-FREEZE
IMPLEMENTATION_AUTHORIZED=NO
MERGE_AUTHORIZED=NO
READY_FOR_FINANCE_0_1D_REVIEW=YES
```
