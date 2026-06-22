# Demo Seed Pack v0.1

## Summary

`make seed-demo-data` populates the dev tenant with deterministic demo entities so web-admin list pages are not empty. The script is idempotent: it finds existing records by stable business keys (`legal_name`, `email`, `order_number`, etc.) and skips creation when data is already present.

Prerequisites:

1. Platform services running (`make platform-up-no-build` or `make platform-up-safe`)
2. Dev admin seeded (`make seed-dev-admin`)

No backend business logic, API contracts, or production code are modified by this workflow.

## Demo Tenant

| Field | Value |
|---|---|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Tenant code | `dev-7rights` |
| Name | 7Rights Dev Tenant |

## Demo Login

| Role | Email | Password |
|---|---|---|
| Platform admin | admin@7rights.local | Admin123456! |
| Shipper | shipper@7rights.local | Demo123456! |
| Carrier | carrier@7rights.local | Demo123456! |
| Forwarder | forwarder@7rights.local | Demo123456! |
| Consignee | consignee@7rights.local | Demo123456! |

Login URL: http://localhost:3000/login

## What Data Is Created

### A. Companies

| Legal name | Type | Notes |
|---|---|---|
| ООО 7Rights Dev | PLATFORM_OPERATOR | May already exist from `seed-dev-admin` |
| ООО Грузовладелец Север | SHIPPER | Demo shipper |
| ООО Перевозчик Волга | CARRIER | Demo carrier |
| ООО Экспедитор Логистик | FORWARDER | Demo forwarder |
| ООО Грузополучатель Центр | CONSIGNEE | Demo consignee |

### B. Users / memberships

Demo users are created in identity-service with company memberships and roles:

- `shipper@7rights.local` → SHIPPER_LOGIST on ООО Грузовладелец Север
- `carrier@7rights.local` → CARRIER_DISPATCHER on ООО Перевозчик Волга
- `forwarder@7rights.local` → PROCUREMENT_MANAGER on ООО Экспедитор Логистик
- `consignee@7rights.local` → CONSIGNEE_OPERATOR on ООО Грузополучатель Центр

### C. Transport orders (5)

| Number | Route |
|---|---|
| DEMO-TO-001 | Москва → Санкт-Петербург |
| DEMO-TO-002 | Казань → Екатеринбург |
| DEMO-TO-003 | Краснодар → Ростов-на-Дону |
| DEMO-TO-004 | Новосибирск → Омск |
| DEMO-TO-005 | Нижний Новгород → Самара |

All orders are submitted to `READY_FOR_SOURCING`.

### D. Freight requests / RFx

Freight requests (from transport orders):

| Number | Transport order |
|---|---|
| DEMO-FR-001 | DEMO-TO-001 |
| DEMO-FR-002 | DEMO-TO-002 |
| DEMO-FR-003 | DEMO-TO-003 |

RFX event:

| Number | Title |
|---|---|
| DEMO-RFX-001 | Демо тендер: магистральные перевозки |

RFX lots, lanes, and participant responses are **not** seeded in v0.1 (see Known Limitations).

### E. Bids

| Number | Carrier / forwarder | Route |
|---|---|---|
| DEMO-BID-001 | ООО Перевозчик Волга | Москва — Санкт-Петербург |
| DEMO-BID-002 | ООО Перевозчик Волга | Казань — Екатеринбург |
| DEMO-BID-003 | ООО Экспедитор Логистик | Краснодар — Ростов-на-Дону |

Bids are submitted and accepted where the API flow allows.

### F. Shipments (3)

| Number | Scenario | Target status |
|---|---|---|
| DEMO-SH-PLANNED | Planned shipment | CARRIER_ASSIGNED |
| DEMO-SH-IN-PROGRESS | Active transport | IN_TRANSIT |
| DEMO-SH-BILLING | Billing-ready flow | READY_FOR_BILLING |

Driver `DEMO-LIC-001` and vehicle `DEMO-A123BC77` are created for in-progress and billing shipments.

### G. Documents

| Number | Type | Related shipment |
|---|---|---|
| DEMO-DOC-001 | POD | DEMO-SH-BILLING |

Document signing sessions are **not** created in v0.1 (optional UI enrichment).

### H. Billing registers

| Number | Customer | Contractor | Flow |
|---|---|---|---|
| DEMO-BR-001 | ООО Грузовладелец Север | ООО Экспедитор Логистик | create → add shipment → calculate → approve → UPD |

UPD number: `DEMO-UPD-001` with UTF-8 function code `СЧФДОП` (via jq unicode escape).

Post-UPD EDO mock steps (mark-sent-to-edo, mark-signed, mark-paid, close) are **not** run in demo seed to keep the register visible in intermediate UI states.

## Idempotency

The script uses find-or-create for every entity:

- Lists existing records and matches by stable keys
- Treats HTTP 409 conflicts as success and re-resolves IDs
- Advances shipment status only when current status is behind the target
- Safe to run repeatedly: `make seed-demo-data` three times produces the same counts

Technical notes:

- JSON POST bodies use `printf … | curl --data-binary @-` for UTF-8 safety on Windows Git Bash
- Uses the same service URL defaults as integration smoke tests

## How To Run

```bash
cd D:/Projects/freight-platform

# 1. Ensure platform is up
make platform-up-no-build

# 2. Seed dev admin (required once)
make seed-dev-admin

# 3. Seed demo data (repeatable)
make seed-demo-data

# Optional: verify
make health-check
make integration-smoke-test
```

Direct script invocation:

```bash
bash scripts/dev/seed_demo_data.sh
```

Environment overrides: `TENANT_ID`, `DEMO_PASSWORD`, `API_GATEWAY_URL`, service URLs (same as other dev scripts).

## UI Pages To Check

After seeding, log in as `admin@7rights.local` and verify list pages show demo rows:

| Page | Expected content |
|---|---|
| `/companies` | 5+ demo companies |
| `/transport-orders` | DEMO-TO-001..005 |
| `/rfx` | DEMO-RFX-001 |
| `/freight-requests` | DEMO-FR-001..003 |
| `/shipments` | DEMO-SH-PLANNED, IN-PROGRESS, BILLING |
| `/documents` | DEMO-DOC-001 |
| `/billing-registers` | DEMO-BR-001 |
| `/control-tower` | Active / in-transit shipments |
| `/dashboard` | Non-zero summary metrics |

## Known Limitations

| Area | Status |
|---|---|
| RFX lots / lanes / responses | Not seeded — API supports creation but omitted in v0.1 to limit scope |
| Document signing workflow | Not seeded — POD record only |
| Billing register post-UPD lifecycle | Not seeded — stops after UPD creation |
| DEMO-TO-004 / DEMO-TO-005 freight requests | Not created — orders exist for list UI only |
| Second carrier bid on same FR | Prevented by API unique constraint (`uq_bid_carrier_request`) |
| UI verification | Manual — run web-admin at http://localhost:3000 |

## Next Actions

1. Add RFX lot/lane/participant seed for richer `/rfx` detail pages
2. Seed document signing session for `/documents` workflow UI
3. Extend billing demo through EDO mock steps (optional closed register)
4. Add `seed-demo-data` to CI as optional post-smoke dev job (non-blocking)
5. Wire demo seed into local onboarding docs (`QUICK_START.md`)
