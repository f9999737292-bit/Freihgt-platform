# Customer Portals 0.1A — current state

Discovery only. No product behavior is changed by this pack.

```text
AGENT=F
STAGE=CUSTOMER-PORTALS-0.1A
BASE_SHA=67e72d37cdf8f658de04415b0ac2fc94e0dd408d
ORIGIN_MAIN=67e72d37cdf8f658de04415b0ac2fc94e0dd408d
BRANCH=discovery/customer-portals-current-state-v0.1a
WORKTREE=D:\Projects\freight-platform-wt\customer-portals-current-state-v0.1a
SHIPPER_PORTAL_PRESENT=YES
CARRIER_PORTAL_PRESENT=YES
CONSIGNEE_PORTAL_PRESENT=YES
SHIPPER_PORTAL_READINESS=2
CARRIER_PORTAL_READINESS=2
CONSIGNEE_PORTAL_READINESS=2
CUSTOMER_PORTALS_OVERALL_READINESS=2
SHARED_UI_REUSABLE=YES
SHARED_AUTH_PATTERN_FOUND=YES
SHARED_API_CLIENT_PATTERN_FOUND=NO
SHARED_I18N_REUSABLE=YES
SHIPPER_BUILD_READY=NO
CARRIER_BUILD_READY=NO
CONSIGNEE_BUILD_READY=NO
SHIPPER_CI_PRESENT=NO
CARRIER_CI_PRESENT=NO
CONSIGNEE_CI_PRESENT=NO
SHIPPER_STAGING_PACKAGED=NO
CARRIER_STAGING_PACKAGED=NO
CONSIGNEE_STAGING_PACKAGED=NO
PORTAL_MAY_DEFINE_BACKEND_BUSINESS_FACTS=NO
PORTAL_MAY_DEFINE_ANALYTICS_KPI=NO
PORTAL_MAY_BYPASS_GATEWAY=NO
TOTAL_GAPS=23
BLOCKING_GAPS=13
RECOMMENDED_FIRST_PORTAL=CARRIER
RECOMMENDED_SECOND_PORTAL=SHIPPER
RECOMMENDED_THIRD_PORTAL=CONSIGNEE
PRODUCT_CODE_CHANGED=NO
MIGRATION_CREATED=NO
STAGING_CHANGED=NO
DOCS_ONLY=YES
ARCHITECTURE_FROZEN=NO
READY_FOR_CUSTOMER_PORTALS_0_1B=YES
```

`SHARED_UI_REUSABLE=YES` means a shared package exists and the richer admin kit can be inventoried. It does not mean the three portals already share a production design system.

`SHARED_API_CLIENT_PATTERN_FOUND=NO` means no shared package exports an API client. The working client lives inside `apps/web-admin` and is copied in spirit by `apps/web-procurement`, not exported.

`SHIPPER_BUILD_READY=NO` (and the same for carrier and consignee) means this discovery did not execute `nuxt build`, and CI does not build these apps. Each app does declare `nuxt build` and is present in `pnpm-lock.yaml`.

## Documents

| File | Contents |
| --- | --- |
| [CUSTOMER_PORTALS_0_1A_CURRENT_STATE.md](CUSTOMER_PORTALS_0_1A_CURRENT_STATE.md) | App inventory, shared frontend, UX, build, scores, 0.1B inputs, product order |
| [CUSTOMER_PORTALS_0_1A_API_MAP.md](CUSTOMER_PORTALS_0_1A_API_MAP.md) | Gateway capabilities for shipper, carrier, and consignee |
| [CUSTOMER_PORTALS_0_1A_ROLE_MATRIX.md](CUSTOMER_PORTALS_0_1A_ROLE_MATRIX.md) | Seeded roles, auth flow, tenant source, portal access |
| [CUSTOMER_PORTALS_0_1A_GAPS.md](CUSTOMER_PORTALS_0_1A_GAPS.md) | Gap register |

0.1B freezes portal topology, auth, tenancy, and design-system policy. This pack does not choose them.
