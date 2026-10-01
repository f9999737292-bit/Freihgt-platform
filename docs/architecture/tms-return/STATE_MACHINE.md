# TMS-RETURN-0.1 disposition state machine

Shipment coarse status is not used.

```mermaid
stateDiagram-v2
  [*] --> DISPOSITION_PENDING: rejected quantity recorded
  DISPOSITION_PENDING --> DISPOSITION_PENDING: HOLD_PENDING_DISPOSITION
  DISPOSITION_PENDING --> IN_RETURN_TRANSIT: RETURN_TO_ORIGIN successor
  DISPOSITION_PENDING --> IN_REDIRECT_TRANSIT: REDIRECT successor
  IN_RETURN_TRANSIT --> RESOLVED: completed return action
  IN_REDIRECT_TRANSIT --> RESOLVED: completed redirect action
```

Hold does not create a revision.
Return and redirect cannot both become active for the same case.
A driver may record the rejection. A driver cannot authorize return or redirect.
Resolution requires a completed introduced action on the successor revision at the authorized canonical location.
