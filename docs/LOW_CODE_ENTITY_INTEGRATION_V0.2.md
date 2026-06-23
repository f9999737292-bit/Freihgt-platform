# Low-code Entity Integration v0.2

## Summary

Wires **validation_context** from core entity detail pages into `LowCodeCustomFieldsPanel` save flow. Entity detail panels for transport orders, shipments, and billing registers now build a compact context snapshot from loaded core entity state and send it on `PUT /v1/low-code/custom-field-values`. Runtime APIs, admin auth defaults, and core business logic are unchanged.

## Scope

**In scope**

- `apps/web-admin/utils/lowCodeValidationContext.ts` — builders + compact merge helpers
- Entity detail pages: transport order, shipment, billing register
- `LowCodeCustomFieldsPanel.vue` — optional `validationContext` prop on save
- Lightweight verification script (no new test framework)
- This document and `NEXT_COMMANDS.md`

**Out of scope**

- Core service → low-code-service direct calls
- Database migrations
- Core entity lifecycle / status / financial rules
- Expanding backend `validation_context` parsing beyond `entity_status` + `role` (extra keys are ignored safely)
- New frontend test framework (Vitest/Jest)

## Entity Types

| Priority | Entity type | Detail page | Builder |
|----------|-------------|-------------|---------|
| 1 | `TRANSPORT_ORDER` | `/transport-orders/[id]` | `buildTransportOrderValidationContext` |
| 2 | `SHIPMENT` | `/shipments/[id]` | `buildShipmentValidationContext` |
| 3 | `BILLING_REGISTER` | `/billing-registers/[id]` | `buildBillingRegisterValidationContext` |

Generic entry point: `buildLowCodeValidationContext(entityType, entity, options?)`.

## validation_context Contract

PUT body field (unchanged contract):

```json
{
  "entity_type": "TRANSPORT_ORDER",
  "entity_id": "...",
  "form_template_id": "...",
  "validation_context": {
    "entity_type": "TRANSPORT_ORDER",
    "entity_id": "...",
    "entity_status": "READY_FOR_SOURCING",
    "role": "PLATFORM_ADMIN",
    "cargo_type": "TENT_20T",
    "route": { "from": "...", "to": "..." },
    "dates": { "loading_date": "...", "delivery_date": "..." }
  },
  "values": []
}
```

### Backend keys used today (v0.1)

| Key | Used by low-code-service |
|-----|--------------------------|
| `entity_status` | Conditional required rules (`context.entity_status`) |
| `role` | Conditional required rules (`context.role`) |

Additional compact keys (`cargo_type`, `route`, `dates`, IDs, `period`, `amount`, `currency`) are sent for traceability and future rules; backend ignores unknown fields without error.

### Safety rules

- Missing core fields → omitted (no throw)
- No secrets, tokens, or full nested entity blobs
- Strings capped at 128 characters
- Only `route` and `dates` nested objects allowed

## Frontend Integration

### Helper

**Path:** `apps/web-admin/utils/lowCodeValidationContext.ts`

| Function | Purpose |
|----------|---------|
| `buildTransportOrderValidationContext(order)` | TO snapshot |
| `buildShipmentValidationContext(shipment, options?)` | Shipment snapshot; optional route labels |
| `buildBillingRegisterValidationContext(register)` | Billing register snapshot |
| `buildLowCodeValidationContext(type, entity)` | Dispatch by entity type |
| `mergeLowCodeValidationContext(...)` | Merge entity + preview + role |
| `compactValidationContextForPut(ctx)` | Strip empty / unsafe values |

### Panel

`LowCodeCustomFieldsPanel.vue`:

| Prop | Behavior |
|------|----------|
| `validationContext` | Optional entity snapshot from detail page |
| (existing) `entityStatus`, `previewContext` | Still supported |

On save:

```typescript
validation_context: effectiveValidationContext.value
```

If `validationContext` is absent → save behaves as before (role + `entity_status` from props only).

### Entity pages

Each priority detail page passes `:validation-context="lowCodeValidationContext"` computed from loaded core entity.

## Backend Compatibility

- `PUT` **without** `validation_context` — unchanged, still valid
- `PUT` **with** `validation_context` — unchanged envelope; extra JSON keys ignored by Go struct unmarshaling
- Conditional required validation reads `entity_status` and `role` from merged context
- No changes to `transport-order-service`, `shipment-service`, or `billing-register-service`

## Auth Compatibility

| Setting | Behavior |
|---------|----------|
| `LOW_CODE_ADMIN_AUTH_ENABLED` unset / `false` | Default; smoke/curl without `X-User-ID` work |
| `LOW_CODE_ADMIN_AUTH_ENABLED=true` | Admin routes require `PLATFORM_ADMIN`; runtime GET/PUT unchanged |

Pilot: set `LOW_CODE_ADMIN_AUTH_ENABLED=true` on `low-code-service`. See `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`.

## Tests

No Vitest/Jest in web-admin v0.2. Verification:

```powershell
node scripts/dev/verify_lowcode_validation_context.mjs
cd D:\Projects\freight-platform\apps\web-admin
npm run build
```

Script checks:

- TRANSPORT_ORDER / SHIPMENT / BILLING_REGISTER context shape
- Missing-field safe fallback
- No large nested object leakage

## Manual Verification

After `make seed-demo-data` + `make seed-lowcode-demo`:

| Entity | URL | Check |
|--------|-----|-------|
| Transport order | `/transport-orders/{DEMO-TO-id}` | Panel loads; edit/save succeeds; network PUT includes `validation_context.entity_status` |
| Shipment | `/shipments/{DEMO-SH-id}` | Same + `transport_order_id` / route when loaded |
| Billing register | `/billing-registers/{DEMO-BR-id}` | Same + `period` / `amount` when present |

Browser DevTools → Network → `PUT .../custom-field-values` → request body `validation_context`.

## Safety Guardrails

- `validation_context` is **soft validation only** — not used for billing approval, UPD, or core workflow gates
- Core services do not call low-code in this pack
- Client-built context is not a trust boundary for financial decisions

## What Is Not Implemented Yet

- Core BFF automatic header/body forwarding from server-side saves
- Backend parsing of extended context keys (`cargo_type`, `route`, etc.) for rules
- Vitest component tests for `LowCodeCustomFieldsPanel`
- RFx / document / freight-request validation context wiring (lower priority)

## Next Action

**Low-code Runtime Pilot Readiness Pack v0.1** — pilot checklist, operator runbook, and go/no-go criteria for controlled production integration.

See `docs/NEXT_COMMANDS.md`.
