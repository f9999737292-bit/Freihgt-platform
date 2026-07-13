# Low-code Pilot Week-3 Demo Seed Execution Evidence v0.1

## Summary

Operator approval captured for staging demo seed execution. Remote execution attempted from agent environment; shell output not captured — operator must confirm completion via SSH runbook below.

## Decision

```text
DEMO_SEED_EXECUTION_APPROVED_PENDING_OPERATOR_CONFIRMATION
```

## Operator Approval

```text
разрешаю staging seed execution
```

Captured in:

```text
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_OPERATOR_APPROVAL_CAPTURE_V0.1.md
```

## Execution Status

| Step | Status |
| ---- | ------ |
| Operator approval | **captured** |
| SSH seed-demo-data | **pending operator run** — agent shell no output capture |
| SSH seed-lowcode-demo | **pending operator run** |
| Post-seed read-only verify | **pending** |

## Script Update

`scripts/dev/seed_demo_data.sh` updated to support staging email overrides via env:

```text
ADMIN_EMAIL, SHIPPER_EMAIL, CARRIER_EMAIL, FORWARDER_EMAIL, CONSIGNEE_EMAIL
```

Defaults unchanged for local dev (`@7rights.local`).

## Operator SSH Runbook

Connect and execute:

```bash
ssh root@161.104.53.221

cd /opt/bintrans/freight-platform
git pull origin main

export API_GATEWAY_URL=http://161.104.53.221
export TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f
export IDENTITY_SERVICE_URL=http://127.0.0.1:8081
export COMPANY_SERVICE_URL=http://127.0.0.1:8082
export TRANSPORT_ORDER_SERVICE_URL=http://127.0.0.1:8083
export RFX_SERVICE_URL=http://127.0.0.1:8084
export SHIPMENT_SERVICE_URL=http://127.0.0.1:8085
export DOCUMENT_SERVICE_URL=http://127.0.0.1:8086
export BILLING_REGISTER_SERVICE_URL=http://127.0.0.1:8087
export ADMIN_EMAIL=admin@bintrans.local
export SHIPPER_EMAIL=shipper@bintrans.local
export CARRIER_EMAIL=carrier@bintrans.local
export FORWARDER_EMAIL=forwarder@bintrans.local
export CONSIGNEE_EMAIL=consignee@bintrans.local
# DEMO_PASSWORD — use staging demo password from secure channel

make seed-demo-data
make seed-lowcode-demo
```

## Post-seed Verification (read-only)

```powershell
$T = "74519f22-ff9b-4a8b-8fff-a958c689682f"
$h = @{ "X-Tenant-ID" = $T }
Invoke-WebRequest -UseBasicParsing http://161.104.53.221/health | Select-Object StatusCode
Invoke-WebRequest -UseBasicParsing -Headers $h "http://161.104.53.221/api/v1/transport-orders?tenant_id=$T&limit=5"
```

Expected: health **200**; transport-orders contain **DEMO-TO-*** entries.

## STG-LIM Impact

| ID | Status after this pack |
| -- | ---------------------- |
| STG-LIM-005 | OPEN — approval captured; execution pending confirmation |
| STG-LIM-006 | OPEN — approval captured; execution pending confirmation |

## Production-ready

```text
not claimed
```

## Safety

| Item | Value |
| ---- | ----- |
| Secrets in docs | no |
| Passwords in docs | no |
| Production data | no |
| Backend logic changed | no — seed script env overrides only |

## Next Step

Operator runs SSH runbook, then writes **«seed выполнен»** for verification evidence update and STG-LIM closure candidate review.
