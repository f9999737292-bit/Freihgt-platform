# RBAC Role Navigation Acceptance Checklist v0.1

## Summary

Acceptance checklist for future RBAC and role navigation implementation.

## Decision

```text
RBAC_ROLE_NAVIGATION_ACCEPTANCE_CHECKLIST_CREATED
```

## P0 Acceptance Criteria

| ID | Criterion | Required |
| --- | --- | --- |
| RBAC-AC-001 | Canonical roles defined in frontend helper | yes |
| RBAC-AC-002 | Identity roles mapped to product roles | yes |
| RBAC-AC-003 | Sidebar filters items by role/permission | yes |
| RBAC-AC-004 | Admin sees full navigation | yes |
| RBAC-AC-005 | Non-admin roles do not see low-code unless allowed | yes |
| RBAC-AC-006 | Carrier first screen can be /shipments | yes |
| RBAC-AC-007 | Finance first screen can be /billing-registers | yes |
| RBAC-AC-008 | Demo login defaults removed or dev-only guarded | yes |
| RBAC-AC-009 | Access denied state exists or is explicitly planned | yes |
| RBAC-AC-010 | Frontend build passes | yes |

## Safety Acceptance Criteria

| ID | Criterion | Required |
| --- | --- | --- |
| SAFE-001 | No backend code changed unless explicitly approved | yes |
| SAFE-002 | No API contract changes | yes |
| SAFE-003 | No database migrations | yes |
| SAFE-004 | No production deploy without approval | yes |
| SAFE-005 | No secrets captured | yes |
| SAFE-006 | No `.env` values committed | yes |

## Manual Review Checklist

```text
1. Login page opens.
2. Dashboard opens.
3. Sidebar displays role-specific items.
4. Admin menu remains complete.
5. Non-admin menu is reduced.
6. Direct restricted route shows safe access denied state.
7. No demo credentials are visible in production build.
8. Carrier lands on /shipments after login.
9. Finance lands on /billing-registers after login.
10. Low-code admin routes hidden from non-admin roles.
```

## Role Spot-check Matrix

| Role | Sidebar reduced? | First screen | Low-code hidden? |
| ---- | ---------------- | ------------ | ---------------- |
| admin | no (full) | /dashboard | no |
| shipper | yes | /dashboard | yes |
| carrier | yes | /shipments | yes |
| forwarder | yes | /freight-requests | yes |
| consignee | yes (minimal) | /shipments | yes |
| finance | yes | /billing-registers | yes/limit |
| procurement | yes | /freight-requests | yes/limit |

## Next

```text
RBAC_AND_ROLE_NAVIGATION_IMPLEMENTATION_APPROVAL_PACK v0.1
```
