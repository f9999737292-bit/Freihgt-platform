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
| STG-LIM-001 | HTTP-only IP access | OPEN | STAGING_LIMITATIONS_REVIEWED | P1 |
| STG-LIM-002 | HTTPS / Certbot not configured | OPEN | STAGING_LIMITATIONS_REVIEWED | P1 |
| STG-LIM-003 | SSH 22 Selectel Security Group /32 restriction | OPEN — retry #4: trusted SSH fail; external scan 5/5 connect | SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_FAILED_PORT_22_STILL_PUBLIC | P0 |
| STG-LIM-004 | Web-admin UI not deployed | OPEN | STAGING_LIMITATIONS_REVIEWED | P2 |
| STG-LIM-005 | Full demo UI seed-data not executed | OPEN | STAGING_LIMITATIONS_REVIEWED | P3 |
| STG-LIM-006 | seed-lowcode-demo custom field values skipped | OPEN | STAGING_LIMITATIONS_REVIEWED | P3 |

## STG-LIM-003 Detail

Trusted SSH path:

```text
fail — banner exchange timeout (retry #4, 2026-07-12)
```

API health:

```text
200
```

Runtime:

```text
not verified — SSH unavailable on retry #4
```

UFW 5432/6379/internal ports:

```text
not verified — SSH unavailable on retry #4
```

Selectel SG /32 confirmed:

```text
no — external non-trusted scan retry #4: 5/5 nodes TCP 22 connect success
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
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_NON_TRUSTED_REJECTION_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STG_LIM_003_CLOSURE_CANDIDATE_NOTE_V0.1.md
```

## Production-ready Status

```text
not claimed
```

## Next Recommended Event

```text
operator applies Selectel SG SSH /32 restriction in control panel, then re-run non-trusted rejection test
```
