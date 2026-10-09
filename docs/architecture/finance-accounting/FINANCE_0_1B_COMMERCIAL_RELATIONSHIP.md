# FINANCE-0.1B commercial relationship

```text
ONE_EXECUTION_CAN_HAVE_MULTIPLE_COMMERCIAL_RELATIONSHIPS=YES
FINANCIAL_PARTY_ROLE_IS_TRANSACTION_SCOPED=YES
FORWARDER_COMPANY_TYPE_DOES_NOT_FIX_FINANCIAL_DIRECTION=YES
SETTLEMENT_IDENTITY_INCLUDES_COMMERCIAL_RELATIONSHIP=YES
SHIPMENT_ID_UNIQUELY_IDENTIFIES_SETTLEMENT=NO
MULTIPLE_SETTLEMENTS_PER_EXECUTION_ALLOWED=YES
FORWARDER_TWO_SIDED_FINANCE_REQUIRED=YES
CUSTOMER_SETTLEMENT_SEPARATE_FROM_SUPPLY_SETTLEMENT=YES
CUSTOMER_BILLING_CHAIN_SEPARATE_FROM_SUPPLY_BILLING_CHAIN=YES
CUSTOMER_PAYMENT_CHAIN_SEPARATE_FROM_SUPPLY_PAYMENT_CHAIN=YES
CUSTOMER_EDO_CHAIN_SEPARATE_FROM_SUPPLY_EDO_CHAIN=YES
ONE_REGISTER_MAY_MIX_FORWARDER_CUSTOMER_AND_SUPPLY_CHAINS=NO
FINANCE_OWNS_EXECUTION_FACTS=NO
RATE_SNAPSHOT_RELATIONSHIP_MODEL=RELATIONSHIP_SCOPED_COMMERCIAL_PRICE_SNAPSHOT
FREIGHT_BASE_HISTORICAL_REPRICING_ALLOWED=NO
PAYMENT_OBLIGATION_IS_RECEIVABLE=NO
HISTORICAL_FORWARDER_RELATIONSHIPS_MAY_BE_INFERRED_WITHOUT_EVIDENCE=NO
```

No schema is created in this stage.

## Execution and relationship

A physical execution is not a commercial relationship. Agent C owns execution facts. Finance may reference `shipment_id`, `transport_order_id`, and completion evidence such as POD. Finance does not redefine those facts.

Until Agent C publishes a different execution identifier, the execution reference is the C-owned `shipment_id` together with `transport_order_id` when the shipment has one. That reference does not make the shipment the settlement identity.

```text
FINANCE_OWNS_EXECUTION_FACTS=NO
```

Cardinality is both of these:

- one execution, one commercial relationship
- one execution, two or more commercial relationships

A direct shipper-to-carrier move is valid with one relationship. A forwarder move is valid with two. The model does not require every execution to have two chains.

## CommercialRelationship

A commercial relationship is the agreement between two parties about one execution. A settlement belongs to exactly one relationship.

Required context:

| Context | Rule |
| --- | --- |
| Tenant | Server-verified tenant. Public `X-Tenant-ID` is not authority. |
| Execution reference | C-owned shipment and, when present, transport order. Not a finance-owned execution fact. |
| Customer party | The party that buys the service on this relationship. |
| Provider party | The party that provides the service on this relationship. |
| Relationship side | `DIRECT`, `CUSTOMER_SIDE`, or `SUPPLY_SIDE`. |
| Currency | One ISO 4217 code for the relationship. |
| Commercial basis | Pricing source and the immutable price snapshot for this relationship. |
| Lifecycle identity | The relationship id. Status may move. History is not deleted. |

`DIRECT` is shipper or customer as customer party and carrier as provider party. No forwarder is required.

`CUSTOMER_SIDE` is shipper or customer as customer party and forwarder as provider party. The financial reading is forwarder customer revenue and the future receivable side. This stage does not implement accounts receivable and does not define revenue recognition.

`SUPPLY_SIDE` is forwarder as customer party and carrier as provider party. The financial reading is forwarder carrier cost and the future payable side. This stage does not implement accounts payable.

```text
FINANCIAL_PARTY_ROLE_IS_TRANSACTION_SCOPED=YES
FORWARDER_COMPANY_TYPE_DOES_NOT_FIX_FINANCIAL_DIRECTION=YES
```

Company type `FORWARDER` does not mean buyer. Current gateway and billing actor derivation that maps `FORWARDER` to `BUYER` is a compatibility fact from FINANCE-0.1A. It is not the target model.

The acting company must be a party to the relationship and must hold the permission required for the command. Selecting a company is context. It is not authority by itself.

## Settlement identity

A settlement answers what is financially owed under this commercial relationship for this execution. It does not answer what the single financial record of a shipment is.

Target uniqueness: one non-cancelled settlement lineage for `(tenant, commercial_relationship, execution reference)`.

```text
SETTLEMENT_IDENTITY_INCLUDES_COMMERCIAL_RELATIONSHIP=YES
SHIPMENT_ID_UNIQUELY_IDENTIFIES_SETTLEMENT=NO
MULTIPLE_SETTLEMENTS_PER_EXECUTION_ALLOWED=YES
```

Several relationships on one execution produce several settlements. Two non-cancelled settlements for the same relationship and the same execution are duplicates.

Settlement correction and revision are not defined here. A later correction must be an explicit successor record. It must not rewrite the frozen settlement facts. The legal shape of that successor is a FINANCE-0.1C gap. This stage does not invent revision numbers, credit notes, or UKD.

## Forwarder chains

```text
FORWARDER_TWO_SIDED_FINANCE_REQUIRED=YES
CUSTOMER_SETTLEMENT_SEPARATE_FROM_SUPPLY_SETTLEMENT=YES
CUSTOMER_BILLING_CHAIN_SEPARATE_FROM_SUPPLY_BILLING_CHAIN=YES
CUSTOMER_PAYMENT_CHAIN_SEPARATE_FROM_SUPPLY_PAYMENT_CHAIN=YES
CUSTOMER_EDO_CHAIN_SEPARATE_FROM_SUPPLY_EDO_CHAIN=YES
```

Customer-side and supply-side settlements, registers, payment obligations, payments, and EDO document chains stay separate. One EDO package must not cover both relationships. Margin is not a formula in this stage.

## Billing register

A register is one coherent counterparty relationship for a period: one tenant, one customer party, one provider party, one relationship side, one currency, and one tax-eligibility gate. It includes only settlements of that same relationship side and those parties.

```text
ONE_REGISTER_MAY_MIX_FORWARDER_CUSTOMER_AND_SUPPLY_CHAINS=NO
```

## Price snapshot

```text
RATE_SNAPSHOT_RELATIONSHIP_MODEL=RELATIONSHIP_SCOPED_COMMERCIAL_PRICE_SNAPSHOT
FREIGHT_BASE_HISTORICAL_REPRICING_ALLOWED=NO
```

The chain is: commercial pricing decision, then an immutable commercial price snapshot for that relationship, then settlement. A later change to a contract rate, rate card, RFx price, or spot price does not alter the snapshot or the settlement principal.

`transport.transport_order_rate_snapshots` is one row per transport order for one buyer and one carrier. It can seed the commercial snapshot only for the single relationship whose parties and amount are that row. It cannot be the price of every relationship on the execution. A second relationship requires its own immutable snapshot. Reusing one transport-order snapshot for both forwarder sides is forbidden.

## Payment

A payment obligation remains payment-execution intent. It is not a receivable and it is not a payable account.

```text
PAYMENT_OBLIGATION_IS_RECEIVABLE=NO
```

Each obligation keeps payer, payee, commercial source, currency, and amount. Customer-side and supply-side obligations are reconciled independently. Accounts receivable, accounts payable, and factoring stay unimplemented.

## Idempotency identities

A retry must not create a second amount.

| Command | Identity |
| --- | --- |
| Create commercial relationship | Tenant, execution reference, customer party, provider party, relationship side |
| Create settlement | Tenant, relationship, execution reference, plus the caller idempotency key |
| Include settlement in a register | Tenant, register, settlement |
| Calculate or approve a register | Register and the expected version |
| Generate a closing document | Tenant, register, document type, until an explicit correction successor exists |
| Create payment | Tenant and idempotency key. A new payment number alone is not sufficient. |
| Allocate payment | Tenant, payment, obligation, idempotency key |
| Reconcile payment | Payment. A repeat returns the reconciled payment. |
| Manual adjustment | Tenant and idempotency key |

## Manual adjustment

```text
GENERIC_CLIENT_FINANCIAL_OVERRIDE=DENY
AUDITED_MANUAL_ADJUSTMENT_CONCEPT=YES
CLIENT_AUTHORED_BASE_AMOUNT=DENY
CLIENT_AUTHORED_EXTRA_CHARGES_TOTAL=DENY
CLIENT_AUTHORED_PENALTY_TOTAL=DENY
CLIENT_AUTHORED_VAT_RATE=DENY
CLIENT_AUTHORED_REGISTER_TOTAL=DENY
```

Normal billing input is an identity or selection. The server loads canonical amounts. The existing client line route that accepts `base_amount`, `extra_charges`, `penalties`, and `vat_rate` is outside the target model.

`AUDITED_MANUAL_ADJUSTMENT_CONCEPT=YES` as a separate aggregate, not as a free line total. A future implementation must require permission, reason code, comment or evidence, actor, timestamp, currency, signed amount and direction, idempotency, and audit history. It must not set a register total directly. This stage does not implement it.

## Security boundary

```text
PUBLIC_CLIENT_TENANT_HEADER_AUTHORITY=NO
DOWNSTREAM_FINANCE_SERVICE_TRUSTS_GATEWAY_TENANT_HEADER=YES
DIRECT_FINANCE_SERVICE_EXPOSURE_VERIFIED=NO
```

These markers stay as corrected in FINANCE-0.1A-R1. This stage does not change gateway code. FIN-SEC-001 stays with Agent A.
