# NLO-0.9C-I2C adversarial matrix

| Case | Expected | Proof |
| --- | --- | --- |
| manual capacity seed | 201 COMPLETED, server version | I2C-POST-01 |
| current-trip seed | 201, no ETA horizon | I2C-POST-02 |
| predicted capacity | 422, not reinterpreted | I2C-POST-03 |
| client pair / L1 / chain id | 422 | I2C-POST-04..06 |
| same key terminal replay | 200, zero recompute | I2C-POST-07, I2C-POST-15 |
| same key different fingerprint | 409 FINGERPRINT_CONFLICT | I2C-POST-08 |
| unexpired in-flight | 409 Retry-After 1 and same search id | I2C-POST-09-10 |
| expired matching claim | 200 FAILED CHAIN_INTERRUPTED, no recompute | I2C-POST-11 |
| omitted idempotency key | distinct server-owned searches | I2C-POST-12-13 |
| same key other tenant | independent search | I2C-POST-14 |
| GET completed / restart | same durable row | I2C-GET-01, I2C-GET-02 |
| ordinal order | increasing ordinals | I2C-GET-03 |
| non-completed candidates | empty list | I2C-GET-04, 09, 10 |
| expired PENDING/RUNNING GET | FAILED CHAIN_INTERRUPTED, zero provider calls | I2C-GET-05..07 |
| terminal immutability | identical GET body | I2C-GET-08 |
| foreign vs unknown | identical 404 | I2C-SEC-10-11 |
| private JSON | no owner token, fingerprint, tenant, shipper, contract | I2C-SEC-12 |
| unauthenticated / shipper / forwarder | 401 / 403 | I2C-SEC-01..03 |
| carrier admin, dispatcher, platform admin | allowed | I2C-SEC-04..06 |
| spoofed tenant/user | JWT identity wins | I2C-SEC-07, I2C-SEC-08 |
| foreign company | 403 before lookup | I2C-SEC-09 |
| startup and periodic sweep | expired claim terminalized, sweep stops on cancel | I2C recovery startup and periodic |
| dedicated CI | selector includes TestI2C_ | contract test + ci.yml |
