# Low-code Pilot Week-3 Selectel SSH SG Non-Trusted Rejection Note v0.1

## Summary

External non-trusted rejection test completed. Port 22 is publicly reachable. STG-LIM-003 remains open.

## Decision

```text
SELECTEL_SSH_SG_NON_TRUSTED_REJECTION_FAILED_PORT_22_PUBLICLY_OPEN
```

## Pass

* Trusted SSH path
* API health 200
* 10 runtime containers healthy
* Non-trusted external source available (check-host.net)

## Fail

* Non-trusted TCP 22 rejection — 5/5 international nodes connected
* Selectel SG /32 restriction not effective
* STG-LIM-003 not closed

## Blocker

```text
BLOCKED_WAITING_FOR_SELECTEL_SG_PANEL_CHANGE
```

## Operator Action

Apply Selectel Security Group SSH /32 rule manually, then re-run verification.

## Production-ready Status

```text
not claimed
```

## Next Pack

```text
Selectel SSH SG Post-Panel Re-Verification Pack v0.1
```
