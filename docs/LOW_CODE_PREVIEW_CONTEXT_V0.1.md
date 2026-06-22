# Low-code Preview Context v0.1

## Summary

Entity detail pages pass **entity status** and the signed-in user's **role** into form template preview via `LowCodeCustomFieldsPanel`. Context-aware visibility rules (`context.entity_status`, `context.role`) now evaluate on entity cards.

## Wiring

**Composable:** `apps/web-admin/composables/useLowCodePreviewContext.ts`

- Resolves role: `PLATFORM_ADMIN` for dev platform admin (`admin@7rights.local` + mock auth) or first entry from `user.roles` when available
- Builds `PreviewRuleContext` from `entityStatus` prop + role

**Panel:** `LowCodeCustomFieldsPanel.vue`

| Prop | Description |
| ---- | ----------- |
| `entityStatus` | Core entity status string (e.g. transport order `status`) |
| `previewContext` | Optional override / extension |

Passes merged context to `LowCodeFormTemplatePreview`.

## Entity pages

| Page | `entity-status` source |
| ---- | -------------------- |
| `/transport-orders/[id]` | `order.status` |
| `/shipments/[id]` | `shipment.status` |
| `/billing-registers/[id]` | `item.status` |
| `/freight-requests/[id]` | `request.status` |
| `/rfx/[id]` | `event.status` |
| `/documents/[id]` | `document.document_status` |

Preview shows active context chips (status / role) when present.

## Demo visibility rules (seed)

`make seed-lowcode-demo` updates:

| Field | Rule |
| ----- | ---- |
| `internal_cost_center` | visible when entity status in `READY_FOR_SOURCING`, `SOURCING_IN_PROGRESS`, `AWARDED` |
| `loading_contact_phone` | visible when entity status in `IN_TRANSIT`, `ARRIVED_AT_CONSIGNEE`, `DELIVERED`, `UNLOADING` |
| `payment_priority` | visible when role = `PLATFORM_ADMIN` |

**Verify (dev tenant, logged in as admin):**

- **DEMO-TO-001** — `internal_cost_center` visible when status matches sourcing flow
- **DEMO-SH-PLANNED** — `loading_contact_phone` hidden (planned status)
- **DEMO-SH-IN-PROGRESS** — `loading_contact_phone` visible if status is in-transit
- **DEMO-BR-001** — `payment_priority` visible for platform admin role

## Verification

```powershell
cd D:\Projects\freight-platform
make seed-lowcode-demo

cd apps/web-admin
$env:NUXT_IGNORE_LOCK=1; npm run build
npm run dev
```

## Limitations

- Role from auth is best-effort until `/auth/me` returns RBAC roles for all users
- `/low-code/custom-field-values` edit page does not pass entity status (no core entity loaded)
- Preview-only; save/API does not enforce context rules

## Next action

1. Server-side conditional required validation (future pack)
2. Create-first-value edit flow for empty entities

See also: `docs/LOW_CODE_ENTITY_DETAIL_INLINE_EDIT_V0.1.md`, `docs/LOW_CODE_PREVIEW_CONDITIONAL_REQUIRED_V0.1.md`.
