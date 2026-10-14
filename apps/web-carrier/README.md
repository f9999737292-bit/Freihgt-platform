# Freight Platform Carrier

Nuxt 3 frontend skeleton for the carrier portal.

## Development

From the monorepo root:

```bash
pnpm install
pnpm --filter @freight-platform/web-carrier dev
```

Or from this directory:

```bash
pnpm install
pnpm dev
```

The app runs at http://localhost:3002.

Tracking, ETA, and slots are read-only and open from a transport order that already returns a shipment reference. The screen does not accept a typed shipment id, and it does not expose driver actions.

Documents are the same kind of read-only shipment screen. The office does not upload, sign, or archive them.

## Scripts

- `pnpm dev` — Start development server
- `pnpm build` — Build for production
- `pnpm preview` — Preview production build

## Health Check

`GET /api/health` returns service status.
