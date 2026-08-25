# Low-code Pilot Week-3 Selectel Runtime Preparation Plan v0.1

## Summary

This plan defines the runtime preparation steps for the Selectel staging server.

This plan is not execution evidence and does not close PR-GAP-001.

## Preparation Steps

1. Harden Selectel Security Group / firewall:

   * restrict SSH 22 by trusted IP
   * close PostgreSQL 5432 externally
   * close Redis 6379 externally
   * keep 80/443 open

2. Configure domain:

   * only if a BINTRANS staging DNS endpoint is verified per `docs/LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md`
   * do **not** use invalid legacy external domain references — unrelated external site; never valid BINTRANS endpoints
   * interim access may use verified IP `161.104.53.221` until DNS is confirmed

3. Install base packages:

   * Docker
   * Docker Compose
   * Git
   * Nginx or reverse proxy
   * Certbot / Let's Encrypt if HTTPS is configured directly on VM

4. Clone repository:

   * branch main
   * no secrets in repo
   * staging .env prepared outside git

5. Configure staging runtime:

   * LOW_CODE_ADMIN_AUTH_ENABLED=true
   * no production data
   * no secrets in docs

6. Start platform:

   * Docker Compose or project-approved startup method

7. Prepare read-only GET checks:

   * no POST/PUT/PATCH/DELETE
   * no migration execute
   * no template publish/import/clone
   * sanitized evidence only

## Execution Approval Required

Do not execute SSH, remote Docker commands, deploy, or remote API checks without explicit user approval.

## Decision

```text
SELECTEL_RUNTIME_PREPARATION_PLAN_CREATED_PENDING_HARDENING
```

## PR-GAP-001 Status

```text
BLOCKED_WAITING_FOR_STAGING_HARDENING_AND_RUNTIME_PREPARATION
```
