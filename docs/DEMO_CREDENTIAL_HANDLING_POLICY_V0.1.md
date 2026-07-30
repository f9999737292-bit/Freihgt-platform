# Demo Credential Handling Policy v0.1

## Summary

Credential handling policy for future staging demo users.

Base commit: `47144b1`.

## Decision

```text
DEMO_CREDENTIAL_HANDLING_POLICY_APPROVED
```

## Rules

```text
1. No passwords in repository.
2. No passwords in docs.
3. No passwords in chat.
4. No tokens/JWT/cookies/localStorage in evidence.
5. No screenshots containing secrets.
6. No real employee/customer credentials.
7. No shared real accounts.
8. Use dedicated staging demo accounts only.
9. Rotate or disable demo credentials after the demo if required.
10. Production demo credentials are not approved.
```

## Approved Future User Aliases

| Alias                | Role            |
| -------------------- | --------------- |
| DEMO_PLATFORM_ADMIN  | PLATFORM_ADMIN  |
| DEMO_SHIPPER_ADMIN   | SHIPPER_ADMIN   |
| DEMO_CARRIER_ADMIN   | CARRIER_ADMIN   |
| DEMO_FINANCE_MANAGER | FINANCE_MANAGER |

## Future Execution Requirement

```text
Passwords may be generated only during a separately approved staging execution pack.
The final passwords must be delivered only through an approved secure owner channel and must not be recorded in repo/docs/chat.
```

## Forbidden

```text
Do not use real credentials.
Do not enter production credentials.
Do not create fake production sessions.
Do not record secrets in evidence.
```
