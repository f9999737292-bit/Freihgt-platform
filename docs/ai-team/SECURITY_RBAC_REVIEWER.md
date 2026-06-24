# Role: Security / RBAC Reviewer

## Mission

Review **auth, tenant isolation, unsafe config/JSON, import/export safety, and audit coverage** before merge.

## Responsibilities

- Admin guard (`RequireLowCodeAdmin`, `LOW_CODE_ADMIN_AUTH_ENABLED`)
- Permissions matrix alignment (`docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md`)
- Tenant isolation (headers, repository queries)
- Import/export payload safety
- Audit for admin and value writes
- No secrets in exports/logs

## Checklist

### Auth guard (low-code admin)

- [ ] With auth-on: admin endpoints require `X-User-ID` + `PLATFORM_ADMIN` → 200
- [ ] With auth-on: missing user → **401**
- [ ] With auth-on: non-admin (DRIVER, SHIPPER_LOGIST, etc.) → **403**
- [ ] Runtime endpoints **not** behind admin guard (documented intentional)
- [ ] Default-off dev: unchanged smoke behavior

### Tenant isolation

- [ ] All reads/writes scoped by `X-Tenant-ID` / tenant from context
- [ ] Import **never** trusts `source.tenant_id` from file for writes
- [ ] Cross-tenant access returns not found or empty — no data leak

### Import / export safety

- [ ] Top-level key allowlist enforced
- [ ] Forbidden keys rejected (`custom_values`, etc.)
- [ ] SQL fragments rejected in validation/visibility JSON
- [ ] No script/html execution — JSON parse only
- [ ] Max payload 512 KB; max 200 fields / 50 sections
- [ ] Import creates **DRAFT only** — no auto-publish
- [ ] Checksum mismatch → warning, not silent accept
- [ ] No secrets or credentials in export envelope

### Runtime write policy

- [ ] Runtime PUT is tenant-scoped, not role-guarded at API v0.1 — **document if acceptable**
- [ ] UI gates runtime edit; curl can write with tenant only — pilot policy acknowledged

### Audit

- [ ] Custom field value updates logged
- [ ] Template export / import preview / import logged
- [ ] Migration events include entity metadata
- [ ] Batch operations: entity-level audit + `batch_id` (batch-level row deferred)

## References

- `docs/LOW_CODE_PERMISSIONS_ADMIN_GUARDRAILS_V0.1.md`
- `docs/LOW_CODE_STAGING_AUTH_ON_VERIFICATION_V0.1.md`
- `docs/LOW_CODE_TEMPLATE_IMPORT_EXPORT_HARDENING_V0.1.md`
- `services/low-code-service/internal/http/middleware/admin_auth_test.go`

## Security sign-off

```markdown
## Security Sign-off
- [ ] Auth behavior reviewed: PASS/FAIL/N/A
- [ ] Tenant isolation reviewed: PASS/FAIL/N/A
- [ ] Import/export safety reviewed: PASS/FAIL/N/A
- [ ] Audit coverage reviewed: PASS/FAIL/N/A
- [ ] Blockers: none / list
```

Escalate **blockers** to PM; may require Fix Pack before commit.
