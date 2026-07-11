# Low-code Pilot Week-3 STG-LIM-003 Closure Candidate Note v0.1

## Summary

This note records whether STG-LIM-003 can be moved toward closure after post-panel verification.

## Limitation

STG-LIM-003:

```text
SSH 22 must be restricted by Selectel Security Group to trusted operator IP /32.
```

## Required Closure Evidence

To close STG-LIM-003, the following must be true:

* trusted operator SSH path passes
* TCP 22 is not open to 0.0.0.0/0 in Selectel Security Group
* TCP 22 is allowed only from trusted operator IP /32
* non-trusted SSH source is rejected or independent panel evidence confirms no public 22 exposure
* API health remains available
* runtime remains healthy
* no secrets captured
* production-ready not claimed

## Current Result

Trusted SSH path:

```text
pass
```

API health:

```text
pass — 200
```

Runtime health:

```text
pass — 10 containers healthy
```

UFW database/internal port denial:

```text
pass
```

Non-trusted rejection:

```text
not_available
```

Selectel SG /32 confirmation:

```text
unknown
```

## Closure Candidate Decision

```text
STG_LIM_003_REMAINS_OPEN
```

Reason:

```text
Selectel Security Group /32 restriction not independently confirmed; non-trusted SSH rejection test not available
```

## STG-LIM-003 Status

```text
OPEN_PENDING_NON_TRUSTED_REJECTION_TEST
```

## Production-ready

```text
not claimed
```

## Next Recommended Event

```text
non-trusted SSH rejection test or sanitized Selectel panel screenshot/evidence confirming SG rules — no operator IP in repo
```
