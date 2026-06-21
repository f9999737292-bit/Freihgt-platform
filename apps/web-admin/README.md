# 7Rights Freight Platform — Web Admin

Administrative Nuxt 3 frontend for the Freight Platform logistics system.

## Purpose

- Platform operator console
- Company and user management
- Transport orders, RFx, shipments, documents, billing registers
- API Gateway integration with JWT auth

## Environment variables

Copy `.env.example` to `.env`:

```bash
NUXT_PUBLIC_API_BASE_URL=http://localhost:8080
NUXT_PUBLIC_APP_NAME=Freight Platform Admin
NUXT_PUBLIC_DEFAULT_LOCALE=ru-RU
NUXT_PUBLIC_DEFAULT_TENANT_ID=74519f22-ff9b-4a8b-8fff-a958c689682f
NUXT_PUBLIC_MOCK_AUTH=true
```

| Variable | Description |
|----------|-------------|
| `NUXT_PUBLIC_API_BASE_URL` | API Gateway base URL |
| `NUXT_PUBLIC_APP_NAME` | Application title |
| `NUXT_PUBLIC_DEFAULT_LOCALE` | Default locale (`ru-RU`) |
| `NUXT_PUBLIC_DEFAULT_TENANT_ID` | Default tenant ID for login form prefill |
| `NUXT_PUBLIC_MOCK_AUTH` | When `true`, login accepts any credentials |

## Local development tenant

Для просмотра тестовой компании используйте tenant:

`74519f22-ff9b-4a8b-8fff-a958c689682f`

При входе в mock auth укажите:

- **Tenant ID:** `74519f22-ff9b-4a8b-8fff-a958c689682f`
- **Email:** demo@7rights.local
- **Password:** any

Test company ID: `59a3e6b8-34c0-452a-894c-00938211f9f3` (OOO Virtual Logistics)

## Users and Company Members

Use the same development tenant for users and company membership:

`74519f22-ff9b-4a8b-8fff-a958c689682f`

### Create a user

1. Open `/users`
2. Click **Create user**
3. Fill in full name, email, password (min 8 chars), preferred locale
4. Submit — user is created via `POST /api/v1/users`

### Add user to a company

1. Open company details, e.g. `/companies/59a3e6b8-34c0-452a-894c-00938211f9f3`
2. In **Company members**, click **Add employee**
3. Choose mode:
   - **Create new user** — creates user first, then adds membership
   - **Add existing user** — search by name/email or enter `user_id` manually
4. Set position (required) and optional `role_id`
5. Submit — membership is created via `POST /api/v1/companies/{id}/members`

### View user companies

Open `/users/{user_id}` — the **User companies** table loads data from `GET /api/v1/users/{user_id}/companies?tenant_id=...`

## RFx and Mini Tenders

Use tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` and ensure the backend is running (`make platform-up`, `make migrate-up`).

### Create an RFx event

1. Open `/rfx`
2. Click **Create RFx**
3. Fill required fields (number, type, category, title, owner company, response deadline)
4. Submit — event is created via `POST /api/v1/rfx-events`
5. On the detail page (`/rfx/{id}`), click **Publish RFx** when status is `DRAFT`
6. Add carriers under **Participants** via **Add participant**

### Create a mini tender from a transport order

1. Open a transport order with status `READY_FOR_SOURCING`, e.g. `/transport-orders/{id}`
2. Click **Create mini tender** — the modal pre-fills `transport_order_id` and `shipper_company_id`
3. Or open `/freight-requests` and click **Create from transport order**
4. Select a transport order in `READY_FOR_SOURCING`, set number, type (`MINI_TENDER` by default), shipper, deadline, currency
5. Submit — freight request is created via `POST /api/v1/freight-requests/from-transport-order`

### Publish a mini tender

1. Open `/freight-requests/{id}`
2. When status is `DRAFT`, click **Publish mini tender**
3. After publish, carriers can submit bids (you still add test bids manually in dev)

### Create a test bid

1. On the freight request detail page, click **Add bid**
2. Select a carrier company (`company_type=CARRIER`), fill bid number, amounts, VAT, valid until
3. Submit — bid is created via `POST /api/v1/freight-requests/{id}/bids`
4. For a `DRAFT` bid, click **Submit bid** (`POST /api/v1/bids/{id}/submit`)

### Accept the winning bid

1. On a `SUBMITTED` bid, click **Accept bid** (confirmation dialog)
2. Bid is accepted via `POST /api/v1/bids/{id}/accept`
3. The page shows **Create shipment** — opens `/shipments?bid_id=...&transport_order_id=...` with the create-from-bid modal pre-filled

## Shipments

Use tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` for local development.

### Create shipment from accepted bid

1. Accept a bid on `/freight-requests/{id}` and click **Create shipment**
2. Or open `/shipments?bid_id={bidId}&transport_order_id={transportOrderId}` — the create modal opens automatically
3. Fill shipment number, planned pickup/delivery dates
4. Submit — shipment is created via `POST /api/v1/shipments/from-bid`

### Create shipment from transport order

1. Open `/shipments` and click **Create from transport order**
2. Select transport order, carrier company, optional forwarder, dates
3. Submit — shipment is created via `POST /api/v1/shipments/from-transport-order`

### Assign driver and vehicle

1. Open `/shipments/{id}`
2. Click **Assign driver** — select an existing driver or **Create driver**
3. Click **Assign vehicle** — select an existing vehicle or **Create vehicle**
4. APIs: `POST /api/v1/shipments/{id}/assign-driver`, `POST /api/v1/shipments/{id}/assign-vehicle`

### Move status to READY_FOR_BILLING

1. When status is `CARRIER_ASSIGNED`, click **Accept shipment**
2. Use **Next status** to advance through the lifecycle (or assign driver/vehicle first as required by backend)
3. At `DOCUMENTS_COMPLETED`, click **Move to READY_FOR_BILLING**
4. At `READY_FOR_BILLING`, use **Create billing register** (links to `/billing-registers?shipment_id=...` — billing UI is planned separately)

## Documents and Signing

Use tenant `74519f22-ff9b-4a8b-8fff-a958c689682f` for local development.

### Create POD for a shipment

1. Open a shipment with status `DELIVERED`, `DELIVERY_CONFIRMED`, or `DOCUMENTS_COMPLETED`
2. Click **Create POD** — opens `/documents?shipment_id={id}&document_type=POD` with the create modal pre-filled
3. Or open `/documents` and click **Create document**
4. Submit — document is created via `POST /api/v1/documents`

### Create a document version

1. Open `/documents/{id}` while status is `DRAFT`
2. Click **Create version** — `POST /api/v1/documents/{id}/versions`

### Add file metadata

1. On the document detail page, click **Add file**
2. Select document version and fill storage metadata (no real upload)
3. Submit — `POST /api/v1/documents/{id}/files`

### Ready for signing

1. Click **Ready for signing** on a `DRAFT` document
2. Submit — `POST /api/v1/documents/{id}/ready-for-signing`

### Create signing session and mock signatures

1. When status is `READY_FOR_SIGNING`, click **Create signing session**
2. Set `required_signers_count` (e.g. 2) and expiry
3. Click **Add mock signature** for each signer (select user + company)
4. After enough signatures, document becomes `SIGNED`

### Archive document

1. When status is `SIGNED` or `ACCEPTED`, click **Archive document**
2. Submit — `POST /api/v1/documents/{id}/archive`

## Mock auth mode

When `NUXT_PUBLIC_MOCK_AUTH=true`:

- POST `/api/v1/auth/login` is not called
- Any email/password works
- Tenant ID from the login form is used for API requests (`X-Tenant-ID` header)
- Mock user: **Demo Admin** / `demo@7rights.local`

Use mock mode for UI development when backend is unavailable.

## Run locally

From monorepo root:

```bash
make install-web-admin
make run-web-admin
```

Or from this directory:

```bash
npm install
npm run dev
```

Open: http://localhost:3000

## Pages

| Route | Description |
|-------|-------------|
| `/login` | Sign in |
| `/dashboard` | Overview cards |
| `/companies` | Company list and create |
| `/users` | User list and create |
| `/transport-orders` | Transport orders |
| `/rfx` | RFx / tenders |
| `/freight-requests` | Freight requests / mini tenders |
| `/shipments` | Shipments |
| `/documents` | Documents |
| `/billing-registers` | Billing registers |
| `/settings` | Environment and session info |

## API Gateway dependency

All API calls go through `NUXT_PUBLIC_API_BASE_URL` (default `http://localhost:8080`):

- Auth: `POST /api/v1/auth/login`
- Resources: `GET/POST /api/v1/...`

Ensure the backend platform is running:

```bash
make platform-up
make migrate-up
make health-check
```

See also: [docs/FRONTEND_BACKEND_STATUS.md](../../docs/FRONTEND_BACKEND_STATUS.md) — mock mode vs real backend, status banner, troubleshooting.

OpenAPI docs: http://localhost:8080/docs

## Backend status UX

- Mock mode gives UI permissions only.
- Real API data requires backend.
- Start backend:

  ```bash
  make platform-up
  make migrate-up
  make health-check
  ```

- UI shows **Backend online/offline** banner in the main layout (dev, mock mode, or when gateway is unreachable).
- Login page shows backend status with refresh.
- Network errors from `useApi` return `BACKEND_UNAVAILABLE` with a clear message.

## Docker

Build from monorepo root:

```bash
docker build -f apps/web-admin/Dockerfile -t freight-platform/web-admin .
```

Not included in docker-compose yet.

## Scripts

```bash
npm run dev        # development server (port 3000)
npm run build      # production build
npm run preview    # preview production build
npm run lint       # ESLint
npm run typecheck  # TypeScript check
```
