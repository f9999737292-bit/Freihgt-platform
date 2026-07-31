# Staging Backend DB Isolation Port and Env Policy v0.1

## Summary

Port and environment policy for future isolated staging backend/DB execution.

Base commit: `570c3c4`.

## Decision

```text
STAGING_BACKEND_DB_ISOLATION_PORT_ENV_POLICY_APPROVED
```

## Port Policy

| Purpose                | Port / Binding                               |
| ---------------------- | -------------------------------------------- |
| production API gateway | 127.0.0.1:8080                               |
| staging API gateway    | 127.0.0.1:18080                              |
| staging service ports  | internal Docker network only unless required |
| staging Postgres       | internal Docker network only                 |
| public ports           | no new public ports                          |

## Env Policy

```text
Staging env must be server-only.
Staging env must not be committed.
Staging env values must not be printed in logs/evidence.
Production env must not be modified by staging execution.
```

## Forbidden

```text
No .env.staging in repository.
No passwords in docs/chat.
No DB URLs with passwords in evidence.
No JWT/tokens/cookies/localStorage in evidence.
No public Postgres bind.
```

## Future Execution Checks

```text
1. Check port 18080 availability.
2. Confirm staging env file path.
3. Confirm no env values printed.
4. Confirm Nginx staging proxy targets 127.0.0.1:18080 only.
5. Confirm production proxy remains 127.0.0.1:8080.
```
