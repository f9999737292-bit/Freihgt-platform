# Demo Scenario Controlled Walkthrough Script v0.1

## Purpose

Controlled walkthrough script for showing the current production static UI safely.

Base commit: `60fa973`.

## Audience

```text
Internal team / owner / investor-style static product walkthrough.
```

## Required Disclaimer

```text
This is a production static UI demo.
Live-data/authenticated workflow readiness is still partial and not signed off.
Full production readiness is not claimed.
```

## Walkthrough Flow

### 1. Entry

Open:

```text
https://бинтранс.рф/
```

Expected behavior:

* page opens without blank screen;
* unauthenticated user lands on login entry.

Talking point:

```text
Bintrans is positioned as a B2B logistics/TMS platform interface with role-based product areas for shippers, carriers, forwarders, finance, procurement, and platform administration.
```

### 2. Login

Open:

```text
https://бинтранс.рф/login
```

Show:

* clean login;
* no demo credentials;
* backend status online («Backend доступен»);
* no offline banner;
* no prefill.

Talking point:

```text
The previous production login prefill issue is fixed. The screen is now safe for controlled external viewing.
```

### 3. Product Areas

Explain representative routes conceptually. Do not enter credentials.

```text
/dashboard
/shipments
/freight-requests
/billing-registers
/transport-orders
/documents
/companies
/low-code
```

Current smoke behavior without authentication:

* routes are reachable at HTTP level (200 SPA shell);
* browser redirects unauthenticated navigation to login;
* authenticated page content is not available in this walkthrough.

Explain conceptually:

* dashboard — management overview;
* shipments — shipment execution;
* freight requests — tender/RFx/spot request area;
* billing registers — financial closing/billing workflow;
* transport orders — transport order lifecycle;
* documents — document workflow;
* companies — multi-tenant company directory;
* low-code — configurable forms/templates area.

Important caveat:

```text
Do not claim authenticated live workflows until a separate live-data demo workflow pack signs them off.
```

### 4. RBAC Product Concept

Talking point:

```text
The frontend artifact contains role-based navigation logic. Production static UI includes RBAC role navigation capability, but authenticated role-by-role production workflow is not signed off in this smoke.
```

### 5. Health / API

Technical-only proof:

```text
/health is the canonical backend-status endpoint.
/api/v1/* is the canonical business API route family.
/api/health is not canonical and its 404 is expected.
```

Do not show `/health` to business audience unless needed.

## Do Not Show / Do Not Claim

```text
Do not enter real credentials.
Do not show cookies/JWT/localStorage.
Do not claim full production readiness.
Do not claim full backend/API readiness.
Do not claim live-data demo readiness as complete.
Do not claim SLA/security/legal/document/billing/E2E readiness.
```

## Recommended Next Step

```text
Prepare LIVE_DATA_DEMO_WORKFLOW_PLAN_PACK v0.1 if authenticated data demo is required.
```

## Decision

```text
DEMO_SCENARIO_CONTROLLED_WALKTHROUGH_SCRIPT_CREATED
```
