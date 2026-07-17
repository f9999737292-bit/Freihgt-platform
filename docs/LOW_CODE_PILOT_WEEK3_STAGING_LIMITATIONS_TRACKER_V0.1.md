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
| STG-LIM-001 | HTTP-only IP access | **CLOSED** | STG-LIM-001_CLOSED_DNS_VERIFIED | P1 |
| STG-LIM-002 | HTTPS / Certbot not configured | OPEN_HTTPS_PENDING_DNS_AND_SSH | CYRILLIC_RF_DOMAIN_SELECTED_DNS_PENDING | P1 |
| STG-LIM-003 | SSH 22 Selectel Security Group /32 restriction | READY_FOR_CLOSURE_REVIEW | SELECTEL_SSH_SG_RETRY_007_PASS | P0 |
| STG-LIM-004 | Web-admin UI not deployed | OPEN_WEB_ADMIN_DEPLOY_PLAN_CREATED | WEB_ADMIN_DEPLOY_PLAN_CREATED_PENDING_EXECUTION | P2 |
| STG-LIM-005 | Full demo UI seed-data not executed | **CLOSED** | DEMO_SEED_EXECUTION_OPERATOR_CONFIRMED_COMPLETE | P3 |
| STG-LIM-006 | seed-lowcode-demo custom field values skipped | **CLOSED** | DEMO_SEED_EXECUTION_OPERATOR_CONFIRMED_COMPLETE | P3 |

## STG-LIM-001 Detail

Status:

```text
CLOSED
```

Decision:

```text
STG-LIM-001_CLOSED_DNS_VERIFIED
```

Domain display:

```text
staging.бинтранс.рф
```

Domain technical:

```text
staging.xn--80abvubqje.xn--p1ai
```

Target IP:

```text
161.104.53.221
```

DNS delegation:

```text
PASS
```

A-record:

```text
PASS
```

HTTP health:

```text
PASS 200
```

Closure note:

```text
docs/LOW_CODE_PILOT_WEEK3_STG_LIM_001_DNS_CLOSURE_NOTE_V0.1.md
```

Previous active domain (deprecated):

```text
staging.bintrans.ru
```

Deprecated for this path:

```text
staging.bintrans.ru
pilot.bintrans.ru
staging.7rights.ru
pilot.7rights.ru
```

Current access:

```text
http://staging.бинтранс.рф — HTTP by domain (verified)
http://161.104.53.221 — HTTP by IP
```

DNS configured:

```text
yes — verified 2026-07-17 (retry); A-record staging -> 161.104.53.221; delegation ns*-l2/cloud.nic.ru
```

Verification:

```text
2026-07-17 initial — FAIL (NXDOMAIN)
2026-07-17 retry — PASS: delegation PASS, A-record PASS, domain /health 200
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_CYRILLIC_RF_DOMAIN_MIGRATION_DECISION_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_CYRILLIC_RF_DNS_VERIFICATION_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STG_LIM_001_DNS_CLOSURE_NOTE_V0.1.md
```

## STG-LIM-002 Detail

Status:

```text
OPEN_HTTPS_PENDING_DNS_AND_SSH
```

Decision:

```text
CYRILLIC_RF_DOMAIN_SELECTED_DNS_PENDING
```

Domain display:

```text
staging.бинтранс.рф
```

Domain technical:

```text
staging.xn--80abvubqje.xn--p1ai
```

HTTPS execution:

```text
blocked — docs-only prep pack created; execution pending DNS + operator approval
```

Certbot executed:

```text
no
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_HTTPS_CERTBOT_PREPARATION_PACK_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_BINTRANS_HTTPS_CERTBOT_CHECKLIST_V0.1.md
```

## STG-LIM-003 Detail

Status:

```text
READY_FOR_CLOSURE_REVIEW
```

Decision:

```text
SELECTEL_SSH_SG_RETRY_007_PASS
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_RETRY_007_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_RETRY_6_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STG_LIM_003_CLOSURE_CANDIDATE_NOTE_V0.1.md
```

Trusted TCP 22:

```text
PASS — TcpTestSucceeded: True (retry #7, 2026-07-17)
```

Trusted SSH read-only:

```text
PASS — root@gpt-docker; UFW/docker read-only (retry #7)
```

Non-trusted TCP 22:

```text
PASS — 0/5 external nodes connect; 5/5 timeout/denied (check-host.net retry #7)
```

HTTP health:

```text
PASS 200 — staging.xn--80abvubqje.xn--p1ai/health
```

Trusted SSH path (historical):

```text
pass — SSH_TRUSTED_PATH_OK (retry #6, 2026-07-12)
```

API health:

```text
200
```

Runtime:

```text
10 containers healthy
```

UFW 5432/6379/internal ports:

```text
deny
```

Selectel SG /32 confirmed:

```text
yes — external non-trusted scan retry #7: 0/5 nodes TCP 22 connect; 5/5 timeout/denied
```

Non-trusted SSH rejection:

```text
pass — retry #7 external scan 0/5 connect
```

Closure candidate:

```text
yes — STG-LIM-003_READY_FOR_CLOSURE_REVIEW
```

## STG-LIM-004 Detail

Status:

```text
OPEN_WEB_ADMIN_DEPLOY_PLAN_CREATED
```

Decision:

```text
WEB_ADMIN_DEPLOY_PLAN_CREATED_PENDING_EXECUTION
```

Web-admin deployed:

```text
no
```

API read-only smoke:

```text
pass — STAGING_API_READ_ONLY_SMOKE_PASS
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_WEB_ADMIN_DEPLOY_PLAN_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_WEB_ADMIN_DEPLOY_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_STAGING_API_READ_ONLY_SMOKE_EVIDENCE_V0.1.md
```

## STG-LIM-005 Detail

Status:

```text
CLOSED
```

Decision:

```text
DEMO_SEED_EXECUTION_OPERATOR_CONFIRMED_COMPLETE
```

Seed executed:

```text
yes — operator confirmed «seed выполнен» on 2026-07-13
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_PLAN_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_VERIFICATION_EVIDENCE_V0.1.md
```

## STG-LIM-006 Detail

Status:

```text
CLOSED
```

Decision:

```text
DEMO_SEED_EXECUTION_OPERATOR_CONFIRMED_COMPLETE
```

Custom field values seeded:

```text
yes — operator confirmed «seed выполнен» on 2026-07-13
```

Evidence:

```text
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_PLAN_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_CHECKLIST_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_EVIDENCE_V0.1.md
docs/LOW_CODE_PILOT_WEEK3_DEMO_SEED_EXECUTION_VERIFICATION_EVIDENCE_V0.1.md
```

## Production-ready Status

```text
not claimed
```

## Next Recommended Event

```text
STG-LIM-003 closure review (retry #7 PASS)
STG-LIM-002: OPEN — HTTPS / Certbot pending (explicit approval required)
Web-admin Deploy Execution Pack v0.1 (operator approval required)
```
