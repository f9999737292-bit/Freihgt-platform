# Live Data Demo Staging Presentation Talk Track v0.1

## Summary

Talk track for controlled staging demo.

## Business Narrative

```text
Bintrans is a logistics operating platform for shippers, carriers, forwarders, and finance teams.
The platform covers the flow from procurement request to execution, shipment status, documents, and billing.
```

## PLATFORM_ADMIN Talk Track

Show:

* dashboard;
* companies;
* overall platform visibility;
* optional low-code/admin area only if safe.

Say:

```text
The platform administrator has the broadest view. This role is used to manage organizations, monitor system readiness, and control demo-level setup.
```

Avoid:

* changing settings;
* creating real users;
* editing auth/security settings.

## SHIPPER_ADMIN Talk Track

Show:

* dashboard;
* companies;
* RFx / freight requests;
* transport orders;
* shipments.

Say:

```text
The shipper side starts with freight demand and procurement. A request can become an RFx/tender, then move into transport orders and shipment execution.
```

Avoid:

* creating new live tenders during the demo unless separately approved;
* sending external notifications.

## CARRIER_ADMIN Talk Track

Show:

* assigned or visible transport orders;
* shipments;
* execution statuses.

Say:

```text
The carrier side focuses on accepted work, shipment execution, status visibility, and operational coordination.
```

Avoid:

* changing shipment statuses unless separately approved.

## FINANCE_MANAGER Talk Track

Show:

* documents;
* billing registers.

Say:

```text
Finance sees the closing flow: documents and billing registers. This is where completed or billing-ready shipments can be grouped for financial processing.
```

Avoid:

* marking records paid/signed;
* creating production-like legal documents.

## Key Value Points

| Value               | Message                                             |
| ------------------- | --------------------------------------------------- |
| isolated staging    | demo is separated from production                   |
| synthetic data      | no real customer data                               |
| role workflow       | different users see business-relevant areas         |
| end-to-end chain    | procurement → order → shipment → document → billing |
| production boundary | production live-data demo remains separate approval |

## Decision

```text
LIVE_DATA_DEMO_STAGING_TALK_TRACK_CREATED
```
