# Test Environment Matrix — Freight Platform v1

| Environment | Purpose | Data | Mutation | Network | Test types |
|-------------|---------|------|----------|---------|------------|
| **LOCAL** | Dev debugging | seed/demo | full | localhost | unit, manual smoke |
| **CI** | PR gate | ephemeral PG | isolated | GitHub runner | L0–L4, partial L5–L6 |
| **DISPOSABLE_INTEGRATION** | compose stack | migrate + smoke | full | docker network | L5–L7 smoke, golden skeleton |
| **STAGING** | pilot prep | bintrans seed | controlled | Selectel VPN/SSH | L6–L11, browser, mobile |
| **UAT** | business sign-off | staging copy | scenario | corp network | L12 MANUAL_UAT |
| **PILOT** | cohort rollout | production-like | limited | pilot tenants | L13 |
| **PRODUCTION** | live | real | **no destructive tests** | — | observability only |

## Current status

| Env | Ready |
|-----|-------|
| LOCAL | YES |
| CI | YES |
| DISPOSABLE | YES |
| STAGING | **NO** (F22R001–008, SSH blocked) |
| UAT | NO |
| PILOT | NO |

## Staging execution pack

When SSH restored:

```bash
make staging-acceptance-pack   # orchestrates preflight → health → E2E → report
```

Scripts: `scripts/test/staging-acceptance-pack.sh`

## Secrets

From env / `.env.example` only. Never hardcode in tests.
