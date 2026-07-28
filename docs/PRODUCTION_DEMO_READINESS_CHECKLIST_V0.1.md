# Production Demo Readiness Checklist v0.1

## Summary

Production demo readiness checklist prepared after production deployment closure, monitoring cycle v0.2 PASS, and local workspace hygiene closure.

## Decision

```text
PRODUCTION_DEMO_READINESS_PREPARED
```

## Demo URLs

| Environment         | URL                              | Purpose                |
| ------------------- | -------------------------------- | ---------------------- |
| Production          | https://бинтранс.рф/             | owner/product review   |
| Production punycode | https://xn--80abvubqje.xn--p1ai/ | technical verification |
| Staging             | https://staging.бинтранс.рф/     | staging comparison     |

## Technical Checks Before Demo

| Check                                    | Expected      |
| ---------------------------------------- | ------------- |
| Production `/`                           | 200 text/html |
| Production `/login`                      | 200 text/html |
| Production `/health`                     | 200           |
| Production HTTP -> HTTPS                 | 301/308       |
| Production API active templates TO/SH/BR | 200           |
| Staging `/`                              | 200 text/html |
| Staging `/login`                         | 200 text/html |
| Staging `/health`                        | 200           |

## Pack Execution Evidence (2026-07-28)

| Check | Result |
| ----- | ------ |
| Production `/` | PASS — 200 text/html |
| Production `/login` | PASS — 200 text/html |
| Production `/health` | PASS — 200 |
| Production HTTP -> HTTPS | PASS — 301 |
| Production API TO | PASS — 200 |
| Production API SH | PASS — 200 |
| Production API BR | PASS — 200 |
| Staging `/` | PASS — 200 text/html |
| Staging `/login` | PASS — 200 text/html |
| Staging `/health` | PASS — 200 |

## 30–60 Minute Demo Plan

| Step                       |      Time | What to Check                                   |
| -------------------------- | --------: | ----------------------------------------------- |
| Open production            |     5 min | site loads, HTTPS, first impression             |
| Open login                 |     5 min | route works, UI loads                           |
| Compare staging            |     5 min | staging separate from production                |
| Check health/API readiness |     5 min | `/health` and active templates                  |
| Product walkthrough        | 15–25 min | what screens exist, what is missing, UX quality |
| Gap list                   | 10–15 min | what must be improved before pilot users        |

## Owner Review Questions

```text
1. Does the platform open reliably?
2. Is the brand/domain acceptable for pilot?
3. Is the login route usable?
4. What should be shown first to users: landing, login, dashboard, or controlled pilot screen?
5. What modules are ready enough to demonstrate?
6. What must be improved before inviting external users?
```

## Expected Gaps

```text
This is a controlled technical production/pilot state, not a final commercial release.
Likely next work may include:
- demo data
- role-based demo users
- shipper/carrier/forwarder flows
- RFx/TMS/billing walkthrough
- UI polishing
- business scenario scripts
- pilot acceptance checklist
```

## Safety

```text
Production changed: no
Staging changed: no
Server changed: no
Deploy executed: no
Database writes executed: no
Secrets captured: no
```
