# FINANCE-0.1D Accounting document rule freeze

```text
STAGE=FINANCE-0.1D-ACCOUNTING-DOCUMENT-RULE-FREEZE
KIND=ARCHITECTURE_AND_LEGAL_RULE_FREEZE
BASE_SHA=86a6b7b2eaeac3da1f1bc63a046c796eb49d5582
RESEARCH_RECORD=FINANCE-0.1C-R1
RESEARCH_ACCESS_DATE=2026-10-13
FREEZE_DATE=2026-10-14
PRODUCT_IMPLEMENTATION_AUTHORIZED=NO
MIGRATION_AUTHORIZED=NO
STAGING_AUTHORIZED=NO
GLOBAL_BINTRANS_VAT_RATE=NOT_SET
STANDARD_2026_VAT_RATE=22_PERCENT
STANDARD_RATE_IS_GLOBAL_DEFAULT=NO
INTERNATIONAL_ZERO_RATE=CONDITIONAL
INTERNATIONAL_TRANSPORT_ALWAYS_ZERO_RATE=NO
LINE_VAT_ROUNDING_ALGORITHM=NOT_STATUTORILY_IDENTIFIED_IN_REVIEWED_PRIMARY_SOURCES
XML_OWNER=AGENT_B
SIGNATURE_VERIFICATION_OWNER=AGENT_B
CANONICAL_MONEY_TYPE=EXACT_DECIMAL
```

This freeze turns the 0.1C primary-source research into a decision contract. It states which accounting document is required, from which facts, who issues it, to whom, and which VAT treatment may be stated. It does not implement a service, a table, an API, a migration, or a tax calculation.

0.1B remains in force. Unknown tax does not become zero, does not take a register rate, and blocks accounting close and tax-bearing documents. A client or browser field is not tax truth. A generic `tax_id` is not an identification number. The two forwarder chains do not share one decision. Exact decimal remains the internal money type. Persistence scale 2 is not a statutory VAT rounding rule.

Legal claims below restate texts read on 2026-10-13. The source register identifies the act, the provision, and the effective date. Where that reading cannot establish a rule, the question stays open.

## 1. Decision result

A future server-owned result is `ACCOUNTING_DOCUMENT_POLICY_DECISION`. This stage defines it. It does not create an API or a table.

| Field | Meaning |
| --- | --- |
| `transaction_id` | The commercial relationship and execution the decision is about. One shipment does not imply one decision. |
| `decision_version` | The policy version that produced the result. |
| `required_document_types` | Documents the facts require. Empty when the decision is not known. |
| `optional_document_types` | Documents the facts allow and do not require. |
| `vat_treatment` | One catalog class, or `UNKNOWN`. |
| `vat_rate_if_known` | A rate only when `vat_treatment` is known and that class has a rate. Absent when unknown, exempt, or outside Russian VAT. |
| `issuer` | The party that must issue each required document. |
| `recipient` | The party to whom it is issued. |
| `effective_legal_date` | The service, shipment, advance, or correction date that selects the rule. |
| `source_rule_identifiers` | The rule ids in this freeze. |
| `decision_state` | `KNOWN`, `UNKNOWN`, or `NEEDS_REVIEW`. |

`KNOWN` means the facts select one statutory class and every evidence item that class requires has been read and is present.

`UNKNOWN` means a fact required to choose a class is missing. The system does not guess.

`NEEDS_REVIEW` means the facts identify a conditional class, and the confirming evidence is either absent or still an open legal checklist. Zero-rate eligibility in that state is `NOT_PROVEN`.

`VAT_DECISION=UNKNOWN` whenever the rate class cannot be selected. A missing treatment is not 22 percent and not 0 percent.

## 2. Authoritative facts

The server owns these facts. A browser value may be captured as a claim. It becomes a fact only after the server accepts it from a system of record.

| Fact | What the decision needs | If absent |
| --- | --- | --- |
| `LEGAL_ENTITY_ROLE` | Which side of which chain is deciding: forwarder toward customer, or supplier toward forwarder. | `UNKNOWN`. Do not copy the other chain. |
| `SELLER_ENTITY` | Legal name, address, taxpayer identification number, and registration-reason code. For an entrepreneur, the primary state registration number and its assignment date. | VAT invoice parties are incomplete. A tax-bearing invoice is not `KNOWN`. |
| `BUYER_ENTITY` | The same identifiers for the buyer on that chain. | Same block. |
| `SERVICE_TYPE` | Goods, work, or service. Freight carriage and freight forwarding are services, not the article 164 point 2 goods list. | Rate class `UNKNOWN`. |
| `TRANSPORT_ROLE` | Carrier, forwarder, or another performer, and whether that person is named as a carrier in the transport documents. | International and 92-FZ tests stay `NOT_PROVEN`. |
| `PLACE_OF_SUPPLY` | Russian Federation, or not, under article 148. | No Russian VAT rate is selected. |
| `ROUTE_COUNTRIES` | Country of departure and country of destination. | Domestic and international classes stay `UNKNOWN`. |
| `DOMESTIC_OR_INTERNATIONAL` | Derived only when both route ends are known. It is not a client switch. | `UNKNOWN`. |
| `CONTRACT_TYPE` | Carriage, freight forwarding, or another contract. | Forwarding zero-rate class stays `NOT_PROVEN`. |
| `SERVICE_COMPLETION_DATE` | Performance or shipment date. The 22 percent wording applies to supplies from 01.01.2026. | Rate version `UNKNOWN`. |
| `DOCUMENT_EVENT_DATE` | Invoice compilation date, advance receipt date, or correction-document date. | Timing rule cannot be applied. |
| `TAX_REGIME` | General regime, simplified system, agricultural regime, or another regime the server has recorded. | `UNKNOWN`. A BINTRANS carrier is not a regime. |
| `VAT_STATUS` | Whether the seller must charge VAT, is released under article 145, or is outside the charge. | Invoice duty and rate stay `UNKNOWN`. |
| `VAT_RATE_DECISION` | Output of this policy. Never an input from the client or the register. | The output is `UNKNOWN`. |
| `CORRECTION_PRESENT` | Whether the event is an error, a price or quantity change, or neither. | No correction document is selected. |
| `ORIGINAL_DOCUMENT_REFERENCE` | Number and date of the invoice being corrected or of the advance invoice being offset. | Correction and line 5b stay incomplete. |

Also required when the event has them: advance-payment fact and amount, document currency, mode of transport, customs transit or export fact, the seller's election under article 164 point 7, and the zero-rate evidence set. Absence of an optional commercial fact is not zero quantity and not a zero rate.

## 3. VAT decision

```text
GLOBAL_BINTRANS_VAT_RATE=NOT_SET
STANDARD_2026_VAT_RATE=22_PERCENT
STANDARD_RATE_IS_GLOBAL_DEFAULT=NO
THIS_IS_NOT_A_GLOBAL_BINTRANS_DEFAULT=YES
INTERNATIONAL_ZERO_RATE=CONDITIONAL
INTERNATIONAL_TRANSPORT_ALWAYS_ZERO_RATE=NO
```

The catalog is the one read in article 164. TaxDecision selects a class from facts.

| Class | Statutory position | When the decision may be `KNOWN` | Otherwise |
| --- | --- | --- | --- |
| `STANDARD_RATE` | Article 164 point 3: 22 percent for cases not stated in points 1, 2, and 4, for supplies from 01.01.2026 | Place of supply is the Russian Federation, the operation is not a point 1 case, not a point 2 listed good, not a point 4 calculated-method case, the seller is not on point 8 and not released from VAT duties, and the service date is on or after 01.01.2026 | `UNKNOWN` if any of those facts is missing. Do not use 22 percent as a fallback. |
| `REDUCED_RATE` | Article 164 point 2: 10 percent for the listed goods | The supply is one of those goods | Freight carriage and forwarding are not this class on the text that was read. |
| `ZERO_RATE` | Article 164 point 1, including international carriage and the listed forwarding and export-leg services | Only when the subpoint's own conditions and its confirmation evidence are both present. See section 4. | `NOT_PROVEN` or `UNKNOWN`. Never automatic. |
| `SPECIAL_USN_RATE` | Article 164 point 8: 5 percent or 7 percent for a simplified-system seller who meets the stated conditions | Seller regime, the applicable subpoint, and the income test are all known, including the indexed threshold | `UNKNOWN`. The indexed 2026 ruble magnitudes were not read. |
| `CALCULATED_METHOD` | Article 164 point 4 | An advance, or another point 4 case, whose underlying class is `KNOWN` | If the underlying class is unknown, this method is `UNKNOWN`. |
| `EXEMPT` | Article 149, for the passenger services that were located | The passenger, tariff, and contract facts of those subpoints are present | Not a goods-freight result. Do not mark BINTRANS freight exempt from those subpoints. |
| `NOT_TAXABLE` | Article 148 point 1.1 subpoint 5: the carriage or connected service is not supplied in the Russian Federation | The place-of-supply facts positively exclude subpoints 4.1 through 4.3 | This is not a zero rate. |
| `UNKNOWN` | No class selected | Used when a required fact is missing | No rate, no exemption, and no zero. |

A seller released under article 145 point 1 who still issues an invoice states no tax amount and uses the inscription "Без налога (НДС)". A simplified-system or agricultural seller who is released from VAT duties does not issue an invoice. Those are document consequences of `VAT_STATUS`, not a freight rate.

## 4. International and domestic carriage

`INTERNATIONAL_ZERO_RATE=CONDITIONAL`.

Article 164 point 1 subpoint 2.1 defines international carriage of goods as carriage by sea, river, mixed river-sea, air, rail, or road where the point of departure or the point of destination is outside the Russian Federation. The same subpoint can include listed transport-forwarding services under a freight-forwarding contract that organises that carriage. The consolidated text excludes a person who only carries between points in the Russian Federation and is not named as a carrier in the transport or shipping documents that confirm export or import. Article 164 point 7 allows the taxpayer to elect out of the zero rate for the listed operations. Article 165 is the confirmation article. Its document list was not transcribed.

Evidence required before zero-rate eligibility may be decided:

| Evidence | Why it is required | If absent |
| --- | --- | --- |
| Both route ends | The definition uses departure or destination outside the Russian Federation | `ZERO_RATE_ELIGIBILITY=UNKNOWN` |
| Mode of transport | The definition lists the modes | `UNKNOWN` |
| Contract type and service kind | Forwarding is inside the subpoint only for the listed services under a forwarding contract | `NOT_PROVEN` |
| Named-carrier fact | The exclusion depends on it | `NOT_PROVEN` |
| Seller has not elected out under article 164 point 7 | The zero rate is then not applied | `UNKNOWN` |
| Article 165 confirmation documents | The statute requires them and the list is still open | `ZERO_RATE_ELIGIBILITY=NOT_PROVEN` |

A domestic carriage, both ends in the Russian Federation, is not subpoint 2.1 unless another zero-rate subpoint fits, such as the export-related rail, inland-water, or sea legs that were read. Those subpoints stay conditional on their own facts. If the facts do not positively exclude every zero-rate subpoint, the decision is not silently `STANDARD_RATE`.

Place of supply is decided first. Article 148 point 1 subpoint 4.1 places carriage in the Russian Federation when a Russian organisation or entrepreneur performs it and at least one end is in the Russian Federation, or when an unregistered foreign person performs it and both ends are in the Russian Federation. If the service is not supplied in the Russian Federation, the result is `NOT_TAXABLE` for this chapter, not 0 percent.

## 5. Document decision matrix

Not every shipment requires every accounting document. A document is required only for the event and the facts named here.

| Event | Required documents | Optional documents | Issuer | Recipient | VAT fields | Correction document | Rule | Confidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Commercial request before performance, no taxable supply and no recorded fact | None from the statutes read | Payment invoice, if the contract requires one | The party the contract names | The party the contract names | None. It is not a VAT invoice. | No | `R-PAY` | Source-bounded. No general statutory form was found. |
| Completed freight service, VAT class `KNOWN` and an invoice is required | Primary document of the fact, and a VAT invoice. One UPD status 1 may carry both. | Payment invoice | Seller of that chain issues the VAT invoice. The compiling entity issues the primary document. | Buyer of that chain | Rate, tax amount, parties, currency, and line 5b when an advance is offset | No | `R-ACT`, `R-SF`, `R-UPD1` | Primary sources read. |
| Completed freight service, `VAT_DECISION=UNKNOWN` | No tax-bearing VAT invoice and no tax-bearing UPD status 1 | A primary document of the commercial fact, without a VAT rate | Compiling entity | Counterparty of the fact | Must not state a rate or a tax amount | No | `R-BLOCK` | 0.1B gate plus unread rate. |
| Advance received for a supply realised in the Russian Federation, underlying class `KNOWN` | Advance VAT invoice within five calendar days of receipt | None | Seller | Buyer | Calculated method of the underlying rate | No | `R-ADV` | Article 168 points 1 and 3; article 164 point 4. |
| Advance received, underlying class not `KNOWN` | No tax-bearing advance invoice | None | — | — | None | No | `R-BLOCK` | Do not invent the underlying rate. |
| Later shipment against that advance, class `KNOWN` | Shipment VAT invoice with line 5b citing the advance invoice | UPD status 1 | Seller | Buyer | Shipment treatment, not a second guess | No | `R-5B` | Code from 01.01.2026. Form line 5b from 01.04.2026. |
| International or export-leg candidate, definition facts incomplete | No zero-rated invoice | Primary document without a VAT rate | Compiling entity | Counterparty | Zero-rate fields forbidden | No | `R-ZERO` | `ZERO_RATE_ELIGIBILITY=UNKNOWN` |
| Definition facts present, article 165 evidence absent or not yet a closed list | No document that states 0 percent as a known treatment | Primary document without a VAT rate | Compiling entity | Counterparty | Zero-rate fields forbidden until proven | No | `R-ZERO` | `ZERO_RATE_ELIGIBILITY=NOT_PROVEN`. `decision_state=NEEDS_REVIEW`. |
| Service not supplied in the Russian Federation | Primary document if a fact must be recorded. No Russian VAT invoice that states a Russian rate. | Payment invoice | Compiling entity | Counterparty | No Russian VAT rate | No | `R-POS` | Article 148. Not a zero rate. |
| Seller released from VAT duties and not required to issue an invoice | Primary document if a fact must be recorded | UPD status 2 | Compiling entity | Counterparty | No invoice tax | No | `R-REL` | Article 168 point 5. |
| Price or quantity change after a VAT invoice | Correction VAT invoice, plus a consent or notification document | UKD as the combined form | Seller | Buyer | Before, after, and the difference. Predecessor number and date. | Yes | `R-KSF`, `R-CONSENT` | Articles 168, 169, and 172. |
| Error that prevents identification of seller, buyer, name, cost, rate, or tax | Replacement copy. Line 1 unchanged. Line 1a holds the correction number and date. | None | Seller | Buyer | Corrected indicators | The copy is the correction | `R-FIX` | Decree 1137 and article 169 point 2. |
| Error that does not prevent that identification | None | None | — | — | Unchanged | No | `R-FIX` | No new invoice. |
| Passenger service matching a located article 149 exemption | Not a BINTRANS goods-freight row | — | — | — | — | — | `R-EX` | `NOT_APPLICABLE` to carriage of goods. |

`R-PAY`. The sources read do not identify a general mandatory statutory form for a payment invoice. A contract may still require one. It is not the VAT invoice and it is not, by itself, the article 9 primary document.

`R-ACT`. 402-FZ article 9 does not prescribe a unified act form. An act is required when the entity records a fact of economic life. The seven requisites are the document name, compilation date, name of the compiling entity, content of the fact, natural and/or monetary measure with units, position of the responsible person, and signatures with identifying requisites. The entity approves the form. Quantity may be absent when the service has no quantity. The compilation date is the document date.

`R-SF`. The seller issues a VAT invoice no later than five calendar days from shipment, performance, or transfer, and from receipt of an advance toward a supply realised in the Russian Federation. Article 169 states the requisites. Decree 1137, as amended by decree 26, states the lines. From 01.04.2026 the form includes line 5b. An electronic invoice is signed with an enhanced qualified electronic signature. The signature check itself belongs to Agent B.

`R-BLOCK`. Unknown tax blocks a tax-bearing document and an accounting close. It does not block the later recording of a commercial fact that states no tax.

`R-UPD1`. UPD is optional. Status 1 may be used for a completed supply that needs both a VAT invoice and a primary transfer document. It satisfies both regimes or it is not status 1. It does not create a third, weaker document.

`R-UPD2`. Status 2 may be used as a primary document only. It must not be treated as a VAT invoice and must not carry a VAT rate as if it were one.

`R-UKD`. UKD is optional. It may record a correction invoice together with a primary document of the cost change. Refusal to use it does not, on the 2014 letter, deny the cost change. The taxpayer may use the decree 1137 correction invoice and a separate primary document.

`R-KSF`. The correction invoice has its own number and date, cites the predecessor, and states before, after, and the difference. It is issued within five calendar days of the documents named in article 172 point 10.

`R-CONSENT`. Article 172 point 10 requires a contract, agreement, or other primary document confirming the buyer's consent, or the fact of notification, as a condition of the deduction and as the document that starts that five-day period. The electronic format of that document belongs to Agent B.

`R-ZERO`. Zero rate is not encoded as a property of international transport. Missing evidence stays `UNKNOWN` or `NOT_PROVEN`.

## 6. UPD and UKD

| Question | Frozen rule |
| --- | --- |
| Which event can use UPD | A completed supply or transfer that needs a primary document. Status 1 also needs a VAT invoice duty that is `KNOWN`. Status 2 is the primary document alone. |
| When UPD replaces separate documents | Status 1 may be used instead of a separate act and a separate VAT invoice for the same event. It does not remove either set of requisites. |
| Status meaning | Status 1 is invoice plus transfer. Status 2 is primary only. |
| Correction relationship | A later price or quantity change does not rewrite the UPD in place. The correction path is a correction invoice, optionally carried as UKD. |
| Original reference | The correction document cites the original invoice number and date. An error copy keeps that number and date and adds line 1a. |
| XML and signature | Not frozen here. Agent B owns schemas, files, signature verification, storage, and EDO transport. |

The 2013 and 2014 letter appendices were not transcribed. That remains open. The accounting facts Agent B must reproduce are the article 169 requisites, the decree 1137 lines, and the 402-FZ requisites, not an unread appendix.

## 7. Rounding and money

```text
BUDGET_TAX_ROUNDING_RULE=ARTICLE_52_POINT_6_FULL_RUBLE
INVOICE_LINE_VAT_ROUNDING_RULE=NOT_THE_FULL_RUBLE_RULE
LINE_VAT_ROUNDING_ALGORITHM=NOT_STATUTORILY_IDENTIFIED_IN_REVIEWED_PRIMARY_SOURCES
LINE_VAT_ROUNDING_OPEN=YES
```

Article 52 point 6 computes the tax amount in full rubles. Invoice columns are in rubles and kopecks, or in the subunits of another invoice currency. Article 168 point 2 computes the tax presented to the buyer as a percentage share of the price for each kind of supply and states no kopeck algorithm. This freeze does not invent one and does not adopt the 0.1B persistence rounding as a statutory rule.

A later stage may choose a product calculation policy. If it does, that policy must be labeled as a product policy. It must not be described as article 52 or as a statutory line algorithm.

Document VAT versus the sum of the per-kind amounts is open. Article 168 point 2 does not state that identity.

| Amount | Rule |
| --- | --- |
| `DOCUMENT_CURRENCY` | The currency named on the document. Article 169 point 7 allows the invoice amounts to be stated in foreign currency when the obligation is in foreign currency. |
| `SETTLEMENT_CURRENCY` | The commercial relationship's settlement currency. It is not silently replaced by the tax currency. |
| `TAX_REPORTING_AMOUNT` | The budget tax amount follows article 52 point 6 when the tax is computed in rubles. Conversion from a foreign-currency invoice into that ruble amount was not frozen in the texts used here. If the invoice currency is not the ruble, the conversion rule stays open and the tax-reporting amount is `UNKNOWN`. |

Internal storage remains exact decimal until a named boundary. The invoice scale, once a product policy exists, is kopecks or the foreign currency's subunits. The budget boundary is full rubles. Those boundaries are not the same step.

## 8. Agent B boundary

| Concern | Owner |
| --- | --- |
| Which document is required, the VAT class, the issuer, the recipient, and the monetary facts | Agent G |
| Canonical document instance, XML, schema, file generation | Agent B |
| Signature verification, certificate material, EDO transport, operator exchange, storage, EDO state | Agent B |
| Binding | Finance cites the Agent B artifact. The artifact reproduces Finance amounts and the decided treatment. It does not recompute them. |

The electronic VAT invoice uses an enhanced qualified electronic signature under article 169 point 6. Checking that signature is Agent B's work. 402-FZ article 9 part 5 requires an electronic signature on an electronic primary document and does not, in the sentence read, say "qualified". The two sentences stay separate.

## 9. Open questions

| Item | State | Why it is not closed |
| --- | --- | --- |
| 1. Statutory invoice-line VAT rounding algorithm | `OPEN` | No reviewed primary source states a line-by-line kopeck algorithm. |
| 2. Document-level VAT aggregation rule | `OPEN` | Article 168 point 2 states a per-kind percentage. It does not state whether the document tax is the sum of those amounts. |
| 3. Article 165 evidence set for the supported international and export-leg scenarios | `OPEN` | The confirmation article was identified and its document list was not transcribed. Zero-rate eligibility stays `NOT_PROVEN` without it. |
| 4. Indexed 2026 simplified-system income thresholds | `OPEN` | Article 164 point 9 requires indexation. The Code states 250 million and 450 million rubles. The act that sets the indexed magnitudes was not read. |
| 5. Historical and current UPD and UKD format appendices | `OPEN` | The letters' recommendation and status split were read. The appendices were not transcribed. Statutory requisites do not depend on those appendices. |

No silent assumption closes any of these five.

## 10. Source register

Access date for every primary text below is 2026-10-13, the date the text was read in FINANCE-0.1C-R1. This freeze does not replace that reading with commentary.

| Rule | Source | Number | Provision | Effective date |
| --- | --- | --- | --- | --- |
| `R-ACT` | Federal Law on accounting | 402-FZ | Article 9 | Text read includes amendments through 26.07.2019 No. 247-FZ |
| `R-SF`, `R-FIX` | Tax Code part two; Government decree 1137 | 117-FZ; 1137 | Article 169; decree appendices 1 and 2 | Consolidated code list through 04.08.2026 No. 292-FZ. Decree 26 form from 01.04.2026 |
| `R-ADV`, `R-5B` | Tax Code articles 168 and 164; decree 26 | 425-FZ; 26 | Article 168 points 1 and 3; article 164 point 4; article 169 point 5 subpoint 4; invoice line 5b | Code advance reference from 01.01.2026. Form line from 01.04.2026 |
| `STANDARD_RATE` | Tax Code article 164; Federal Law 425-FZ | 425-FZ | Article 164 point 3; 425-FZ article 25 point 13 | Supplies from 01.01.2026 |
| `ZERO_RATE`, `R-ZERO` | Tax Code articles 148, 164, and 165 | 117-FZ | Article 148; article 164 point 1 | Consolidated text. Article 165 list not transcribed |
| `SPECIAL_USN_RATE` | Tax Code article 164 | 117-FZ | Points 8 and 9 | Consolidated text. Indexed magnitudes not read |
| `R-CONSENT` | Tax Code article 172 | 117-FZ | Point 10 | Consolidated text |
| Budget rounding | Tax Code part one | 146-FZ | Article 52 point 6 | Edition dated 01.10.2026 |
| `R-UPD1`, `R-UPD2` | FNS letter | MMV-20-3/96@ | Recommendation and statuses | 21.10.2013. Appendix not transcribed |
| `R-UKD` | FNS letter | MMV-20-15/86@ | Recommendation | 17.10.2014. Appendix not transcribed |
| Electronic formats | FNS orders | ED-7-26/970@; ED-1-26/29@ | Titles and publication records | 19.12.2023; 19.01.2026. XML not read |

Primary URLs are recorded in `FINANCE_0_1C_RUSSIAN_ACCOUNTING_DOCUMENT_RESEARCH.md`. A commercial database, a vendor page, a blog, or a forum is not a source of these rules.

## 11. What this stage does not do

```text
PRODUCT_IMPLEMENTATION_AUTHORIZED=NO
VAT_RATE_RUNTIME_IMPLEMENTED=NO
ACCOUNTING_DOCUMENT_POLICY_DECISION_IMPLEMENTED=NO
RECOMMENDED_NEXT_STEP=CONTROLLER_REVIEW
```

The next implementation stage is not authorized by this freeze. A controller review comes first.
