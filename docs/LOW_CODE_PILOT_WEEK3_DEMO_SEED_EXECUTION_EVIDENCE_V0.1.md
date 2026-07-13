# Low-code Pilot Week-3 Demo Seed Execution Evidence v0.1

## Summary

Operator approval captured for staging demo seed execution. Operator confirmed completion on 2026-07-13.

## Decision

```text
DEMO_SEED_EXECUTION_OPERATOR_CONFIRMED_COMPLETE
```

## Operator Approval

```text
разрешаю staging seed execution
разрешаю staging demo seed на сервере
```

## Operator Completion Confirmation (2026-07-13)

```text
seed выполнен
```

Machine-captured verify script output was not attached to the operator message. Completion recorded per operator confirmation after approved server run.

## Execution Status

| Step | Status |
| ---- | ------ |
| Operator approval | **captured** (including server run) |
| SSH seed-demo-data (STG-LIM-005) | **complete** — operator-confirmed |
| SSH seed-lowcode-demo (STG-LIM-006) | **complete** — operator-confirmed |
| Post-seed read-only verify | **operator-confirmed pass** — output not attached |

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
| STG-LIM-005 | **CLOSED** — seed-demo-data executed (operator-confirmed) |
| STG-LIM-006 | **CLOSED** — seed-lowcode-demo executed (operator-confirmed) |

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

Optional: attach verify script output (`PASS VFY-001..005`) for machine-captured evidence.

Next recommended staging events: DNS A-record `staging.bintrans.ru` → `161.104.53.221`; web-admin deploy (separate approval).

See `docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_VERIFICATION_EVIDENCE_V0.1.md`.
