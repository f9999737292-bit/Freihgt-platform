# Test strategy

No production tests are added in this freeze because no production implementation is authorized. The future suite should include:

| Layer | Intent |
|-------|--------|
| Unit | Hard constraints, score components, state transitions, visibility filters |
| Contract | Draft OpenAPI versus handlers, event envelope versus ADR-EDO-006 |
| Integration | Publication then search inside one tenant; gateway header spoof rejected |
| Property-based | Residual capacity never negative; hard reject never receives a positive offer |
| Solver correctness | Golden SCN outputs for ranking order and reject reasons, deterministic across runs |
| Tenant isolation | Foreign shipment id, foreign opportunity id, cross-tenant SQL absent |
| Security | Threat-model cases: replay, double accept, spoofed carrier, anonymized fields |
| Performance | Scale classes SMALL through VERY_LARGE, measured not assumed |
| Load | Concurrent jobs and reservation conflicts |
| Determinism | Same snapshot, same score |
| Reoptimization | SCN-11 moves a chain to `AT_RISK` and does not cancel the shipment |

## Golden scenarios

| Id | Assert |
|----|--------|
| SCN-01 | Prediction uses ETA; missing ETA yields no offer |
| SCN-02 | 35 km proximity can candidate; executable km require the routing port |
| SCN-03 | Residual 9 t and 38 m³; overflow is `HARD_REJECT` |
| SCN-04 | Shipper A payload has no B or C rate, contract, customer, or internal id |
| SCN-05 | Stop sequence Moscow, Khimki, Podolsk, Saint Petersburg |
| SCN-06 | Multi-drop sequence; placement `NOT_EVALUATED` or a real sequence result |
| SCN-07 | Closed depot loop constraints present |
| SCN-08 | Moscow profile id; stale rules reject |
| SCN-09 | Saint Petersburg profile id; same code path |
| SCN-10 | Three-leg chain and slack stored |
| SCN-11 | Late first leg marks chain at risk |
| SCN-12 | 100 orders and 30 vehicles stay inside one tenant and do not publish marketplace rows |

Fixtures must be synthetic. They must not copy live customer data.
