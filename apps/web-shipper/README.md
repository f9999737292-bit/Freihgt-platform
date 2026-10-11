# Freight Platform Shipper

Nuxt 3 shipper office. Login, tab session, company context, read-only shipment inbox/detail, tracking, and shipment-scoped documents. The browser calls the API Gateway only. Document writes, signing, and map providers are not included.

## Development

From the monorepo root:

```bash
pnpm install
pnpm --filter @freight-platform/web-shipper dev
```

The app runs at http://localhost:3001.

## Scripts

- `pnpm dev` — Start development server
- `pnpm typecheck` — Typecheck
- `pnpm test` — Unit and component tests
- `pnpm build` — Build for production
- `pnpm test:e2e` — Playwright (headless)

## Health Check

`GET /api/health` returns service status.

Public runtime configuration is the API Gateway base URL only.
