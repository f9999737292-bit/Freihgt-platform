# Low-code Pilot Release Notes v0.1

## Release Name

**Low-code Pilot Release v0.1**

Limited staging/pilot release of the low-code runtime layer for custom fields on core entities.

## What Is Included

- Published form templates and **active template** resolution per entity type
- **Custom field values** read/edit on entity detail pages
- **validation_context** for conditional field rules (advisory)
- **Admin UI** for template builder, clone-to-draft, publish, export/import
- **Migration preview/execute** and batch migration (with guardrails)
- **Audit log** for value changes and admin actions
- **Permissions matrix** and admin auth guard (when enabled)

## Who Can Use It

| Audience | Access |
|----------|--------|
| **PLATFORM_ADMIN** | Full admin low-code (templates, import/export, migration) |
| **Operator roles** (shipper logist, etc.) | Runtime custom field edit on allowed entities (UI) |
| **DRIVER** | No runtime edit in UI v0.1 |
| **Pilot scope** | **One tenant**, **TRANSPORT_ORDER** first |

## Admin Capabilities

- Create/edit DRAFT templates, publish with review
- Export template JSON (portable v1)
- Import preview → execute creates **DRAFT only** (never auto-publish)
- Migration preview before data moves
- Batch migration preview (max 100 entities)

**Requires:** `LOW_CODE_ADMIN_AUTH_ENABLED=true` in staging + `PLATFORM_ADMIN` role.

## Runtime Capabilities

- Load active template for entity type
- Display and save custom field values on transport order detail (Phase 1)
- Audit trail of changes

Runtime APIs are **tenant-scoped** (no admin role check at API v0.1).

## Safety Rules

1. **Preview before execute** — migration and batch migration
2. **Export before change** — templates and imports
3. **No auto-publish** on import
4. **No manual database edits**
5. **Auth-on in staging** — admin routes protected
6. **One pilot tenant** — isolated from production data
7. **Do not use** low-code validation for billing/financial authorization

## Known Limitations

- Runtime API write is not role-guarded (tenant header only) — UI gates operators
- No automated frontend test suite (Vitest)
- Batch-level audit row deferred
- Import/export limited to 200 fields per template
- Staging auth-on must be verified after deploy (local verification done)
- Manual UI walkthrough recommended before pilot users

## Support / Escalation

| Issue | Action |
|-------|--------|
| Admin blocked | Check PLATFORM_ADMIN + auth-on flag |
| Wrong field values | Audit log → entity_id; do not manual DELETE |
| Template problem | Keep previous PUBLISHED; use export backup |
| Service down | Entity panels show unavailable; core platform unaffected |

Escalate stop conditions to platform ops and pilot lead. See `LOW_CODE_PILOT_OPERATOR_CHECKLIST_V0.1.md`.

## Rollback

1. Disable pilot user access or set `LOW_CODE_ADMIN_AUTH_ENABLED=false` if auth blocks ops (approved)
2. Do **not** publish bad DRAFTs
3. Restore previous PUBLISHED template from export
4. DB restore from pre-pilot backup (DBA only) if widespread bad writes

Full procedure: `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md` → Rollback Procedure.

---

**Decision:** GO_WITH_CONDITIONS  
**Docs:** `LOW_CODE_PILOT_RELEASE_PACKAGE_V0.1.md`  
**Next step:** Manual UI Verification Pack v0.1
