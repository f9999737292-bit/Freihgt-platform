# Freight Platform Finance

Nuxt 3 frontend skeleton for the finance portal.

## Development

From the monorepo root:

```bash
pnpm install
pnpm --filter @freight-platform/web-finance dev
```

Or from this directory:

```bash
pnpm install
pnpm dev
```

The app runs at http://localhost:3004

## Scripts

- `pnpm dev` — Start development server
- `pnpm build` — Build for production
- `pnpm preview` — Preview production build

## Health Check

`GET /api/health` returns service status.
