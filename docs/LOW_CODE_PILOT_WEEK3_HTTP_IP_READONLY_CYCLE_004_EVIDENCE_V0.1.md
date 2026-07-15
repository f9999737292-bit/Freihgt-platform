# Low-code Pilot Week-3 HTTP IP Read-only Cycle 004 Evidence v0.1

## Summary

Controlled pilot read-only cycle 004 was executed over HTTP by IP without DNS on 2026-07-15.

Production-ready is not claimed.

Live machine-captured checks obtained via local PowerShell re-run (`_cycle004_out.txt` capture). Per pack rule, results are recorded as PASS.

Correlated prior machine-captured PASS on the same target remains in `LOW_CODE_PILOT_WEEK3_HTTP_IP_READONLY_CYCLE_003_EVIDENCE_V0.1.md`.

## Environment

```text
HTTP base: http://161.104.53.221
API base: http://161.104.53.221/api/v1
DNS: pending (staging.бинтранс.рф)
HTTPS: pending
Controlled pilot: active
Production-ready: not claimed
```

## Scope

Read-only checks only.

No POST, PUT, PATCH, DELETE, DNS changes, Certbot, Nginx changes, web-admin deploy, migrations, production data, or production-ready claim were executed.

## Pre-flight

```text
Recent HEAD: 15bc81a docs: migrate staging domain to Cyrillic .рф
git status --short: docs-only changes outside pack; _cycle004_out.txt untracked (not committed)
```

No blocking changes detected in apps/, services/, infrastructure/, migrations/, .env, docker configs, or secrets via repository inspection.

## Executed Checks

| Step | Command / endpoint |
| ---- | ------------------ |
| Health | GET `http://161.104.53.221/health` |
| Demo seed verify | `.\scripts\dev\Verify-StagingDemoSeed.ps1 -Base "http://161.104.53.221"` |
| Low-code runtime | GET `/api/v1/low-code/form-templates/active?entity_type=TRANSPORT_ORDER` with `X-Tenant-ID: 74519f22-ff9b-4a8b-8fff-a958c689682f` |

Note: `/api/v1/low-code/runtime/active-templates` is not the project endpoint; use `form-templates/active` per docs/scripts.

## Results

| Check                 | Result | Evidence                                              |
| --------------------- | ------ | ----------------------------------------------------- |
| HTTP health by IP     | PASS   | HEALTH=200 — machine-captured                         |
| Demo seed verify      | PASS   | `Verify-StagingDemoSeed.ps1` — machine-captured         |
| VFY-001               | PASS   | health=200 — machine-captured                         |
| VFY-002               | PASS   | StatusCode=200, pattern=DEMO-TO — script label CHECK  |
| VFY-003               | PASS   | StatusCode=200, pattern=DEMO-SH — script label CHECK  |
| VFY-004               | PASS   | StatusCode=200, pattern=DEMO-BR — script label CHECK  |
| VFY-005               | PASS   | StatusCode=200 — machine-captured                     |
| VFY-006               | PASS   | operator-confirmed (prior seed evidence; not in .ps1) |
| Low-code runtime read | PASS   | active-templates=200 — machine-captured               |
| Writes executed       | NO     | required                                              |
| Secrets captured      | NO     | required                                              |

## Results (detail)

```text
HTTP health by IP: PASS — HEALTH=200
Demo seed verify: PASS — machine-captured
VFY-001: PASS — health=200
VFY-002: PASS — StatusCode=200, pattern=DEMO-TO, script label CHECK
VFY-003: PASS — StatusCode=200, pattern=DEMO-SH, script label CHECK
VFY-004: PASS — StatusCode=200, pattern=DEMO-BR, script label CHECK
VFY-005: PASS — StatusCode=200
VFY-006: PASS — operator-confirmed, prior evidence
Low-code runtime read: PASS — active-templates=200
Writes executed: NO
Secrets captured: NO
Production-ready: not claimed
```

## Machine-captured Output

Source: local operator re-run captured to `_cycle004_out.txt` (not committed).

Timestamp: 2026-07-15 13:55:47

Sanitized output:

```text
=== HTTP IP READONLY CYCLE 004 ===
Base: http://161.104.53.221

=== HEALTH ===
HEALTH=200

=== VERIFY ===
=== Verify-StagingDemoSeed ===
PASS VFY-001 health=200
CHECK VFY-002 StatusCode=200 pattern=DEMO-TO
CHECK VFY-003 StatusCode=200 pattern=DEMO-SH
CHECK VFY-004 StatusCode=200 pattern=DEMO-BR
PASS VFY-005 StatusCode=200

=== RUNTIME ===
low-code runtime active-templates=200

=== DONE ===
```

## Decision

```text
HTTP_IP_READONLY_CYCLE_004_PASS
```

STG-LIM-001..004 remain open. STG-LIM-005/006 not reopened.

## Limitations

```text
STG-LIM-001: OPEN — DNS pending (staging.бинтранс.рф)
STG-LIM-002: OPEN — HTTPS pending DNS and SSH
STG-LIM-003: OPEN — SSH SG /32 verification open
STG-LIM-004: OPEN — web-admin deploy execution pending
STG-LIM-005: CLOSED
STG-LIM-006: CLOSED
```

## Production-ready

```text
not claimed
```

## Next Step

```text
DNS A-record staging.бинтранс.рф -> 161.104.53.221
```

Alternative if DNS remains pending:

```text
Continue read-only controlled pilot checks by IP.
```
