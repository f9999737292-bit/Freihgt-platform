# Low-code Pilot Week-3 Selectel SSH SG Post-Panel Re-Verification Note v0.1

## Summary

Operator reported SG change done. Re-verification shows port 22 still publicly open.

## Decision

```text
SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_FAILED_PORT_22_STILL_PUBLIC
```

## Pass

* Trusted SSH
* API 200
* 10 containers healthy

## Fail

* Non-trusted TCP 22 — 5/5 external nodes connect
* Selectel SG /32 not evidenced
* STG-LIM-003 not closed

## Blocker

```text
BLOCKED_WAITING_FOR_SELECTEL_SG_PANEL_CHANGE
```

## Production-ready

```text
not claimed
```
