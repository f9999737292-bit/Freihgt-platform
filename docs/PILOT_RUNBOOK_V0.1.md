# Pilot Runbook v0.1

Companion to `PILOT_OPERATIONAL_READINESS_AND_HANDOFF_V0.1.md`. **Read-only reference — do not execute destructive steps without explicit authorized ops approval.**

---

## Severity Model

| Severity | Definition | Pilot action |
| --- | --- | --- |
| P1 CRITICAL | Platform unavailable, cross-tenant leak, auth bypass, DB unavailable, data corruption | STOP traffic; preserve evidence; escalate immediately |
| P2 HIGH | Control Tower unavailable, persistent 5xx, critical service degraded | Mitigate; authorized rollback assessment |
| P3 MEDIUM | Isolated workflow degradation, slow response, incomplete event timeline | Workaround; schedule fix |
| P4 LOW | Cosmetic, minor observability gap | Track in daily report |

---

## Auth Incident Runbook

| Scenario | Detection | First checks | Escalation | Recovery |
| --- | --- | --- | --- | --- |
| Login unavailable | 5xx/timeout on `POST /api/v1/auth/login` | `curl localhost:18080/health`; identity container logs; postgres | P2 → DevOps + Backend | Rollback if regression; no auth disable |
| Mass 401 | Spike in gateway 401 metrics/logs | Clock skew; JWT secret drift; identity health | P2 | Compare env; rollback if deploy-related |
| Mass 403 | RBAC spike | Role assignment changes; gateway auth middleware | P3 unless tenant leak suspected | Review identity roles |
| Token validation failure | Valid users rejected | JWT_SECRET consistency; identity logs | P2 | Authorized secret rotation (separate task) |
| Identity unavailable | `/ready` fails identity dep | Container status; DB connectivity | P2 | No restart without authorization |

---

## Control Tower Incident Runbook

| Scenario | Detection | First checks | Escalation |
| --- | --- | --- | --- |
| Summary 5xx | `GET /api/v1/control-tower/summary` fails | Gateway logs (`request_id`); shipment-service; read-model health | P2 |
| Summary timeout | High latency | Legacy aggregate duration metrics; downstream timeouts | P3 |
| Demo/fallback active | `fallbackUsed=true` or DEMO IDs in response | **P1 if real Pilot** — treat as data integrity | Security + Backend |
| No KPI data | Empty KPI with errors in logs | BFF downstream; tenant scope | P3 |
| Critical events unavailable | Panel empty + API errors | Same as summary | P3 |

**First checks (SSH + localhost):**

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18080/health
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18080/ready
docker logs --since 15m freight_api_gateway | tail -50   # redact before sharing
docker ps --filter name=freight_
```

---

## Database Incident Runbook

| Scenario | Detection | First checks | Escalation |
| --- | --- | --- | --- |
| DB unavailable | `/ready` fail; connection errors | `docker exec freight_postgres pg_isready`; disk space | P1 |
| Connection exhaustion | Pool timeout logs | Active connections; restart count (observe only) | P2 |
| Slow queries | Latency spike | Postgres logs; disk I/O | P3 |
| Disk full | `df -h` >90% | Log volume; backup dir size; image layers | P2 |
| Backup failure | Backup script fail / missing file | Last backup timestamp; `BACKUP_VERIFIED` flag | P2 |

**Forbidden without authorization:** `DROP`, `TRUNCATE`, restore execution, volume delete.

---

## Disk Full Runbook (Dedicated VM)

| Step | Action |
| --- | --- |
| 1 | Alert at **>80%** warning, **>90%** critical (proposed) |
| 2 | `df -h`; identify largest dirs under `/var/lib/docker`, `/protected/bintrans/backups`, logs |
| 3 | Safe: rotate/compress old **non-production** logs per policy |
| 4 | **Do not** run `docker system prune -a` automatically |
| 5 | Escalate to DevOps owner for approved cleanup |

---

## Cross-Tenant Security Incident

**Severity: P1 — immediate**

1. Stop Pilot traffic expansion if leak confirmed.
2. Preserve gateway/shipment logs (redacted exports only).
3. Collect `X-Request-ID` / `request_id` from affected requests.
4. Do **not** delete logs or DB rows.
5. Notify Security Owner + Technical Owner.
6. Identify affected tenants (redacted aliases in reports).
7. Revoke affected sessions if authorized.
8. Fix in separate controlled task — **not during incident doc update**.
9. Re-run tenant isolation gate before resume.

---

## Rollback Decision Tree

```text
Issue detected
        |
        v
Security or data integrity?
   YES -> STOP / P1 / rollback-or-disable Pilot
   NO
        |
        v
Safe mitigation without deploy?
   YES -> mitigate + observe (15/30/60 min)
   NO
        |
        v
Known-good image digest / SHA available?
   YES -> authorized rollback (see BINTRANS runbook)
   NO -> NO_GO escalation / incident commander
```

**Rollback references:** `docs/BINTRANS_DEDICATED_CONTROL_TOWER_STAGING_RUNBOOK.md`, digest-pinned images in protected `staging.env`.

---

## Backup / Restore Decision Tree

```text
Data loss or corruption suspected?
        |
        v
Preserve evidence (logs, request IDs, timestamps)
        |
        v
Identify restore point (BACKUP_PATH + SHA256 from staging.env)
        |
        v
Confirm target environment (STAGING only — never prod by mistake)
        |
        v
Explicit authorization (Rollback Owner + DBA Owner)
        |
        v
Execute documented restore procedure (separate authorized ops task)
        |
        v
Validate: schema_migrations, health, tenant isolation sample
```

**Restore test status:** `NOT_VERIFIED` as of v0.1 — must be scheduled before scale-up.

---

## Abort Criteria (During Pilot)

Stop or escalate immediately if:

- Cross-tenant leak confirmed
- Auth bypass confirmed
- Data corruption
- Persistent critical 5xx on Control Tower/auth
- Database outage
- Uncontrolled container restart loop
- Unexpected demo data in real Pilot responses
- Loss of auditability for Pilot writes
