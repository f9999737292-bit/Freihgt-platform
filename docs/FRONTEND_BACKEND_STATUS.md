# Frontend Backend Status

How web-admin shows mock mode vs real backend availability.

## Mock mode vs real backend

### Mock mode (`NUXT_PUBLIC_MOCK_AUTH=true`)

- UI login may work without a running API Gateway.
- `admin@7rights.local` can be treated as platform admin in the UI (permissions checks).
- **API data still requires a real backend** — lists, dashboards, and mutations call `http://localhost:8080`.

### Real backend

Requires Docker platform stack:

```bash
make platform-up
make migrate-up
make health-check
```

Optional seed for dev admin:

```bash
make seed-dev-admin
```

## How to check backend

1. **Web-admin banner** — `BackendStatusBanner` in the main layout (dev, mock mode, or when offline).
2. **Login page** — backend status block before the form.
3. **Health page** — `/health` in web-admin (service monitoring UI).
4. **API Gateway directly:**

   ```text
   http://localhost:8080/health
   ```

## UI indicators

| Indicator | Meaning |
|-----------|---------|
| Backend online | API Gateway `/health` returned 200 |
| Backend offline | Gateway unreachable or non-200 |
| Mock mode active | `NUXT_PUBLIC_MOCK_AUTH=true` — UI auth bypass, not real API |

## Common issue

**UI opens but data does not load:**

- Backend is probably offline.
- Run:

  ```bash
  make platform-up
  make migrate-up
  make health-check
  ```

- In web-admin, use **Refresh backend status** on the banner or login page.

## Dev credentials (after seed)

| Field | Value |
|-------|-------|
| Tenant ID | `74519f22-ff9b-4a8b-8fff-a958c689682f` |
| Email | `admin@7rights.local` |
| Password | `Admin123456!` |

## Related docs

- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)
- [AUTH_RBAC.md](./AUTH_RBAC.md)
- [QUICK_START.md](./QUICK_START.md)
