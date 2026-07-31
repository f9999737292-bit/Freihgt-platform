# Live Data Demo Staging Presentation Route Order v0.1

## Summary

Recommended route order for controlled staging live-data demo.

## Route Order

### 1. Login

```text
https://staging.xn--80abvubqje.xn--p1ai/login
```

Purpose:

* show authenticated entry point;
* confirm staging URL;
* do not reveal password.

### 2. Dashboard

Purpose:

* show command-center concept;
* explain role-aware interface;
* confirm backend online state.

### 3. Companies

Purpose:

* show shipper/carrier structure;
* explain multi-tenant company model.

Talk track:

```text
Here we show the organization layer: shippers, carriers, and platform-level company data.
This is the basis for access control, contracts, tenders, and operational workflows.
```

### 4. RFx / Freight Requests

Purpose:

* show procurement/tender workflow;
* explain RFx as the starting point for transport procurement.

Talk track:

```text
The shipper can create a freight request or RFx event, invite carriers, compare responses, and move selected demand into transport execution.
```

### 5. Transport Orders

Purpose:

* show execution objects;
* explain order lifecycle.

Talk track:

```text
Transport orders represent operational execution after procurement. They connect demand, carrier assignment, shipment execution, and documents.
```

### 6. Shipments

Purpose:

* show tracking/execution layer;
* explain statuses.

Talk track:

```text
Shipments are the execution layer: planned, in progress, delivered, and billing-ready states.
```

### 7. Documents

Purpose:

* show document metadata;
* explain future ЭПД/ЭТрН/УПД expansion.

Talk track:

```text
Documents are connected to the shipment and billing lifecycle. In production roadmap this area expands to legally significant electronic documents.
```

### 8. Billing Registers

Purpose:

* show finance closing flow;
* explain register-based billing.

Talk track:

```text
Billing registers aggregate completed/billing-ready shipments and support invoice/UPD/closing document workflows.
```

## Decision

```text
LIVE_DATA_DEMO_STAGING_ROUTE_ORDER_DEFINED
```
