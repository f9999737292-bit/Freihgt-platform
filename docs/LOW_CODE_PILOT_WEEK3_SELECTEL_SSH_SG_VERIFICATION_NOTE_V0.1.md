# Low-code Pilot Week-3 Selectel SSH SG Verification Note v0.1

## Summary

Verification note for STG-LIM-003.

Partial pass: trusted SSH and runtime healthy. Fail: Selectel SG not verified.

## Decision

```text
SELECTEL_SSH_SG_VERIFICATION_PARTIAL_SSH_TRUSTED_PASS_SG_PENDING
```

## Pass

* Operator SSH from trusted workstation
* API health 200
* UFW denies 5432/6379 on VM
* 10 runtime containers healthy

## Fail / Pending

* Selectel SG SSH /32 restriction not verified
* Non-trusted IP SSH rejection not tested
* STG-LIM-003 not closed

## Operator Action Required

1. Apply Selectel Security Group inbound rule: TCP 22 → trusted operator IP /32 only
2. Remove broad SSH allow in Selectel SG if present
3. Confirm operator SSH private key is used for staging access
4. Re-run verification after panel change

## Production-ready Status

```text
not claimed
```
