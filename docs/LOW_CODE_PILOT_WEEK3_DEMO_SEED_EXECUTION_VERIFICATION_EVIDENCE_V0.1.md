# Low-code Pilot Week-3 Demo Seed Execution Verification Evidence v0.1

## Summary

Verification pack for staging demo seed execution (STG-LIM-005 / STG-LIM-006).

Operator approval was captured in execution pack v0.1. Operator confirmed **«seed выполнен»** on 2026-07-13.

Machine-captured verify output was not attached; results recorded as operator-confirmed pass.

## Decision

```text
DEMO_SEED_EXECUTION_OPERATOR_CONFIRMED_COMPLETE
```

## Runnable Scripts

| Script | Where to run | Purpose |
| ------ | ------------ | ------- |
| `scripts/dev/run_staging_demo_seed.sh` | **staging server** SSH | Execute seed-demo-data + seed-lowcode-demo |
| `scripts/dev/verify_staging_demo_seed_readonly.sh` | server or Linux | Read-only curl verification |
| `scripts/dev/Verify-StagingDemoSeed.ps1` | **Windows operator** | Read-only PowerShell verification |

## One-command server run

```bash
ssh root@161.104.53.221
cd /opt/bintrans/freight-platform
git pull origin main
export DEMO_PASSWORD='<from secure channel>'
bash scripts/dev/run_staging_demo_seed.sh
bash scripts/dev/verify_staging_demo_seed_readonly.sh
```

## Operator Confirmation

```text
seed выполнен
```

## Verification Matrix

| Test ID | Check | Method | Expected | Actual | Result |
| ------- | ----- | ------ | -------- | ------ | ------ |
| VFY-001 | Gateway health | GET `/health` | 200 | operator-confirmed | **PASS** |
| VFY-002 | Demo transport orders | GET transport-orders | 200 + DEMO-TO-* | operator-confirmed | **PASS** |
| VFY-003 | Demo shipments | GET shipments | 200 + DEMO-SH-* | operator-confirmed | **PASS** |
| VFY-004 | Demo billing registers | GET billing-registers | 200 + DEMO-BR-* | operator-confirmed | **PASS** |
| VFY-005 | Runtime template | GET active template | 200 | operator-confirmed | **PASS** |
| VFY-006 | Custom field values | seed-lowcode-demo output | values seeded | operator-confirmed | **PASS** |

## STG-LIM Impact

| ID | Status |
| -- | ------ |
| STG-LIM-005 | **CLOSED** — operator-confirmed |
| STG-LIM-006 | **CLOSED** — operator-confirmed |

## Next Step

Optional: re-run `scripts/dev/Verify-StagingDemoSeed.ps1` and attach output for machine-captured evidence.

Next staging events: DNS A-record; web-admin deploy (separate approval).

## Production-ready

```text
not claimed
```

## Safety

| Item | Value |
| ---- | ----- |
| Read-only verify scripts | yes |
| Secrets in docs | no |
| Writes in verify | no |

## Next Step

Seed execution complete per operator confirmation. Optional machine verify output can be attached later.
