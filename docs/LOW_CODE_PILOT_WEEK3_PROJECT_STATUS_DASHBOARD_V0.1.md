# Low-code Pilot Week-3 Project Status Dashboard v0.1

## Current Summary

* Controlled pilot: active
* Production-ready: not claimed
* PR-GAP-001: CLOSED
* Production gaps open: 0
* Staging IP: 161.104.53.221
* Current HTTP API: http://161.104.53.221
* Selected domain: staging.bintrans.ru
* Fallback domain: pilot.bintrans.ru
* DNS: pending operator action
* HTTPS: pending (prep docs created)
* Web-admin deploy: plan created, execution pending
* Last commit: 57e5da6

## Completed Commits

Recent important commits (from `git log`):

| Hash | Message |
| ---- | ------- |
| c318e47 | docs: add web-admin deploy plan and staging API smoke |
| ba3577a | docs: prepare Bintrans HTTPS Certbot staging pack |
| 3e39256 | docs: record week 3 Selectel SSH SG retry 6 evidence |
| 799217a | docs: select Bintrans staging domain |
| bda347a | docs: record week 3 Selectel SSH SG retry 5 failure |
| 1dcd6d0 | docs: close week 3 PR-GAP-001 with owner approval |
| 663983a | docs: record week 3 remote auth-on staging verification |

## Production Gaps

* PR-GAP-001: CLOSED
* Open production gaps: 0
* Production-ready: not claimed
* Decision: NOT_PRODUCTION_READY_CONTROLLED_PILOT_ONLY

## Staging Limitations

| ID | Status | Meaning | Next action |
| -- | ------ | ------- | ----------- |
| STG-LIM-001 | OPEN_DNS_PENDING_BINTRANS_DOMAIN | DNS for staging.bintrans.ru pending | operator creates A-record |
| STG-LIM-002 | OPEN_HTTPS_PENDING_DNS_AND_SSH | HTTPS pending; prep pack created | wait for DNS + SSH approval |
| STG-LIM-003 | OPEN | SSH SG /32 not verified; external scan deferred per operator | fix Selectel SG; re-run verification when ready |
| STG-LIM-004 | PLAN_CREATED_EXECUTION_PENDING | web-admin deploy plan exists | deploy after operator approval |
| STG-LIM-005 | OPEN_DEMO_SEED_PLAN_CREATED | demo seed plan exists | execute after approval |
| STG-LIM-006 | OPEN_DEMO_SEED_PLAN_CREATED | custom field values plan exists | after seed-demo-data |

## What Works Now

* HTTP API by IP — http://161.104.53.221
* `/health` returns 200
* Remote Auth-On verification passed (PR-GAP-001 closed)
* Low-code auth-on matrix passed on staging
* Staging API read-only smoke passed
* Trusted SSH path available
* Runtime: 10 containers healthy (last verified)
* Controlled pilot read-only test execution pass (CP-RO-001..008)
* Demo seed plan exists (STG-LIM-005/006)
* Web-admin deploy plan exists
* HTTPS prep docs exist
* Bintrans domain decision exists
* Bintrans DNS checklist exists

## What Is Blocked

* HTTPS / Certbot execution (DNS pending)
* Web-admin deploy execution (approval + DNS/SSH readiness)
* STG-LIM-003 closure (external port 22 verification deferred)
* Production-ready claim (staging limitations open)

## Next Operator Actions

1. Create DNS A-record:

   ```text
   staging.bintrans.ru -> 161.104.53.221
   ```

2. Fix Selectel Security Group (if re-verification requested later):

   * TCP 22 only trusted operator IP /32
   * remove 0.0.0.0/0 and ::/0 for TCP 22
   * verify allowed address pairs on port (no 0.0.0.0/0 bypass)

3. After DNS / SG readiness:

   * DNS verification: `Resolve-DnsName staging.bintrans.ru`
   * HTTP domain check: `http://staging.bintrans.ru/health`
   * optional: re-run external port 22 verification

## Next Technical Packs

* Selectel SSH SG Re-verification + Bintrans DNS Verification Pack v0.1
* Bintrans HTTPS / Certbot Execution Pack v0.1
* Web-admin Deploy Execution Pack v0.1

## Forbidden Until Ready

* production-ready claim
* Certbot before DNS resolves
* web-admin deploy execution before operator approval
* staging writes without explicit approval
* secrets / JWT / tokens / .env values in docs
* API POST / PUT / PATCH / DELETE on staging without approval

## Key Evidence Docs

| Topic | Doc |
| ----- | --- |
| Dashboard | `LOW_CODE_PILOT_WEEK3_PROJECT_STATUS_DASHBOARD_V0.1.md` |
| Limitations | `LOW_CODE_PILOT_WEEK3_STAGING_LIMITATIONS_TRACKER_V0.1.md` |
| Bintrans domain | `LOW_CODE_PILOT_WEEK3_BINTRANS_DOMAIN_DECISION_V0.1.md` |
| DNS checklist | `LOW_CODE_PILOT_WEEK3_BINTRANS_DNS_CHECKLIST_V0.1.md` |
| HTTPS prep | `LOW_CODE_PILOT_WEEK3_BINTRANS_HTTPS_CERTBOT_PREPARATION_PACK_V0.1.md` |
| Web-admin plan | `LOW_CODE_PILOT_WEEK3_WEB_ADMIN_DEPLOY_PLAN_V0.1.md` |
| API smoke | `LOW_CODE_PILOT_WEEK3_STAGING_API_READ_ONLY_SMOKE_EVIDENCE_V0.1.md` |
| Demo seed plan | `LOW_CODE_PILOT_WEEK3_DEMO_SEED_PLAN_V0.1.md` |
| Controlled pilot RO execution | `LOW_CODE_PILOT_WEEK3_CONTROLLED_PILOT_READ_ONLY_TEST_EXECUTION_EVIDENCE_V0.1.md` |
| Remote auth-on | `LOW_CODE_PILOT_WEEK3_REMOTE_AUTH_ON_STAGING_REPEAT_EVIDENCE_V0.1.md` |
| SSH SG retry 6 | `LOW_CODE_PILOT_WEEK3_SELECTEL_SSH_SG_POST_PANEL_REVERIFICATION_RETRY_6_EVIDENCE_V0.1.md` |

## Production-ready

```text
not claimed
```
