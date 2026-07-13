# Low-code Pilot Week-3 Controlled Pilot Test Plan v0.1

## Summary

This document defines what can be tested in the controlled pilot / staging environment.

Production-ready is not claimed.

## Environment

Staging API:

```text
http://161.104.53.221/api/v1
```

Selected future domain:

```text
staging.bintrans.ru
```

DNS:

```text
pending
```

HTTPS:

```text
pending
```

Web-admin UI:

```text
deploy plan created, execution pending
```

Controlled pilot:

```text
active
```

Production-ready:

```text
not claimed
```

## Current Testing Scope

Allowed now:

* API health checks
* read-only GET checks
* auth-on verification
* role access verification
* tenant isolation verification
* low-code runtime templates verification
* audit read behavior verification
* documentation review

Not allowed without separate approval:

* staging write tests
* POST/PUT/PATCH/DELETE API calls
* migration execute
* template publish/import/clone
* production data use
* real customer onboarding
* production-ready claim

## Demo Users Required

| Role              | Email                                                       | Status    |
| ----------------- | ----------------------------------------------------------- | --------- |
| PLATFORM_ADMIN    | [admin@bintrans.local](mailto:admin@bintrans.local)         | available |
| SHIPPER_LOGIST    | [shipper@bintrans.local](mailto:shipper@bintrans.local)     | available |
| CARRIER_MANAGER   | [carrier@bintrans.local](mailto:carrier@bintrans.local)     | needed    |
| FORWARDER_MANAGER | [forwarder@bintrans.local](mailto:forwarder@bintrans.local) | needed    |
| CONSIGNEE_VIEWER  | [consignee@bintrans.local](mailto:consignee@bintrans.local) | needed    |

## Demo Companies Required

| Company                  | Type      | Status    |
| ------------------------ | --------- | --------- |
| ООО Bintrans Dev Tenant  | tenant    | available |
| ООО Грузовладелец Север  | shipper   | available |
| ООО Перевозчик Тест      | carrier   | needed    |
| ООО Экспедитор Тест      | forwarder | needed    |
| ООО Грузополучатель Тест | consignee | needed    |

## Read-only Test Matrix

| Test ID   | Scenario                            | Method | Expected                            |
| --------- | ----------------------------------- | ------ | ----------------------------------- |
| CP-RO-001 | API health                          | GET    | 200                                 |
| CP-RO-002 | low-code runtime active templates   | GET    | 200                                 |
| CP-RO-003 | admin access to admin templates     | GET    | 200                                 |
| CP-RO-004 | non-admin denied on admin templates | GET    | 403                                 |
| CP-RO-005 | anonymous denied on admin templates | GET    | 401/403                             |
| CP-RO-006 | wrong tenant rejected               | GET    | 403/404/empty                       |
| CP-RO-007 | audit events read behavior          | GET    | 200 or expected restricted response |
| CP-RO-008 | service health summary              | GET    | all expected services healthy       |

## Write Test Matrix — Planned Only

Write tests require separate explicit approval:

```text
разрешаю staging write smoke-test на demo data
```

| Test ID   | Scenario                         | Method    | Expected       |
| --------- | -------------------------------- | --------- | -------------- |
| CP-WR-001 | create demo transport order      | POST      | 201/200        |
| CP-WR-002 | create demo RFQ / mini-tender    | POST      | 201/200        |
| CP-WR-003 | create demo shipment             | POST      | 201/200        |
| CP-WR-004 | update shipment status           | PATCH/PUT | status updated |
| CP-WR-005 | create demo document record      | POST      | 201/200        |
| CP-WR-006 | create billing register          | POST      | 201/200        |
| CP-WR-007 | verify audit events after writes | GET       | events visible |

## Pilot Business Scenarios

| Scenario                          | Status             |
| --------------------------------- | ------------------ |
| Shipper creates transport request | planned            |
| Carrier receives offer            | planned            |
| Forwarder participates            | planned            |
| Shipment lifecycle                | planned            |
| Documents lifecycle               | planned            |
| Billing register lifecycle        | planned            |
| Low-code custom fields visibility | planned            |
| Audit and access control          | verified (read-only) |

## Reporting Template

For each test, record:

```text
Test ID:
Actor:
Endpoint / UI page:
Method:
Expected:
Actual:
Result: PASS/FAIL/BLOCKED
Evidence:
Secrets captured: no
Writes executed: yes/no
Notes:
```

## Open Limitations

| Limitation                              | Status            |
| --------------------------------------- | ----------------- |
| STG-LIM-001 DNS                         | open              |
| STG-LIM-002 HTTPS                       | open              |
| STG-LIM-003 SSH SG /32                  | open              |
| STG-LIM-004 web-admin UI deploy         | pending execution |
| STG-LIM-005 demo seed-data              | plan created, execution pending |
| STG-LIM-006 low-code custom fields demo | plan created, execution pending |

## Next Recommended Events

1. DNS A-record: staging.bintrans.ru -> 161.104.53.221
2. Selectel SG retry #6
3. HTTPS / Certbot execution after DNS + SSH
4. Web-admin deploy execution
5. Controlled pilot write smoke-test approval
