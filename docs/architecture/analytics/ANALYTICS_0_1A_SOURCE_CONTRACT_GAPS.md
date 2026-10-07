# Analytics-0.1A source contract gaps

Missing facts are assigned to the domain that must emit them. Analytics (Agent E) does not invent the fact inside another service.

`REQUIRES_PRODUCT_CHANGE=YES` means a later source-domain change. This document does not authorize that change.

Priority:

- **P0** blocks a first Operations Analytics release that includes the named KPI.
- **P1** blocks carrier, procurement, document-trust, or finance analytics.
- **P2** blocks network, executive network metrics, or historical network analytics.

## Register

### GAP-C-001

```
GAP_ID=GAP-C-001
KPI_IDS_AFFECTED=OPS_OTIF, CAR_OTIF, EXEC_OTIF
MISSING_FACT=Per-delivery in-full assertion (accepted quantity versus shipped quantity) for every completed delivery, including deliveries with zero rejection
WHY_CURRENT_DATA_INSUFFICIENT=transport.delivery_disposition_cases stores attempted_quantity, accepted_quantity, and rejected_quantity (UOM PALLET) only when a case exists. Domain comment in services/shipment-service/internal/domain/transport_execution_disposition.go: a zero rejection does not open a disposition case. Shipment status DELIVERED is set when the final required delivery action completes (transport_execution_command_repository.go transition to DELIVERED). Integration test PARTIAL_DELIVERY_NO_EARLY_DELIVERED shows a partial delivery does not become DELIVERED. Absence of a case is therefore not proof of in-full, and DELIVERED is not proof of in-full.
AUTHORITATIVE_OWNER=shipment-service / TMS execution
OWNER_AGENT=C
SUGGESTED_SOURCE_CONTRACT=On delivery completion, persist shipped_quantity, accepted_quantity, rejected_quantity, uom, and an explicit in_full boolean or equivalent derived only from those quantities. Record event time separately from processing time.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.2 (OTIF), ANALYTICS-0.3, ANALYTICS-0.7
PRIORITY=P0
```

### GAP-C-002

```
GAP_ID=GAP-C-002
KPI_IDS_AFFECTED=OPS_POD_COMPLETENESS, CAR_POD_COMPLETENESS
MISSING_FACT=Authoritative rule that a shipment or stop requires a POD, and which document satisfies it
WHY_CURRENT_DATA_INSUFFICIENT=documents.documents.document_type includes POD (000005). Execution domain sets RequiresPOD false on DELIVERY_COMPLETED in the driver action map. No completeness aggregate was found. Counting POD rows divided by delivered shipments would invent the requirement.
AUTHORITATIVE_OWNER=shipment-service execution requirement; document-service holds the file once required
OWNER_AGENT=C
SUGGESTED_SOURCE_CONTRACT=Execution fact: pod_required, required_document_type, satisfied_document_id, satisfied_at. Document trust remains Agent B.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.2, ANALYTICS-0.3
PRIORITY=P0
```

### GAP-B-001

```
GAP_ID=GAP-B-001
KPI_IDS_AFFECTED=OPS_DOCUMENT_COMPLETENESS, DOC_DOCUMENT_COMPLETENESS, CAR_DOCUMENT_COMPLETENESS
MISSING_FACT=Required document set per shipment or transport order, and a server-owned completeness result
WHY_CURRENT_DATA_INSUFFICIENT=Document status enums exist. No completeness field or required-set table was found in document-service. Control Tower awaitingDocuments counts shipment statuses DELIVERED and DELIVERY_CONFIRMED (api-gateway CalculateKPI). That is not document completeness.
AUTHORITATIVE_OWNER=document-service, with the required set agreed with TMS
OWNER_AGENT=B
SUGGESTED_SOURCE_CONTRACT=required_document_types[], present_document_ids[], completeness_status, evaluated_at, tenant_id
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.2, document analytics
PRIORITY=P0
```

### GAP-B-002

```
GAP_ID=GAP-B-002
KPI_IDS_AFFECTED=DOC_QUALIFIED_TRUST_COUNT
MISSING_FACT=A qualified verification outcome other than verifier-unavailable
WHY_CURRENT_DATA_INSUFFICIENT=000096 signature_verification_evidence allows verification_status PENDING only and reason_code VERIFIER_UNAVAILABLE only. ADR-EDO-010 states QUALIFIED_DOCUMENT_TRUST_READY=NO and forbids reading legacy DocumentStatus=SIGNED as qualified verification. Legacy signing_repository can write verification_status VALID without a cryptographic check.
AUTHORITATIVE_OWNER=document-service / EDO
OWNER_AGENT=B
SUGGESTED_SOURCE_CONTRACT=Evidence row that can record VALID or INVALID under policy QUALIFIED_CADES_BES only after a real verifier result, with attempted_at, verifier_version, policy_id, policy_version. Do not overload document_status.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=Document trust analytics (after Agent B unblocks the verifier)
PRIORITY=P1
```

### GAP-RFX-001

```
GAP_ID=GAP-RFX-001
KPI_IDS_AFFECTED=PROC_TENDER_SAVINGS, PROC_TENDER_SAVINGS_PCT, EXEC_TENDER_SAVINGS
MISSING_FACT=Authoritative tender savings baseline
WHY_CURRENT_DATA_INSUFFICIENT=NOT_FOUND as RFx columns or enums: BASELINE_PRICE, BUDGET, HISTORICAL_RATE, FIRST_BID. rfx.rfx_lots.estimated_value exists and is not labeled as the savings baseline. contract_rate pricing and freight_cost.cost_analytics_opportunity_projection.baseline_amount are lane median benchmarks, not tender-time baselines. No baseline was selected.
AUTHORITATIVE_OWNER=rfx-service
OWNER_AGENT=UNASSIGNED_SOURCE_DOMAIN
SUGGESTED_SOURCE_CONTRACT=On the RFx or lot: baseline_kind (one of an explicit allowed set), baseline_amount, currency_code, baseline_source_id, baseline_as_of. Savings = baseline minus awarded amount only when currency matches and baseline_kind is the frozen kind.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.4, ANALYTICS-0.7
PRIORITY=P1
```

The Agent E map has no procurement owner. This gap is not assigned to E, A, B, C, or D.

### GAP-C-003

```
GAP_ID=GAP-C-003
KPI_IDS_AFFECTED=FIN_COST_PER_KM
MISSING_FACT=Executed distance in kilometres for the shipment or transport order
WHY_CURRENT_DATA_INSUFFICIENT=NOT_FOUND: distance_km on transport.shipments or transport orders. network_optimizer policy columns min_loaded_distance_km and max_deadhead_km are thresholds, not executed distance. match_candidates.road_deadhead_km is a search-candidate deadhead, not the loaded trip distance.
AUTHORITATIVE_OWNER=shipment-service / transport execution
OWNER_AGENT=C
SUGGESTED_SOURCE_CONTRACT=executed_loaded_distance_km, distance_source, measured_at, tenant_id on the execution or shipment. Do not reuse NLO deadhead.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.5
PRIORITY=P1
```

### GAP-C-004

```
GAP_ID=GAP-C-004
KPI_IDS_AFFECTED=FIN_COST_PER_TON
MISSING_FACT=Weight unit for transport.cargoes.gross_weight
WHY_CURRENT_DATA_INSUFFICIENT=000003 defines gross_weight NUMERIC(18,3) and net_weight NUMERIC(18,3). NOT_FOUND: weight unit column in migrations. A ton denominator cannot be calculated without inventing kilograms.
AUTHORITATIVE_OWNER=transport cargo master (transport-order-service / shipment cargo)
OWNER_AGENT=C
SUGGESTED_SOURCE_CONTRACT=gross_weight, weight_uom (explicit unit code), recorded_at
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.5
PRIORITY=P1
```

### GAP-C-005

```
GAP_ID=GAP-C-005
KPI_IDS_AFFECTED=CAR_REJECTION_RATE
MISSING_FACT=Canonical carrier rejection of an assignment or offered shipment, distinct from delivery-quantity rejection and from RFx non-response
WHY_CURRENT_DATA_INSUFFICIENT=Shipment status list includes ACCEPTED_BY_CARRIER and CANCELLED. NOT_FOUND: a carrier-rejected status. Delivery disposition reason CUSTOMER_REFUSAL is cargo disposition, not carrier acceptance. RFx participant non-response is procurement.
AUTHORITATIVE_OWNER=shipment-service / TMS
OWNER_AGENT=C
SUGGESTED_SOURCE_CONTRACT=Assignment offer and a terminal decision ACCEPTED or REJECTED with occurred_at, actor, reason_code, tenant_id
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.3
PRIORITY=P1
```

### GAP-E-001

```
GAP_ID=GAP-E-001
KPI_IDS_AFFECTED=CAR_PERFORMANCE_SCORE, EXEC_CARRIER_PERFORMANCE
MISSING_FACT=Canonical score weights and component set
WHY_CURRENT_DATA_INSUFFICIENT=NOT_FOUND: carrier_score, performance_score, or scoring weights in the repository.
AUTHORITATIVE_OWNER=Analytics definition, using only source facts that are themselves READY or explicitly included while partial
OWNER_AGENT=E
SUGGESTED_SOURCE_CONTRACT=SCORING_POLICY_REQUIRED=YES. KPI_ID, KPI_VERSION, component KPI ids, weights, inclusion rules. Do not invent weights in 0.1A.
REQUIRES_PRODUCT_CHANGE=NO
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.3, ANALYTICS-0.7
PRIORITY=P1
```

### GAP-D-001

```
GAP_ID=GAP-D-001
KPI_IDS_AFFECTED=NET_AVG_DEADHEAD_REDUCTION_KM
MISSING_FACT=Deadhead kilometres saved against a declared baseline for an accepted plan
WHY_CURRENT_DATA_INSUFFICIENT=match_candidates.road_deadhead_km is candidate deadhead. NOT_FOUND: a reduction column or baseline-versus-accepted fact.
AUTHORITATIVE_OWNER=network-optimizer-service
OWNER_AGENT=D
SUGGESTED_SOURCE_CONTRACT=accepted plan id, baseline_deadhead_km, resulting_deadhead_km, reduction_km, distance_source, occurred_at. Mark the fact version while B2/B3/B4 semantics move.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.6
PRIORITY=P2
```

### GAP-D-002

```
GAP_ID=GAP-D-002
KPI_IDS_AFFECTED=NET_LOADED_KM, NET_EMPTY_KM, NET_DEADHEAD_PCT, EXEC_DEADHEAD_PCT
MISSING_FACT=Executed loaded kilometres and empty kilometres
WHY_CURRENT_DATA_INSUFFICIENT=NOT_FOUND: loaded_km and empty_km columns. Policy thresholds and candidate deadhead are different measures.
AUTHORITATIVE_OWNER=network-optimizer-service for network intelligence; do not borrow shipment status as distance
OWNER_AGENT=D
SUGGESTED_SOURCE_CONTRACT=trip or plan fact with loaded_km, empty_km, deadhead_km, unit, calculation_version, tenant or anonymized marketplace scope
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.6, ANALYTICS-0.7
PRIORITY=P2
```

### GAP-D-003

```
GAP_ID=GAP-D-003
KPI_IDS_AFFECTED=NET_BACKHAUL_OPPORTUNITIES, NET_BACKHAUL_ACCEPTED, NET_BACKHAUL_CONVERSION
MISSING_FACT=Backhaul opportunity and backhaul acceptance
WHY_CURRENT_DATA_INSUFFICIENT=NOT_FOUND in network-optimizer-service Go code and in infrastructure/migrations. route_plans.status=ACCEPTED is a route-plan acceptance, not a backhaul fact. Docs ADR-NET-023 and NLO_0_5A_BACKHAUL_MODEL are design material, not a persisted fact.
AUTHORITATIVE_OWNER=network-optimizer-service
OWNER_AGENT=D
SUGGESTED_SOURCE_CONTRACT=backhaul_opportunity_id, feasibility result, accepted_at, tenant scope, anonymization class. Do not alias route-plan ACCEPTED.
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.6
PRIORITY=P2
```

### GAP-D-004

```
GAP_ID=GAP-D-004
KPI_IDS_AFFECTED=NET_SEARCH_TO_ACCEPT_CONVERSION
MISSING_FACT=Canonical link from a search run or candidate to a subsequent route-plan acceptance
WHY_CURRENT_DATA_INSUFFICIENT=next_load_search_runs and match_candidates persist search output. route_plans.accepted_at persists acceptance. A conversion fact joining those populations was NOT proven. Counting accepts divided by searches would invent the link.
AUTHORITATIVE_OWNER=network-optimizer-service
OWNER_AGENT=D
SUGGESTED_SOURCE_CONTRACT=search_run_id, candidate_id, route_plan_id, accepted_at, nullable when no accept
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.6
PRIORITY=P2
```

### GAP-D-005

```
GAP_ID=GAP-D-005
KPI_IDS_AFFECTED=NET_ROUTE_INCREASE_KM
MISSING_FACT=Persisted route_increase_km on the candidate or plan
WHY_CURRENT_DATA_INSUFFICIENT=SearchCandidateView exposes route_increase_km in services/network-optimizer-service/internal/service/search.go. The explore of persistSearch showed that field is not a match_candidates column. max_route_increase_km is a policy cap (000078). Historical route-increase analytics cannot be rebuilt from the candidate table.
AUTHORITATIVE_OWNER=network-optimizer-service
OWNER_AGENT=D
SUGGESTED_SOURCE_CONTRACT=Persist route_increase_km, distance_source, and policy version on the candidate or accepted plan. Straight-line distance must not be stored as road distance (routing/straight.go comment).
REQUIRES_PRODUCT_CHANGE=YES
BLOCKS_ANALYTICS_PHASE=ANALYTICS-0.6 historical route increase
PRIORITY=P2
```

## Totals

```
SOURCE_CONTRACT_GAP_TOTAL=14
AGENT_A_GAPS=0
AGENT_B_GAPS=2
AGENT_C_GAPS=5
AGENT_D_GAPS=5
AGENT_E_GAPS=1
UNASSIGNED_SOURCE_DOMAIN_GAPS=1
P0_GAPS=3
P1_GAPS=6
P2_GAPS=5
```

P0 = GAP-C-001, GAP-C-002, GAP-B-001.

P1 = GAP-B-002, GAP-RFX-001, GAP-C-003, GAP-C-004, GAP-C-005, GAP-E-001.

P2 = GAP-D-001 through GAP-D-005.

Definition choices that are not missing source facts (dwell interval, which actual-cost stage, which financial-close marker, reporting timezone, participation definition) are inputs to Analytics-0.1B. They are recorded in the current-state document, not as fake source facts.
