# FINANCE-0.1A accounting document map

```text
LEGAL_COMPLIANCE_RESEARCH_REQUIRED=YES
MONETARY_TRUTH_OWNER=BILLING
LEGAL_XML_TRUTH_OWNER=EDO
```

Field classes are `PRESENT`, `DERIVABLE_FROM_CANONICAL_SOURCE`, `MISSING`, `NOT_APPLICABLE`, and `REQUIRES_LEGAL_POLICY`.

`PRESENT` means the accounting-document row stores the value. `DERIVABLE_FROM_CANONICAL_SOURCE` means another canonical row can supply it and this discovery found that source. A company UUID can be joined to `core.companies.legal_name` and `tax_id`. `tax_id` is not labeled ИНН. `core.companies` has no KPP column and no address column (`000002_create_core_tables.up.sql`). No later migration alters `core.companies`.

This map does not decide which fields a statute requires. Rows marked `REQUIRES_LEGAL_POLICY` are concepts the code either names without a rule or does not store, where a later legal pass has to decide whether they belong in scope.

Amounts on all four documents are copied from the billing register header in `closing_document_idempotent.go`. They are not entered as client totals on the actor path. Seller and buyer company IDs are overwritten from `contractor_company_id` and `customer_company_id`.

There is one invoice, one act, one VAT invoice, and one UPD per register (`find*ByRegisterTx` uses `LIMIT 1`). There are no goods or service lines on these tables.

## Shared field matrix

| Field | INVOICE | ACT | VAT_INVOICE | UPD |
| --- | --- | --- | --- | --- |
| Document number | PRESENT | PRESENT | PRESENT | PRESENT |
| Document date | PRESENT | PRESENT | PRESENT | PRESENT |
| Seller company | PRESENT | PRESENT | PRESENT | PRESENT |
| Buyer company | PRESENT | PRESENT | PRESENT | PRESENT |
| Currency | PRESENT | PRESENT | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE |
| Amount without VAT | MISSING | MISSING | PRESENT | PRESENT |
| VAT rate | MISSING | MISSING | PRESENT | PRESENT |
| VAT amount | MISSING | MISSING | PRESENT | PRESENT |
| Amount with VAT | PRESENT as `total_amount` | PRESENT as `total_amount` | PRESENT | PRESENT |
| Service / goods lines | MISSING | MISSING | MISSING | MISSING |
| Description | MISSING | MISSING | MISSING | MISSING |
| Quantity | MISSING | MISSING | MISSING | MISSING |
| Unit | MISSING | MISSING | MISSING | MISSING |
| Unit price | MISSING | MISSING | MISSING | MISSING |
| ИНН | MISSING | MISSING | MISSING | MISSING |
| КПП | MISSING | MISSING | MISSING | MISSING |
| Legal address | MISSING | MISSING | MISSING | MISSING |
| Consignor | MISSING | MISSING | MISSING | MISSING |
| Consignee | MISSING | MISSING | MISSING | MISSING |
| Contract / basis document | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE |
| Transport reference | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE |
| Settlement reference | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE | DERIVABLE_FROM_CANONICAL_SOURCE |
| Billing register reference | PRESENT | PRESENT | PRESENT | PRESENT |
| Payment document reference | MISSING | MISSING | MISSING | MISSING |
| Country / customs fields | REQUIRES_LEGAL_POLICY | REQUIRES_LEGAL_POLICY | REQUIRES_LEGAL_POLICY | REQUIRES_LEGAL_POLICY |
| Signatory | MISSING | MISSING | MISSING | MISSING |
| Function code | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | PRESENT |
| Correction relation | MISSING | MISSING | MISSING | MISSING |
| Format version | MISSING | MISSING | MISSING | MISSING |
| XML artifact | MISSING | MISSING | MISSING | MISSING |
| PDF artifact | MISSING | MISSING | MISSING | MISSING |
| EDO `document_id` | PRESENT as nullable column, never inserted | PRESENT as nullable column, never inserted | PRESENT as nullable column, never inserted | PRESENT as nullable column, never inserted |

Notes:

- Act `service_description` exists on the table. `ensureActTx` does not insert it, so a created act leaves it null. Class for description on ACT is `MISSING` in the created row. The column is not a line model.
- VAT invoice and UPD tables have no `currency_code`. The parent register currency is the canonical source.
- Register `contract_id` is optional. Register items can carry `shipment_id` and `transport_order_id`. Those are not columns on the four document tables. They are derivable through `register_id`.
- Consignor and consignee company IDs exist only as optional register-item columns, and settlement inclusion does not fill them.
- Company `tax_id` is a master-data string. It is not copied onto the document and it is not named ИНН. ИНН and КПП stay `MISSING` on the document. Whether `tax_id` may stand for ИНН is `REQUIRES_LEGAL_POLICY`.
- Country of the company is `country_code` on `core.companies`. Customs fields on the accounting document were not found. They stay `REQUIRES_LEGAL_POLICY` rather than a guessed mandatory set.
- UPD function codes in the check constraint are `СЧФ`, `СЧФДОП`, and `ДОП`. No format version accompanies them.
- Correction, UKD, and credit note relations are `MISSING`. ADR-EDO-002 mentions a future correction document. That ADR is not a procedure.

`billing.closing_document_packages` stores package number, type (`INVOICE_ONLY`, `ACT_PLUS_VAT_INVOICE`, `UPD`, `CUSTOM`), and status `DRAFT`. It does not store document ids. EDO package members are in the documents schema (`000087`) and are not written by billing.

Gateway `router.go` does not register invoice, act, VAT invoice, or UPD routes. OpenAPI `packages/openapi/billing-register-service.yaml` does. The service handlers exist.

## Billing to EDO

| Document | BILLING_RECORD_EXISTS | DOCUMENT_ID_SUPPORTED | DOCUMENT_ID_REQUIRED_BEFORE_OPERATOR_STATE | REAL_DOCUMENT_SERVICE_BINDING | REAL_OPERATOR_FLOW |
| --- | --- | --- | --- | --- | --- |
| Invoice | YES | YES | NO | NO | NO |
| Act | YES | YES | NO | NO | NO |
| VAT invoice | YES | YES | NO | NO | NO |
| UPD | YES | YES | NO | NO | NO |

`DOCUMENT_ID_SUPPORTED=YES` means the nullable UUID column exists. Inserts in `ensureInvoiceTx`, `ensureActTx`, `ensureVATInvoiceTx`, and `ensureUPDTx` do not set it. There is no foreign key to `documents.documents`. `document-service` has no references to `billing.upd_documents` or billing VAT amounts.

`MarkSentToEDO` changes register status when the current status is `CLOSING_DOCUMENTS_CREATED`. It does not read `document_id`. `MarkSigned` accepts `SENT_TO_EDO` or `CLOSING_DOCUMENTS_CREATED` and does not read a signature. Those transitions are manual register statuses.

Document-service types include the labels `INVOICE`, `VAT_INVOICE`, `ACT`, and `UPD`. Versions can store `payload_xml_path` and `pdf_file_path` supplied by the caller. No generator for a statutory XML file id was found. `UnavailableSignatureVerifier` returns `VERIFIER_UNAVAILABLE`. ADR-EDO-011 records `QUALIFIED_VALID_AVAILABLE_NOW=NO` and `QUALIFIED_DOCUMENT_TRUST_READY=NO`.

```text
REAL_BILLING_EDO_BINDING=NO
REAL_EDO_PROVIDER=NO
QUALIFIED_SIGNATURE_READY=NO
```

Billing does not store XML or signature blobs. That part of ADR-EDO-002 holds. The rule that `document_id` is mandatory before an operator-facing state does not hold in the billing status machine.
