# Live Data Demo Workflow Staging Limitations v0.1

## Summary

Limitations for staging live-data demo workflow smoke.

## Always True

```text
Production live-data demo is not approved.
Production writes are not approved.
Production credentials are not approved.
```

## Limitations To Record

| Limitation                                      | Status       |
| ----------------------------------------------- | ------------ |
| staging-only signoff                            | yes          |
| production live-data demo                       | not approved |
| real customer data                              | not used     |
| legal/billing real documents                    | not used     |
| external notifications                          | not used     |
| passwords/tokens in evidence                    | forbidden    |
| full production readiness                       | not claimed  |
| staging AUTH_ENABLED=false during smoke         | yes          |
| role-based API denial not validated             | yes          |
| interactive browser UI nav not fully captured   | yes          |
| logout/session cleanup not automated in smoke     | yes          |

## Demo Wording

```text
This is a staging authenticated live-data demo workflow using synthetic DEMO data.
It does not approve production live-data demo or real operations.
```
