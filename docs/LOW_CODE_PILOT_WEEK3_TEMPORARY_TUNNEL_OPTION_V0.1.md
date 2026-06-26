# Low-code Pilot Week-3 Temporary Tunnel Option v0.1

## Summary

Documents **Option B**: temporary HTTPS exposure of **local Docker** via Cloudflare Tunnel, ngrok, or similar — for **short-term** remote auth-on matrix evidence.

**This is not production.** **This does not replace full staging.**

## Purpose

When VPS + `staging.7rights.ru` is not ready, Security may approve a **temporary tunnel** so QA can run read-only GET matrix from outside localhost.

PR-GAP-001 may be **partially** verified; **full staging may still be required** for final closure.

## Constraints

| Rule | Detail |
|------|--------|
| Not production | Dev/local stack only |
| No production data | Local demo seed only |
| Read-only only | GET matrix — no writes |
| Secrets not stored | Tunnel tokens **never** in git/docs |
| Temporary URL | Changes on tunnel restart |
| Approval required | Security + Ops sign-off |

## When To Use

| Use | Do not use |
|-----|------------|
| Quick remote curl evidence | Long-term staging |
| Blocked on VPS lead time | Production-like load test |
| Pilot team has local Docker running | Compliance requires dedicated staging |

## Generic Flow (Placeholders Only)

1. Start local platform: `make platform-up-no-build` + `make health-check`
2. Enable auth-on via **gitignored** local override (see auth-on runbook) — **not committed**
3. Start tunnel tool with **token from secure store** (not documented here):
   - Cloudflare: `cloudflared tunnel --url http://localhost:8080` (example pattern only)
   - ngrok: `ngrok http 8080` (example pattern only)
4. Record **temporary** public URL in staging input form (item 1–2)
5. Run read-only matrix against `{tunnel-url}/api/v1/low-code`
6. Tear down tunnel and auth-on override after test

**Do not paste real tunnel tokens or authtoken commands into repo docs.**

## Security Rules

- Tunnel exposes local dev stack — restrict who can access URL
- Use demo tenant/users only
- No real customer data
- Rotate or discard tunnel after session
- Document decision: *temporary tunnel used — full staging TBD*

## Evidence Limitations

| Limitation | Impact |
|------------|--------|
| URL ephemeral | Evidence tied to date/session |
| Local identity DB | Not deployment-parity staging |
| PR-GAP-001 | May need **partial** + follow-up full staging |

## Approval Checklist

- [ ] Security approves temporary exposure
- [ ] Read-only permission **yes**
- [ ] No production data
- [ ] Tunnel token not committed
- [ ] Input form notes: `temporary tunnel — not full staging`

## Next Pack

If approved and tunnel URL provided:

**Low-code Pilot Week-3 Temporary Tunnel Auth-On Matrix Pack v0.1**

Preferred long-term path remains:

**Option A** — `LOW_CODE_PILOT_WEEK3_STAGING_DEPLOY_RUNBOOK_V0.1.md` → **Remote Auth-On Staging Repeat Pack v0.1**

Reference: `LOW_CODE_PILOT_WEEK3_REMOTE_STAGING_OPS_REQUEST_V0.1.md`
