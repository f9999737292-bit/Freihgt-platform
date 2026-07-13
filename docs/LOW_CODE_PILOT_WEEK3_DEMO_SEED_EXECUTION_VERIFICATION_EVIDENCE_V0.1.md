# Low-code Pilot Week-3 Demo Seed Execution Verification Evidence v0.1

## Summary

Verification pack for staging demo seed execution (STG-LIM-005 / STG-LIM-006).

Operator approval was captured in execution pack v0.1. This pack provides runnable scripts and read-only verification matrix.

Remote verification from agent environment: **not captured** — operator runs scripts locally or on server.

## Decision

```text
DEMO_SEED_EXECUTION_VERIFICATION_PENDING_OPERATOR_RUN
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

## Verification Matrix

| Test ID | Check | Method | Expected | Actual | Result |
| ------- | ----- | ------ | -------- | ------ | ------ |
| VFY-001 | Gateway health | GET `/health` | 200 | pending | PENDING |
| VFY-002 | Demo transport orders | GET transport-orders | 200 + DEMO-TO-* | pending | PENDING |
| VFY-003 | Demo shipments | GET shipments | 200 + DEMO-SH-* | pending | PENDING |
| VFY-004 | Demo billing registers | GET billing-registers | 200 + DEMO-BR-* | pending | PENDING |
| VFY-005 | Runtime template | GET active template | 200 | pending | PENDING |
| VFY-006 | Custom field values | seed-lowcode-demo output | values seeded | pending | PENDING |

Fill Actual/Result after operator run.

## STG-LIM Impact

| ID | Status |
| -- | ------ |
| STG-LIM-005 | OPEN — verification pending |
| STG-LIM-006 | OPEN — verification pending |

Closure candidate only after VFY-002..006 pass.

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

After successful run, operator writes **«seed выполнен»** with verify script output for evidence update and STG-LIM closure candidate review.
