# Production Demo Walkthrough Script v0.1

## Purpose

This script is for a first owner/product review of the production result.

## Opening

```text
We are reviewing the current controlled production state of Bintrans.
Production deployment is closed.
Monitoring cycle v0.2 passed.
This is not yet the final commercial product; this is a technical/pilot-ready baseline.
```

## Walkthrough

### 1. Production availability

Open:

```text
https://бинтранс.рф/
```

Check:

* page opens
* HTTPS is valid
* visual first impression
* no browser security warning

### 2. Login route

Open:

```text
https://бинтранс.рф/login
```

Check:

* route opens
* layout is acceptable
* no visible runtime error

### 3. Technical health

Open:

```text
https://бинтранс.рф/health
```

Check:

* 200 response

### 4. Staging comparison

Open:

```text
https://staging.бинтранс.рф/
```

Check:

* staging opens
* production and staging are separated

### 5. Product gap capture

Record:

* what is understandable
* what is confusing
* what is missing for pilot users
* which role should be demonstrated first

## Demo Result Options

```text
A. Accept current state as technical production baseline.
B. Prepare pilot demo data and role-based walkthrough.
C. Improve UI/landing/login before showing to external users.
D. Start a specific product module pack: TMS, RFx, billing, documents, admin, dashboard.
```

## Recommended Next Step After Demo

```text
PILOT_DEMO_DATA_AND_ROLE_WALKTHROUGH_PACK
```
