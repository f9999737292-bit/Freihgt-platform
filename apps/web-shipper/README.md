# Freight Platform Shipper

Nuxt 3 shipper office. Login, tab session, company context, and read-only shipment inbox/detail. The browser calls the API Gateway only.

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
