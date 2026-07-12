# Low-code Pilot Week-3 Staging Limitations Tracker v0.1

## Summary

Tracks open staging limitations separately from production readiness gaps.

Production-ready claimed:

```text
no
```

Controlled pilot:

```text
active
```

## Limitations

| ID | Limitation | Status | Decision | Priority |
| -- | ---------- | ------ | -------- | -------- |
| STG-LIM-001 | HTTP-only IP access | OPEN_DNS_PENDING_BINTRANS_DOMAIN | BINTRANS_STAGING_DOMAIN_SELECTED_DNS_PENDING | P1 |
| STG-LIM-002 | HTTPS / Certbot not configured | OPEN_HTTPS_PENDING_DNS_AND_SSH | HTTPS_PREP_PENDING_BINTRANS_DNS_AND_SERVER_ACCESS | P1 |
| STG-LIM-003 | SSH 22 Selectel Security Group /32 restriction | OPEN | SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_FAILED_PORT_22_STILL_PUBLIC | P0 |
| STG-LIM-004 | Web-admin UI not deployed | OPEN | STAGING_LIMITATIONS_REVIEWED | P2 |
| STG-LIM-005 | Full demo UI seed-data not executed | OPEN | STAGING_LIMITATIONS_REVIEWED | P3 |
| STG-LIM-006 | seed-lowcode-demo custom field values skipped | OPEN | STAGING_LIMITATIONS_REVIEWED | P3 |

## STG-LIM-001 Detail

Status:

```text
OPEN_DNS_PENDING_BINTRANS_DOMAIN
```

Decision:

```text
BINTRANS_STAGING_DOMAIN_SELECTED_DNS_PENDING
```

Domain:

```text
staging.bintrans.ru
```

Fallback domain:

```text
pilot.bintrans.ru
```

Target IP:

```text
161.104.53.221
```

Deprecated for this path:

```text
staging.7rights.ru
pilot.7rights.ru
```

Current access:

```text
http://161.104.53.221 — HTTP-only IP
```

DNS configured:

```text
no — pending operator action at registrar
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DOMAIN_DECISION_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md
```

## STG-LIM-002 Detail

Status:

```text
OPEN_HTTPS_PENDING_DNS_AND_SSH
```

Decision:

```text
HTTPS_PREP_PENDING_BINTRANS_DNS_AND_SERVER_ACCESS
```

Domain:

```text
staging.bintrans.ru
```

HTTPS execution:

```text
blocked — docs-only until DNS resolves and SSH available
```

Certbot executed:

```text
no
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md
```

## STG-LIM-003 Detail

Trusted SSH path:

```text
fail — banner exchange timeout (retry #5, 2026-07-12)
```

API health:

```text
200
```

Runtime:

```text
not verified — SSH unavailable on retry #5
```

UFW 5432/6379/internal ports:

```text
not verified — SSH unavailable on retry #5
```

Selectel SG /32 confirmed:

```text
no — external non-trusted scan retry #5: 5/5 nodes TCP 22 connect success
```

Non-trusted SSH rejection:

```text
fail
```

Closure candidate:

```text
no — STG_LIM_003_REMAINS_OPEN
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_RETRY_5_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STG_LIM_003_CLOSURE_CANDIDATE_NOTE_V0.1.md
```

## Production-ready Status

```text
not claimed
```

## Next Recommended Event

```text
operator creates DNS A-record: staging.bintrans.ru -> 161.104.53.221
operator applies Selectel SG SSH /32 restriction in control panel, then re-run non-trusted rejection test
next technical pack: Bintrans HTTPS / Certbot Preparation Pack v0.1 (after DNS + SSH ready)
```
