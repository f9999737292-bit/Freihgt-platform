# FINANCE-0.1A forwarder finance

```text
FORWARDER_TWO_SIDED_FINANCE_REQUIRED=YES
CUSTOMER_CHAIN_SEPARATE_FROM_SUPPLY_CHAIN=YES
FORWARDER_MARGIN_CANONICAL_FACT=NO
```

The required model is two commercial chains for one physical execution.

Customer side: shipper or customer to forwarder. That chain would carry customer revenue and a receivable.

Supply side: forwarder to carrier. That chain would carry carrier cost and a payable.

`FORWARDER_MANAGER` deriving to actor `BUYER` is not that model. `DeriveSettlementActorKind` maps company types `SHIPPER`, `FORWARDER`, and `LSP` to `BUYER`, and `CARRIER` to `CARRIER`. The forwarder is then one party on a single buyer-to-carrier settlement.

## What the schema allows

`billing.freight_settlements` has one `buyer_company_id`, one `carrier_company_id`, and unique `(tenant_id, shipment_id)` (`000042`). A second settlement for the same shipment fails or returns the existing row (`CreateSettlement` short-circuits on the existing shipment).

Register inclusion requires `settlement.buyer_company_id = register.customer_company_id` and `settlement.carrier_company_id = register.contractor_company_id`. One settlement can sit on one register. A second register for the other commercial pair has no second settlement to include.

Closing documents and the payment obligation follow that one register. EDO binding is absent for that single chain, so a second EDO chain is also absent.

`freight_cost.cost_entry` repeats the same buyer and carrier pair. Entry kinds cover planned, accrual, actual, billed, payable amount, and paid amount snapshots. There is no revenue entry kind and no margin entry kind. The ledger is append-only cost evidence for that pair. It is not an AR or AP subledger.

## Markers

```text
FORWARDER_CUSTOMER_AR_READY=NO
FORWARDER_SUPPLY_AP_READY=NO
ONE_EXECUTION_TWO_SETTLEMENTS_SUPPORTED=NO
ONE_EXECUTION_TWO_BILLING_CHAINS_SUPPORTED=NO
ONE_EXECUTION_TWO_EDO_CHAINS_SUPPORTED=NO
REVENUE_FACT_CANONICAL=NO
CARRIER_COST_FACT_CANONICAL=PARTIAL
ADDITIONAL_COST_FACT_CANONICAL=PARTIAL
FORWARDER_MARGIN_CANONICAL_FACT=NO
```

`CARRIER_COST_FACT_CANONICAL=PARTIAL` because the cost ledger stores buyer-to-carrier cost snapshots with `NUMERIC` amounts, and a single settlement stores the agreed principal plus approved accessorials. That is one commercial side. It is not a forwarder supply-side payable distinct from customer revenue.

`ADDITIONAL_COST_FACT_CANONICAL=PARTIAL` because approved settlement accessorials are server-owned `NUMERIC` rows and can enter the cost ledger. They are not classified as a margin input.

`REVENUE_FACT_CANONICAL=NO`. No revenue aggregate exists. Browser `revenueTotal` in `apps/web-admin/composables/useControlTower.ts` adds `total_with_vat` across the loaded register list and does not partition currency. That sum is not a canonical revenue fact and it is not margin.

No code implements `customer revenue - carrier payable - approved additional costs`. This discovery does not define that formula or a recognition policy.

```text
PAYMENT_OBLIGATION_IS_RECEIVABLE=NO
CANONICAL_RECEIVABLE_IMPLEMENTED=NO
CANONICAL_PAYABLE_IMPLEMENTED=NO
```

A payment obligation is the payment-execution intent for one billing register (ADR-EDO-005). It is not a customer receivable and it is not a carrier payable account.
