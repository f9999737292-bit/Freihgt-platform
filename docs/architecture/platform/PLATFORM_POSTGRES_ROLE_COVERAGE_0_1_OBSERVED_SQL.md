# Observed PostgreSQL operations by service

Evidence appendix for Platform PostgreSQL Role Coverage 0.1.
Base `00bc72d6a6b3bbd70cb813ef9c0fe975f2efcebb`.
Runtime Go under `services/`, excluding `*_test.go`, `integration`, `testdata`, and `testkit`.
Operations come from SQL string literals. `SELECT` includes `FROM` and `JOIN`.
`FOR UPDATE` is counted in the discovery, not as a separate verb here.
Excluded false positives: the `EXCLUDED` alias, `SET role = $n` column assignments, the words `copy option` and `copy rule`, and `pg_catalog` helpers.
`company.authorize_carrier_relationship_activation` is a function call, not a table.
This file grants nothing.

## company-service

Proposed privilege role: `company_runtime (exists, NOLOGIN)`.

| Relation | Observed operations |
| --- | --- |
| `company.carrier_relationship_acceptance_idempotency` | INSERT, SELECT |
| `company.carrier_relationship_audit` | INSERT |
| `company.carrier_relationship_cancel_idempotency` | INSERT, SELECT |
| `company.carrier_relationship_heads` | INSERT, SELECT, UPDATE |
| `company.carrier_relationship_outbox` | INSERT |
| `company.carrier_relationship_request_events` | INSERT, SELECT |
| `company.carrier_relationship_request_heads` | INSERT, SELECT, UPDATE |
| `company.carrier_relationship_request_idempotency` | INSERT, SELECT |
| `company.carrier_relationship_requests` | INSERT, SELECT |
| `company.carrier_relationship_revoke_idempotency` | INSERT, SELECT |
| `company.carrier_relationship_versions` | INSERT, SELECT |
| `company.carrier_relationships` | INSERT, SELECT |
| `core.companies` | INSERT, SELECT, UPDATE |
| `core.company_memberships` | INSERT, SELECT, UPDATE |
| `core.roles` | SELECT |
| `core.user_roles` | DELETE, INSERT, SELECT |
| `core.users` | SELECT |

## shipment-service

Proposed privilege role: `shipment_runtime (exists, NOLOGIN)`.

Functions called: `company.authorize_carrier_relationship_activation`, `company.record_activation_audit`, `transport.attach_activation_pins`, `transport.propagate_execution_relationship_pins`.

| Relation | Observed operations |
| --- | --- |
| `core.companies` | SELECT |
| `core.company_memberships` | SELECT |
| `core.roles` | SELECT |
| `core.user_roles` | SELECT |
| `core.users` | SELECT |
| `documents.documents` | SELECT |
| `rfx.bids` | SELECT |
| `rfx.rfx_award_transport_orders` | SELECT |
| `transport.cargoes` | SELECT |
| `transport.delivery_attempt_facts` | INSERT, SELECT |
| `transport.delivery_disposition_audit` | INSERT |
| `transport.delivery_disposition_cases` | INSERT, SELECT, UPDATE |
| `transport.delivery_disposition_commands` | INSERT, SELECT, UPDATE |
| `transport.delivery_disposition_evidence` | INSERT |
| `transport.driver_device` | INSERT, SELECT, UPDATE |
| `transport.driver_notification_delivery` | INSERT, SELECT, UPDATE |
| `transport.driver_operation_idempotency` | INSERT, SELECT |
| `transport.driver_reported_delay` | INSERT, SELECT |
| `transport.driver_reported_exception` | INSERT, SELECT |
| `transport.driver_stop_tasks` | INSERT, SELECT, UPDATE |
| `transport.driver_task` | INSERT, SELECT, UPDATE |
| `transport.driver_task_response` | INSERT, SELECT |
| `transport.drivers` | INSERT, SELECT |
| `transport.locations` | SELECT |
| `transport.shipment_cargo_execution_evidence` | INSERT, SELECT |
| `transport.shipment_event_outbox` | INSERT, SELECT, UPDATE |
| `transport.shipment_status_history` | INSERT, SELECT |
| `transport.shipments` | INSERT, SELECT, UPDATE |
| `transport.transport_execution_actions` | INSERT, SELECT, UPDATE |
| `transport.transport_execution_active_shipments` | INSERT, SELECT |
| `transport.transport_execution_command_audit` | INSERT |
| `transport.transport_execution_commands` | INSERT, SELECT, UPDATE |
| `transport.transport_execution_participants` | INSERT, SELECT |
| `transport.transport_execution_pod_requirement_decisions` | SELECT |
| `transport.transport_execution_revision_actions` | INSERT, SELECT, UPDATE |
| `transport.transport_execution_revision_stops` | INSERT, SELECT, UPDATE |
| `transport.transport_execution_revisions` | INSERT, SELECT, UPDATE |
| `transport.transport_execution_stops` | INSERT, SELECT, UPDATE |
| `transport.transport_executions` | INSERT, SELECT, UPDATE |
| `transport.transport_orders` | SELECT, UPDATE |
| `transport.vehicles` | INSERT, SELECT |

## identity-service

Proposed privilege role: `identity_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `core.companies` | SELECT |
| `core.company_memberships` | SELECT |
| `core.permissions` | SELECT |
| `core.role_permissions` | SELECT |
| `core.roles` | SELECT |
| `core.tenants` | SELECT |
| `core.user_roles` | DELETE, INSERT, SELECT |
| `core.users` | INSERT, SELECT, UPDATE |

## transport-order-service

Proposed privilege role: `transport_order_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `core.companies` | SELECT |
| `transport.cargo_items` | INSERT, SELECT |
| `transport.cargoes` | INSERT, SELECT |
| `transport.locations` | INSERT, SELECT |
| `transport.transport_order_create_idempotency` | INSERT, SELECT |
| `transport.transport_order_rate_snapshots` | INSERT, SELECT |
| `transport.transport_orders` | INSERT, SELECT, UPDATE |

## rfx-service

Proposed privilege role: `rfx_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `core.companies` | SELECT |
| `core.company_memberships` | SELECT |
| `core.roles` | SELECT |
| `core.user_roles` | SELECT |
| `core.users` | SELECT |
| `rfx.audit_events` | INSERT, SELECT |
| `rfx.bid_items` | INSERT, SELECT |
| `rfx.bids` | INSERT, SELECT, UPDATE |
| `rfx.freight_requests` | INSERT, SELECT, UPDATE |
| `rfx.rfx_answer_scores` | DELETE, INSERT, SELECT |
| `rfx.rfx_answers` | DELETE, INSERT, SELECT |
| `rfx.rfx_award_transport_orders` | INSERT, SELECT |
| `rfx.rfx_awards` | INSERT, SELECT |
| `rfx.rfx_change_impact_analyses` | INSERT, SELECT, UPDATE |
| `rfx.rfx_events` | INSERT, SELECT, UPDATE |
| `rfx.rfx_external_object_link_revisions` | INSERT, SELECT |
| `rfx.rfx_external_object_links` | INSERT, SELECT, UPDATE |
| `rfx.rfx_idempotency_records` | INSERT, SELECT |
| `rfx.rfx_import_analyses` | INSERT, SELECT, UPDATE |
| `rfx.rfx_integration_credentials` | INSERT, SELECT |
| `rfx.rfx_integration_principals` | INSERT, SELECT |
| `rfx.rfx_integration_scopes` | INSERT, SELECT |
| `rfx.rfx_lanes` | INSERT, SELECT |
| `rfx.rfx_late_submission_requests` | INSERT, SELECT, UPDATE |
| `rfx.rfx_lots` | INSERT, SELECT, UPDATE |
| `rfx.rfx_participants` | INSERT, SELECT, UPDATE |
| `rfx.rfx_qualification_results` | INSERT, SELECT |
| `rfx.rfx_question_options` | INSERT, SELECT, UPDATE |
| `rfx.rfx_question_rules` | INSERT, SELECT, UPDATE |
| `rfx.rfx_questions` | INSERT, SELECT, UPDATE |
| `rfx.rfx_reference_mapping_entries` | INSERT, SELECT |
| `rfx.rfx_reference_mapping_sets` | INSERT, SELECT |
| `rfx.rfx_response_offer_lines` | DELETE, INSERT, SELECT |
| `rfx.rfx_responses` | INSERT, SELECT, UPDATE |
| `rfx.rfx_score_bindings` | DELETE, INSERT, SELECT |
| `rfx.rfx_score_criteria` | DELETE, INSERT, SELECT |
| `rfx.rfx_score_models` | INSERT, SELECT, UPDATE |
| `rfx.rfx_sections` | INSERT, SELECT, UPDATE |
| `rfx.rfx_template_question_options` | INSERT, SELECT, UPDATE |
| `rfx.rfx_template_question_rules` | INSERT, SELECT, UPDATE |
| `rfx.rfx_template_questions` | INSERT, SELECT, UPDATE |
| `rfx.rfx_template_sections` | INSERT, SELECT, UPDATE |
| `rfx.rfx_template_versions` | INSERT, SELECT, UPDATE |
| `rfx.rfx_templates` | INSERT, SELECT, UPDATE |
| `rfx.rfx_versions` | INSERT, SELECT, UPDATE |
| `transport.cargoes` | INSERT |
| `transport.locations` | SELECT |
| `transport.transport_orders` | SELECT |

## document-service

Proposed privilege role: `document_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `core.companies` | SELECT |
| `core.users` | SELECT |
| `documents.attachment_audit_events` | INSERT, SELECT |
| `documents.attachment_signature_blobs` | INSERT, SELECT |
| `documents.attachment_signatures` | INSERT, SELECT |
| `documents.document_attachments` | DELETE, INSERT, SELECT, UPDATE |
| `documents.document_files` | INSERT, SELECT |
| `documents.document_upload_intent` | INSERT, SELECT, UPDATE |
| `documents.document_versions` | INSERT, SELECT |
| `documents.documents` | INSERT, SELECT, UPDATE |
| `documents.pod_delivery_action_evidence` | INSERT, SELECT |
| `documents.pod_delivery_action_upload_intent` | INSERT, SELECT, UPDATE |
| `documents.signature_verification_evidence` | INSERT, SELECT |
| `documents.signature_verification_history` | INSERT |
| `documents.signatures` | INSERT |
| `documents.signing_sessions` | INSERT, SELECT, UPDATE |
| `transport.shipments` | SELECT |

## billing-register-service

Proposed privilege role: `billing_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `billing.acts` | INSERT, SELECT |
| `billing.billing_register_audit_events` | INSERT |
| `billing.billing_register_items` | DELETE, INSERT, SELECT |
| `billing.billing_registers` | INSERT, SELECT, UPDATE |
| `billing.closing_document_packages` | INSERT, SELECT |
| `billing.freight_cost_outbox` | INSERT, SELECT, UPDATE |
| `billing.freight_settlements` | INSERT, SELECT, UPDATE |
| `billing.invoices` | INSERT, SELECT |
| `billing.payment_obligations` | SELECT |
| `billing.settlement_accessorials` | INSERT, SELECT, UPDATE |
| `billing.settlement_audit_events` | INSERT |
| `billing.settlement_disputes` | INSERT, SELECT, UPDATE |
| `billing.upd_documents` | INSERT, SELECT |
| `billing.vat_invoices` | INSERT, SELECT |
| `core.companies` | SELECT |
| `core.company_memberships` | SELECT |
| `core.roles` | SELECT |
| `core.user_roles` | SELECT |
| `documents.documents` | SELECT |
| `rfx.rfx_award_transport_orders` | SELECT |
| `transport.shipments` | SELECT |
| `transport.transport_order_rate_snapshots` | SELECT |
| `transport.transport_orders` | SELECT |

## payment-service

Proposed privilege role: `payment_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `billing.billing_register_items` | SELECT |
| `billing.billing_registers` | SELECT |
| `billing.freight_settlements` | SELECT |
| `billing.payment_allocations` | INSERT, SELECT, UPDATE |
| `billing.payment_audit_events` | INSERT, SELECT |
| `billing.payment_obligations` | INSERT, SELECT, UPDATE |
| `billing.payment_outbox` | INSERT, SELECT, UPDATE |
| `billing.payments` | INSERT, SELECT, UPDATE |
| `core.companies` | SELECT |
| `core.company_memberships` | SELECT |
| `core.roles` | SELECT |
| `core.user_roles` | SELECT |

## low-code-service

Proposed privilege role: `low_code_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `lowcode.configuration_audit_log` | INSERT, SELECT |
| `lowcode.custom_field_values` | DELETE, INSERT, SELECT |
| `lowcode.form_fields` | DELETE, INSERT, SELECT |
| `lowcode.form_sections` | DELETE, INSERT, SELECT |
| `lowcode.form_templates` | INSERT, SELECT, UPDATE |
| `lowcode.low_code_configurations` | INSERT, UPDATE |

## control-tower-read-model-service

Proposed privilege role: `control_tower_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `control_tower.automation_audit_event` | INSERT |
| `control_tower.automation_recommendation` | INSERT, SELECT, UPDATE |
| `control_tower.automation_rule` | INSERT, SELECT, UPDATE |
| `control_tower.automation_rule_version` | INSERT |
| `control_tower.critical_event_acknowledgement` | INSERT, SELECT |
| `control_tower.critical_event_action` | INSERT, SELECT |
| `control_tower.critical_event_workflow` | INSERT, SELECT, UPDATE |
| `control_tower.delivery_disposition_inbox` | INSERT, UPDATE |
| `control_tower.delivery_disposition_projection` | INSERT, SELECT |
| `control_tower.driver_event_inbox` | INSERT, SELECT |
| `control_tower.execution_action_projection` | INSERT, SELECT |
| `control_tower.execution_event_inbox` | INSERT, SELECT, UPDATE |
| `control_tower.execution_progress_event` | INSERT, SELECT |
| `control_tower.execution_projection` | INSERT, SELECT, UPDATE |
| `control_tower.execution_stop_projection` | INSERT, SELECT, UPDATE |
| `control_tower.operational_case` | INSERT, SELECT, UPDATE |
| `control_tower.operational_case_action_item` | INSERT, SELECT, UPDATE |
| `control_tower.operational_case_active_work_link` | DELETE, INSERT, SELECT |
| `control_tower.operational_case_decision` | INSERT, SELECT |
| `control_tower.operational_case_event` | INSERT, SELECT |
| `control_tower.operational_case_link` | DELETE, INSERT, SELECT |
| `control_tower.operational_case_note` | INSERT, SELECT, UPDATE |
| `control_tower.operational_case_note_mention` | INSERT |
| `control_tower.operational_case_participant` | DELETE, INSERT, SELECT, UPDATE |
| `control_tower.operational_case_reference_counter` | INSERT |
| `control_tower.operational_playbook` | INSERT, SELECT, UPDATE |
| `control_tower.operational_playbook_step` | INSERT, SELECT |
| `control_tower.operational_playbook_version` | INSERT, SELECT |
| `control_tower.playbook_execution` | INSERT, SELECT, UPDATE |
| `control_tower.playbook_execution_step` | INSERT, SELECT, UPDATE |
| `control_tower.saved_view` | DELETE, INSERT, SELECT, UPDATE |
| `control_tower.shift_handoff` | INSERT, SELECT |
| `control_tower.shift_handoff_item` | INSERT, SELECT |
| `control_tower.shipment_risk` | INSERT, SELECT, UPDATE |
| `control_tower.shipment_risk_action` | INSERT, SELECT |
| `control_tower.shipment_risk_assessment` | INSERT, SELECT |
| `control_tower.shipment_risk_signal` | INSERT, SELECT |
| `control_tower.shipment_status_event_dead_letter` | INSERT |
| `control_tower.shipment_status_event_inbox` | INSERT, SELECT |
| `control_tower.shipment_status_projection` | DELETE, INSERT, SELECT |
| `control_tower.shipment_status_projection_rebuild_backup` | DELETE, INSERT, SELECT |
| `control_tower.shipment_status_projection_rebuild_job` | INSERT, SELECT, UPDATE |
| `control_tower.shipment_status_projection_rebuild_stage` | DELETE, INSERT, SELECT |
| `control_tower.user_workspace_preference` | INSERT, SELECT |
| `core.users` | SELECT |
| `transport.shipments` | SELECT |

## tracking-service

Proposed privilege role: `tracking_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `tracking.driver_tracking_automation_state` | INSERT, SELECT |
| `tracking.eta_observation` | INSERT, SELECT |
| `tracking.eta_state_transition` | INSERT |
| `tracking.event_outbox` | INSERT, SELECT, UPDATE |
| `tracking.execution_event_inbox` | INSERT |
| `tracking.execution_stop_eta_state` | INSERT, SELECT |
| `tracking.execution_tracking_state` | INSERT, SELECT, UPDATE |
| `tracking.location_event` | INSERT, SELECT |
| `tracking.shipment_eta_state` | INSERT, SELECT |
| `tracking.shipment_slot_revision` | INSERT, SELECT |
| `tracking.shipment_slot_state` | INSERT, SELECT |
| `tracking.shipment_tracking_binding` | INSERT, SELECT |
| `tracking.shipment_tracking_state` | INSERT, SELECT |
| `tracking.slot_state_transition` | INSERT |
| `tracking.tracking_state_transition` | INSERT |
| `transport.shipment_event_outbox` | INSERT |
| `transport.shipments` | SELECT |

## network-optimizer-service

Proposed privilege role: `network_optimizer_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `network_optimizer.audit_events` | INSERT, SELECT |
| `network_optimizer.capacities` | INSERT, SELECT, UPDATE |
| `network_optimizer.capacity_search_policies` | INSERT, SELECT |
| `network_optimizer.cargo_type_catalog` | SELECT |
| `network_optimizer.carrier_search_policies` | INSERT, SELECT |
| `network_optimizer.catalog_aliases` | SELECT |
| `network_optimizer.chain_candidates` | INSERT, SELECT |
| `network_optimizer.chain_search_idempotency` | INSERT, SELECT |
| `network_optimizer.chain_search_runs` | INSERT, SELECT, UPDATE |
| `network_optimizer.compatibility_rule_sets` | INSERT, SELECT, UPDATE |
| `network_optimizer.compatibility_rules` | DELETE, INSERT, SELECT |
| `network_optimizer.consolidation_candidate_members` | INSERT, SELECT |
| `network_optimizer.consolidation_candidates` | INSERT, SELECT |
| `network_optimizer.consolidation_search_runs` | INSERT, SELECT |
| `network_optimizer.equipment_type_catalog` | SELECT |
| `network_optimizer.idempotency_keys` | INSERT, SELECT, UPDATE |
| `network_optimizer.load_opportunities` | INSERT, SELECT, UPDATE |
| `network_optimizer.match_candidates` | INSERT, SELECT |
| `network_optimizer.next_load_search_runs` | INSERT, SELECT |
| `network_optimizer.outbox` | INSERT, SELECT |
| `network_optimizer.packaging_type_catalog` | SELECT |
| `network_optimizer.pallet_equivalences` | SELECT |
| `network_optimizer.pallet_type_catalog` | SELECT |
| `network_optimizer.predicted_capacities` | INSERT, SELECT, UPDATE |
| `network_optimizer.reference_catalog_versions` | SELECT |
| `network_optimizer.route_capacity_snapshots` | INSERT, SELECT |
| `network_optimizer.route_plan_activations` | INSERT, SELECT, UPDATE |
| `network_optimizer.route_plan_dependencies` | INSERT, SELECT |
| `network_optimizer.route_plan_legs` | INSERT, SELECT |
| `network_optimizer.route_plan_stops` | INSERT, SELECT |
| `network_optimizer.route_plans` | INSERT, SELECT, UPDATE |
| `network_optimizer.route_stop_actions` | INSERT, SELECT |
| `network_optimizer.score_profile_components` | SELECT |
| `network_optimizer.score_profiles` | SELECT |
| `network_optimizer.service_duration_policies` | INSERT, SELECT, UPDATE |
| `network_optimizer.service_duration_policy_entries` | INSERT, SELECT, UPDATE |

## freight-cost-service

Proposed privilege role: `freight_cost_runtime (not created)`.

Explicit `nextval`: `freight_cost.charge_code_mapping_version_seq`.

| Relation | Observed operations |
| --- | --- |
| `freight_cost.analytics_projection_coverage` | DELETE, INSERT, SELECT |
| `freight_cost.analytics_projection_dirty` | DELETE, INSERT, SELECT |
| `freight_cost.analytics_projection_state` | INSERT, SELECT |
| `freight_cost.charge_code_mapping` | INSERT, SELECT |
| `freight_cost.cost_analytics_accessorial_fact` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_accessorial_period_projection` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_benchmark_projection` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_carrier_period_projection` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_lane_period_projection` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_opportunity_projection` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_order_fact` | DELETE, INSERT, SELECT |
| `freight_cost.cost_analytics_period_projection` | DELETE, INSERT, SELECT |
| `freight_cost.cost_entry` | INSERT, SELECT |
| `freight_cost.cost_summary_projection` | INSERT, SELECT |
| `freight_cost.reconciliation_finding` | INSERT, SELECT, UPDATE |
| `freight_cost.source_cursor` | INSERT, SELECT |
| `freight_cost.variance_attribution` | INSERT, SELECT, UPDATE |

## contract-rate-service

Proposed privilege role: `contract_rate_runtime (not created)`.

| Relation | Observed operations |
| --- | --- |
| `contract_rate.audit_event` | INSERT |
| `contract_rate.rate_card` | INSERT, SELECT, UPDATE |
| `contract_rate.rate_card_version` | DELETE, INSERT, SELECT, UPDATE |
| `contract_rate.rate_component` | DELETE, INSERT, SELECT, UPDATE |
| `contract_rate.rate_line` | DELETE, INSERT, SELECT, UPDATE |
| `contract_rate.transport_contract` | INSERT, SELECT, UPDATE |
| `core.companies` | SELECT |
| `core.company_memberships` | SELECT |
| `core.permissions` | SELECT |
| `core.role_permissions` | SELECT |
| `core.roles` | SELECT |
| `core.user_roles` | SELECT |
| `transport.locations` | SELECT |
