# BINTRANS Identity Wave A2 — Operational Migration Toolkit

## PURPOSE

Controlled, UUID-preserving in-place rename of synthetic development/staging identities:

- `@7rights.local` → `@bintrans.local` (five canonical demo users)
- `dev-7rights` → `dev-bintrans`
- `7Rights Dev Tenant` → `Bintrans Dev Tenant`
- `ООО 7Rights Dev` → `ООО Bintrans Dev`

**Tenant UUID is fixed:** `74519f22-ff9b-4a8b-8fff-a958c689682f`

These scripts are **operational migration tools**, not schema migrations. They do not recreate users, merge duplicates, or retire legacy rows automatically.

`7rights.ru` is an unrelated external website and is **not** part of this migration. Do not access or configure it.

## SUPPORTED_ENVIRONMENTS

| Environment | Use |
|---|---|
| Isolated local disposable Postgres | **Yes** — validation only |
| Developer shared local Postgres | Manual — backup first; prefer isolated validation |
| Staging | **Future A2.5 only** — after read-only inventory (A2.4) |
| Production | **`PRODUCTION_USE=FORBIDDEN`** unless separately approved |

## PRECONDITIONS

1. Schema migrations already applied (`make migrate-up` or equivalent).
2. Database backup/snapshot taken before any mutation.
3. `preflight.sql` executed and reviewed.
4. `EMAIL_COLLISION_COUNT = 0` (no target email owned by a **different** user UUID).
5. Application code/seeds updated to Bintrans defaults (Wave A2.1).

## PREFLIGHT

```bash
psql "$DB_URL" -f scripts/ops/bintrans_identity_a2/preflight.sql
```

Review:

- `OLD_IDENTITY_COUNT`
- `TARGET_IDENTITY_COUNT`
- `EMAIL_COLLISION_COUNT`
- `TENANT_CODE_COLLISION`
- Platform operator company rows

**Do not proceed if `COLLISION` rows appear** unless duplicate users are resolved manually in a future approved task.

## BACKUP_REQUIREMENT

Take a logical backup or volume snapshot before `migrate.sql`:

```bash
pg_dump "$DB_URL" -Fc -f bintrans-a2-pre-migration.dump
```

## MIGRATION

```bash
psql "$DB_URL" -v ON_ERROR_STOP=1 -f scripts/ops/bintrans_identity_a2/migrate.sql
```

Properties:

- Single transaction (`BEGIN` … `COMMIT`)
- Fail-closed on duplicate target emails (`DUPLICATE_TARGET_EMAIL_POLICY=FAIL_CLOSED`)
- Preserves user/company UUIDs, memberships, and `user_roles`
- Does **not** delete or disable duplicate users

## POSTCHECK

Re-run `preflight.sql` and verify:

- `@7rights.local` user count in tenant = 0
- Canonical `@bintrans.local` users exist (exactly one each after clean migration)
- Tenant code = `dev-bintrans`
- Users must re-login (JWT `email` claim may be stale until token expiry)

Optional RBAC spot-check:

```sql
SELECT ur.user_id, ur.company_id, r.code
FROM core.user_roles ur
JOIN core.roles r ON r.id = ur.role_id
JOIN core.users u ON u.id = ur.user_id
WHERE u.tenant_id = '74519f22-ff9b-4a8b-8fff-a958c689682f'::uuid;
```

## ROLLBACK

For controlled rollback only (not for staging/production without explicit approval):

```bash
psql "$DB_URL" -v ON_ERROR_STOP=1 -f scripts/ops/bintrans_identity_a2/rollback.sql
```

Rollback also fails closed on reverse email collisions.

## DUPLICATE_POLICY

If a canonical `@bintrans.local` user already exists with a **different UUID** than the legacy `@7rights.local` user:

```text
DUPLICATE_TARGET_EMAIL_POLICY=FAIL_CLOSED
```

The migration **aborts**. No partial mutation. No automatic merge. No automatic RBAC transfer. No automatic user disable/delete.

Document supported `core.users.status` values (`ACTIVE`, `BLOCKED`, `DISABLED`, etc.) for manual retirement in future A2.5 — **not invoked by these scripts**.

## SESSION_RELOGIN_REQUIREMENT

After migration, invalidate active sessions or require users to log in again. JWT authorization uses `sub` (user UUID) so tokens remain technically valid, but email claims and operator expectations diverge until re-login.

## FILES

| File | Purpose |
|---|---|
| `preflight.sql` | Read-only inventory and collision detection |
| `migrate.sql` | UUID-preserving forward migration |
| `rollback.sql` | UUID-preserving reverse migration |
| `fixtures/legacy_dev_identity.sql` | Disposable DB legacy state for local migration tests |
| `fixtures/collision_extra_user.sql` | Disposable collision scenario |
| `run_local_validation.sh` | Isolated local seed + migration test orchestration (Unix/WSL) |
| `run_local_validation.ps1` | Isolated local seed + migration test orchestration (Windows PowerShell) |

## LOCAL VALIDATION (DISPOSABLE ONLY)

These wrappers validate Wave A2.1 on an isolated Compose project (`bintrans_a21_validate`) with a disposable Postgres volume. They **must not** be pointed at staging or production.

```text
STAGING_USE=FORBIDDEN
PRODUCTION_USE=FORBIDDEN
```

### Windows PowerShell (supported)

Requires Docker Desktop, Git Bash (for repository seed scripts), and ports `55432`, `18080`–`18082` available locally.

```powershell
powershell -ExecutionPolicy Bypass -File `
  scripts/ops/bintrans_identity_a2/run_local_validation.ps1
```

Notes:

- Uses `ports: !override` in `infrastructure/docker-compose/docker-compose.a21-validate.yml` so Windows Compose does not merge dev ports (`5432`, `8080`, etc.).
- Starts identity/company services plus a localhost mock gateway (`18080`) because the full api-gateway image may not build on all Windows setups.
- If a rapid disposable reset interrupts golang-migrate, recovery uses `migrate force` **only** when the disposable volume has no `core` schema yet.

### Unix / Linux / WSL (supported when Docker build succeeds)

```bash
bash scripts/ops/bintrans_identity_a2/run_local_validation.sh
```

Git Bash on Windows is **not** supported for this Bash wrapper (observed migrate bind-mount failure: `error: open .: no such file or directory`).

### Isolation guarantees

| Resource | Validation stack | Normal dev stack |
|---|---|---|
| Compose project | `bintrans_a21_validate` | default / `freight` |
| Postgres volume | `bintrans_a21_validate_data` | `freight_postgres_data` |
| Postgres port | `55432` | `5432` |
| Identity / company / gateway | `18081` / `18082` / `18080` | `8081` / `8082` / `8080` |

Cleanup removes only `bintrans_a21_validate_*` containers/volumes (`docker compose ... down -v` with `COMPOSE_PROJECT_NAME=bintrans_a21_validate`).

## PRODUCTION_USE

```text
PRODUCTION_USE=FORBIDDEN
```

Synthetic Bintrans dev identities are not production customer data. Any production use requires a separate approved task.
