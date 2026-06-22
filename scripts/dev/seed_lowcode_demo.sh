#!/usr/bin/env bash
# Dev-only seed for published low-code form templates (psql via Docker).
set -euo pipefail

TENANT_ID="${TENANT_ID:-74519f22-ff9b-4a8b-8fff-a958c689682f}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-freight_postgres}"
POSTGRES_USER="${POSTGRES_USER:-freight}"
POSTGRES_DB="${POSTGRES_DB:-freight_platform}"

step() { echo "==> $1" >&2; }
pass() { echo "OK: $1" >&2; }
skip() { echo "SKIP: $1" >&2; }

docker_cmd() {
  local win_docker="/c/Program Files/Docker/Docker/resources/bin/docker.exe"
  if [[ -x "$win_docker" ]]; then
    "$win_docker" "$@"
  else
    docker "$@"
  fi
}

psql_exec() {
  docker_cmd exec -i "$POSTGRES_CONTAINER" psql -q -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"
}

require_postgres() {
  if ! docker_cmd inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
    echo "ERROR: postgres container '$POSTGRES_CONTAINER' is not running" >&2
    exit 1
  fi
}

template_exists() {
  local entity_type="$1"
  local code="$2"
  local count
  count="$(psql_exec -t -A -c "
    SELECT COUNT(*)
    FROM lowcode.form_templates
    WHERE tenant_id = '${TENANT_ID}'
      AND entity_type = '${entity_type}'
      AND code = '${code}'
      AND status = 'PUBLISHED';
  " | tr -d '[:space:]')"
  [[ "${count:-0}" != "0" ]]
}

seed_transport_order_template() {
  if template_exists "TRANSPORT_ORDER" "transport_order_default"; then
    skip "published template exists: TRANSPORT_ORDER / transport_order_default"
    return 0
  fi

  step "Seed published template TRANSPORT_ORDER / transport_order_default"
  psql_exec <<SQL
INSERT INTO lowcode.low_code_configurations (
  id, tenant_id, code, name, config_type, status, version, published_at
) VALUES (
  'b1111111-1111-4111-8111-111111111101',
  '${TENANT_ID}',
  'cfg_transport_order_default',
  'Transport Order Default Configuration',
  'FORM_TEMPLATE',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_templates (
  id, tenant_id, configuration_id, entity_type, code, name, status, version, published_at
) VALUES (
  'b1111111-1111-4111-8111-111111111102',
  '${TENANT_ID}',
  'b1111111-1111-4111-8111-111111111101',
  'TRANSPORT_ORDER',
  'transport_order_default',
  'Transport Order Default Form',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_sections (
  id, tenant_id, form_template_id, code, title, sort_order
) VALUES (
  'b1111111-1111-4111-8111-111111111103',
  '${TENANT_ID}',
  'b1111111-1111-4111-8111-111111111102',
  'cargo',
  'Cargo',
  100
) ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  sort_order = EXCLUDED.sort_order;

INSERT INTO lowcode.form_fields (
  id, tenant_id, form_template_id, section_id, code, label, field_type,
  required, read_only, system_field, options_json, sort_order
) VALUES
(
  'b1111111-1111-4111-8111-111111111104',
  '${TENANT_ID}',
  'b1111111-1111-4111-8111-111111111102',
  'b1111111-1111-4111-8111-111111111103',
  'cargo_class',
  'Cargo class',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"GENERAL","label":"General"},{"value":"A","label":"Class A"},{"value":"B","label":"Class B"},{"value":"C","label":"Class C"}]}'::jsonb,
  100
),
(
  'b1111111-1111-4111-8111-111111111105',
  '${TENANT_ID}',
  'b1111111-1111-4111-8111-111111111102',
  'b1111111-1111-4111-8111-111111111103',
  'internal_cost_center',
  'Internal cost center',
  'TEXT',
  false,
  false,
  false,
  NULL,
  110
),
(
  'b1111111-1111-4111-8111-111111111106',
  '${TENANT_ID}',
  'b1111111-1111-4111-8111-111111111102',
  'b1111111-1111-4111-8111-111111111103',
  'loading_window_note',
  'Loading window note',
  'TEXT',
  false,
  false,
  false,
  NULL,
  120
)
ON CONFLICT (id) DO UPDATE SET
  label = EXCLUDED.label,
  field_type = EXCLUDED.field_type,
  options_json = EXCLUDED.options_json,
  sort_order = EXCLUDED.sort_order;
SQL
  pass "created TRANSPORT_ORDER / transport_order_default"
}

seed_shipment_template() {
  if template_exists "SHIPMENT" "shipment_default"; then
    skip "published template exists: SHIPMENT / shipment_default"
    return 0
  fi

  step "Seed published template SHIPMENT / shipment_default"
  psql_exec <<SQL
INSERT INTO lowcode.low_code_configurations (
  id, tenant_id, code, name, config_type, status, version, published_at
) VALUES (
  'b2222222-2222-4222-8222-222222222201',
  '${TENANT_ID}',
  'cfg_shipment_default',
  'Shipment Default Configuration',
  'FORM_TEMPLATE',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_templates (
  id, tenant_id, configuration_id, entity_type, code, name, status, version, published_at
) VALUES (
  'b2222222-2222-4222-8222-222222222202',
  '${TENANT_ID}',
  'b2222222-2222-4222-8222-222222222201',
  'SHIPMENT',
  'shipment_default',
  'Shipment Default Form',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_sections (
  id, tenant_id, form_template_id, code, title, sort_order
) VALUES (
  'b2222222-2222-4222-8222-222222222203',
  '${TENANT_ID}',
  'b2222222-2222-4222-8222-222222222202',
  'operations',
  'Operations',
  100
) ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  sort_order = EXCLUDED.sort_order;

INSERT INTO lowcode.form_fields (
  id, tenant_id, form_template_id, section_id, code, label, field_type,
  required, read_only, system_field, options_json, sort_order
) VALUES
(
  'b2222222-2222-4222-8222-222222222204',
  '${TENANT_ID}',
  'b2222222-2222-4222-8222-222222222202',
  'b2222222-2222-4222-8222-222222222203',
  'temperature_mode',
  'Temperature mode',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"FROZEN","label":"Frozen"},{"value":"CHILLED","label":"Chilled"},{"value":"AMBIENT","label":"Ambient"}]}'::jsonb,
  100
),
(
  'b2222222-2222-4222-8222-222222222205',
  '${TENANT_ID}',
  'b2222222-2222-4222-8222-222222222202',
  'b2222222-2222-4222-8222-222222222203',
  'loading_contact_phone',
  'Loading contact phone',
  'TEXT',
  false,
  false,
  false,
  NULL,
  110
),
(
  'b2222222-2222-4222-8222-222222222206',
  '${TENANT_ID}',
  'b2222222-2222-4222-8222-222222222202',
  'b2222222-2222-4222-8222-222222222203',
  'driver_comment',
  'Driver comment',
  'TEXT',
  false,
  false,
  false,
  NULL,
  120
)
ON CONFLICT (id) DO UPDATE SET
  label = EXCLUDED.label,
  field_type = EXCLUDED.field_type,
  options_json = EXCLUDED.options_json,
  sort_order = EXCLUDED.sort_order;
SQL
  pass "created SHIPMENT / shipment_default"
}

seed_billing_register_template() {
  if template_exists "BILLING_REGISTER" "billing_register_default"; then
    skip "published template exists: BILLING_REGISTER / billing_register_default"
    return 0
  fi

  step "Seed published template BILLING_REGISTER / billing_register_default"
  psql_exec <<SQL
INSERT INTO lowcode.low_code_configurations (
  id, tenant_id, code, name, config_type, status, version, published_at
) VALUES (
  'b3333333-3333-4333-8333-333333333301',
  '${TENANT_ID}',
  'cfg_billing_register_default',
  'Billing Register Default Configuration',
  'FORM_TEMPLATE',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_templates (
  id, tenant_id, configuration_id, entity_type, code, name, status, version, published_at
) VALUES (
  'b3333333-3333-4333-8333-333333333302',
  '${TENANT_ID}',
  'b3333333-3333-4333-8333-333333333301',
  'BILLING_REGISTER',
  'billing_register_default',
  'Billing Register Default Form',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_sections (
  id, tenant_id, form_template_id, code, title, sort_order
) VALUES (
  'b3333333-3333-4333-8333-333333333303',
  '${TENANT_ID}',
  'b3333333-3333-4333-8333-333333333302',
  'finance',
  'Finance',
  100
) ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  sort_order = EXCLUDED.sort_order;

INSERT INTO lowcode.form_fields (
  id, tenant_id, form_template_id, section_id, code, label, field_type,
  required, read_only, system_field, options_json, sort_order
) VALUES
(
  'b3333333-3333-4333-8333-333333333304',
  '${TENANT_ID}',
  'b3333333-3333-4333-8333-333333333302',
  'b3333333-3333-4333-8333-333333333303',
  'cost_allocation_code',
  'Cost allocation code',
  'TEXT',
  false,
  false,
  false,
  NULL,
  100
),
(
  'b3333333-3333-4333-8333-333333333305',
  '${TENANT_ID}',
  'b3333333-3333-4333-8333-333333333302',
  'b3333333-3333-4333-8333-333333333303',
  'approval_group',
  'Approval group',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"LOGISTICS_FINANCE","label":"Logistics Finance"},{"value":"FINANCE","label":"Finance"},{"value":"OPS","label":"Operations"},{"value":"MANAGEMENT","label":"Management"}]}'::jsonb,
  110
),
(
  'b3333333-3333-4333-8333-333333333306',
  '${TENANT_ID}',
  'b3333333-3333-4333-8333-333333333302',
  'b3333333-3333-4333-8333-333333333303',
  'payment_priority',
  'Payment priority',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"LOW","label":"Low"},{"value":"NORMAL","label":"Normal"},{"value":"HIGH","label":"High"}]}'::jsonb,
  120
)
ON CONFLICT (id) DO UPDATE SET
  label = EXCLUDED.label,
  field_type = EXCLUDED.field_type,
  options_json = EXCLUDED.options_json,
  sort_order = EXCLUDED.sort_order;
SQL
  pass "created BILLING_REGISTER / billing_register_default"
}

seed_freight_request_template() {
  if template_exists "FREIGHT_REQUEST" "freight_request_default"; then
    skip "published template exists: FREIGHT_REQUEST / freight_request_default"
    return 0
  fi

  step "Seed published template FREIGHT_REQUEST / freight_request_default"
  psql_exec <<SQL
INSERT INTO lowcode.low_code_configurations (
  id, tenant_id, code, name, config_type, status, version, published_at
) VALUES (
  'b4444444-4444-4444-8444-444444444401',
  '${TENANT_ID}',
  'cfg_freight_request_default',
  'Freight Request Default Configuration',
  'FORM_TEMPLATE',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_templates (
  id, tenant_id, configuration_id, entity_type, code, name, status, version, published_at
) VALUES (
  'b4444444-4444-4444-8444-444444444402',
  '${TENANT_ID}',
  'b4444444-4444-4444-8444-444444444401',
  'FREIGHT_REQUEST',
  'freight_request_default',
  'Freight Request Default Form',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_sections (
  id, tenant_id, form_template_id, code, title, sort_order
) VALUES (
  'b4444444-4444-4444-8444-444444444403',
  '${TENANT_ID}',
  'b4444444-4444-4444-8444-444444444402',
  'tender',
  'Tender',
  100
) ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  sort_order = EXCLUDED.sort_order;

INSERT INTO lowcode.form_fields (
  id, tenant_id, form_template_id, section_id, code, label, field_type,
  required, read_only, system_field, options_json, sort_order
) VALUES
(
  'b4444444-4444-4444-8444-444444444404',
  '${TENANT_ID}',
  'b4444444-4444-4444-8444-444444444402',
  'b4444444-4444-4444-8444-444444444403',
  'lane_priority',
  'Lane priority',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"LOW","label":"Low"},{"value":"NORMAL","label":"Normal"},{"value":"HIGH","label":"High"}]}'::jsonb,
  100
),
(
  'b4444444-4444-4444-8444-444444444405',
  '${TENANT_ID}',
  'b4444444-4444-4444-8444-444444444402',
  'b4444444-4444-4444-8444-444444444403',
  'special_instructions',
  'Special instructions',
  'TEXT',
  false,
  false,
  false,
  NULL,
  110
)
ON CONFLICT (id) DO UPDATE SET
  label = EXCLUDED.label,
  field_type = EXCLUDED.field_type,
  options_json = EXCLUDED.options_json,
  sort_order = EXCLUDED.sort_order;
SQL
  pass "created FREIGHT_REQUEST / freight_request_default"
}

seed_document_template() {
  if template_exists "DOCUMENT" "document_default"; then
    skip "published template exists: DOCUMENT / document_default"
    return 0
  fi

  step "Seed published template DOCUMENT / document_default"
  psql_exec <<SQL
INSERT INTO lowcode.low_code_configurations (
  id, tenant_id, code, name, config_type, status, version, published_at
) VALUES (
  'b5555555-5555-4555-8555-555555555501',
  '${TENANT_ID}',
  'cfg_document_default',
  'Document Default Configuration',
  'FORM_TEMPLATE',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_templates (
  id, tenant_id, configuration_id, entity_type, code, name, status, version, published_at
) VALUES (
  'b5555555-5555-4555-8555-555555555502',
  '${TENANT_ID}',
  'b5555555-5555-4555-8555-555555555501',
  'DOCUMENT',
  'document_default',
  'Document Default Form',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_sections (
  id, tenant_id, form_template_id, code, title, sort_order
) VALUES (
  'b5555555-5555-4555-8555-555555555503',
  '${TENANT_ID}',
  'b5555555-5555-4555-8555-555555555502',
  'archive',
  'Archive',
  100
) ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  sort_order = EXCLUDED.sort_order;

INSERT INTO lowcode.form_fields (
  id, tenant_id, form_template_id, section_id, code, label, field_type,
  required, read_only, system_field, options_json, sort_order
) VALUES
(
  'b5555555-5555-4555-8555-555555555504',
  '${TENANT_ID}',
  'b5555555-5555-4555-8555-555555555502',
  'b5555555-5555-4555-8555-555555555503',
  'archive_reference',
  'Archive reference',
  'TEXT',
  false,
  false,
  false,
  NULL,
  100
),
(
  'b5555555-5555-4555-8555-555555555505',
  '${TENANT_ID}',
  'b5555555-5555-4555-8555-555555555502',
  'b5555555-5555-4555-8555-555555555503',
  'document_category',
  'Document category',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"OPERATIONAL","label":"Operational"},{"value":"FINANCE","label":"Finance"},{"value":"LEGAL","label":"Legal"}]}'::jsonb,
  110
)
ON CONFLICT (id) DO UPDATE SET
  label = EXCLUDED.label,
  field_type = EXCLUDED.field_type,
  options_json = EXCLUDED.options_json,
  sort_order = EXCLUDED.sort_order;
SQL
  pass "created DOCUMENT / document_default"
}

seed_rfx_template() {
  if template_exists "RFX" "rfx_default"; then
    skip "published template exists: RFX / rfx_default"
    return 0
  fi

  step "Seed published template RFX / rfx_default"
  psql_exec <<SQL
INSERT INTO lowcode.low_code_configurations (
  id, tenant_id, code, name, config_type, status, version, published_at
) VALUES (
  'b6666666-6666-4666-8666-666666666601',
  '${TENANT_ID}',
  'cfg_rfx_default',
  'RFX Default Configuration',
  'FORM_TEMPLATE',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_templates (
  id, tenant_id, configuration_id, entity_type, code, name, status, version, published_at
) VALUES (
  'b6666666-6666-4666-8666-666666666602',
  '${TENANT_ID}',
  'b6666666-6666-4666-8666-666666666601',
  'RFX',
  'rfx_default',
  'RFX Default Form',
  'PUBLISHED',
  1,
  now()
) ON CONFLICT (id) DO UPDATE SET
  status = EXCLUDED.status,
  published_at = EXCLUDED.published_at,
  name = EXCLUDED.name;

INSERT INTO lowcode.form_sections (
  id, tenant_id, form_template_id, code, title, sort_order
) VALUES (
  'b6666666-6666-4666-8666-666666666603',
  '${TENANT_ID}',
  'b6666666-6666-4666-8666-666666666602',
  'procurement',
  'Procurement',
  100
) ON CONFLICT (id) DO UPDATE SET
  title = EXCLUDED.title,
  sort_order = EXCLUDED.sort_order;

INSERT INTO lowcode.form_fields (
  id, tenant_id, form_template_id, section_id, code, label, field_type,
  required, read_only, system_field, options_json, sort_order
) VALUES
(
  'b6666666-6666-4666-8666-666666666604',
  '${TENANT_ID}',
  'b6666666-6666-4666-8666-666666666602',
  'b6666666-6666-4666-8666-666666666603',
  'evaluation_criteria',
  'Evaluation criteria',
  'TEXT',
  false,
  false,
  false,
  NULL,
  100
),
(
  'b6666666-6666-4666-8666-666666666605',
  '${TENANT_ID}',
  'b6666666-6666-4666-8666-666666666602',
  'b6666666-6666-4666-8666-666666666603',
  'confidentiality_level',
  'Confidentiality level',
  'SELECT',
  false,
  false,
  false,
  '{"options":[{"value":"PUBLIC","label":"Public"},{"value":"INTERNAL","label":"Internal"},{"value":"RESTRICTED","label":"Restricted"}]}'::jsonb,
  110
)
ON CONFLICT (id) DO UPDATE SET
  label = EXCLUDED.label,
  field_type = EXCLUDED.field_type,
  options_json = EXCLUDED.options_json,
  sort_order = EXCLUDED.sort_order;
SQL
  pass "created RFX / rfx_default"
}

print_summary() {
  psql_exec -c "
    SELECT entity_type, code, status, version
    FROM lowcode.form_templates
    WHERE tenant_id = '${TENANT_ID}'
      AND status = 'PUBLISHED'
    ORDER BY entity_type, code;
  "
}

lookup_entity_id() {
  local sql="$1"
  psql_exec -t -A -c "$sql" | tr -d '[:space:]'
}

custom_value_exists() {
  local entity_type="$1"
  local entity_id="$2"
  local field_code="$3"
  local count
  count="$(psql_exec -t -A -c "
    SELECT COUNT(*)
    FROM lowcode.custom_field_values
    WHERE tenant_id = '${TENANT_ID}'
      AND entity_type = '${entity_type}'
      AND entity_id = '${entity_id}'
      AND field_code = '${field_code}';
  " | tr -d '[:space:]')"
  [[ "${count:-0}" != "0" ]]
}

upsert_custom_value() {
  local entity_type="$1"
  local entity_id="$2"
  local form_template_id="$3"
  local field_id="$4"
  local field_code="$5"
  local value_json="$6"

  psql_exec -c "
    INSERT INTO lowcode.custom_field_values (
      tenant_id, entity_type, entity_id, form_template_id, field_id, field_code, value_json
    ) VALUES (
      '${TENANT_ID}',
      '${entity_type}',
      '${entity_id}',
      '${form_template_id}',
      '${field_id}',
      '${field_code}',
      ${value_json}
    )
    ON CONFLICT (tenant_id, entity_type, entity_id, field_id)
    DO UPDATE SET
      form_template_id = EXCLUDED.form_template_id,
      field_code = EXCLUDED.field_code,
      value_json = EXCLUDED.value_json,
      updated_at = now();
  " >/dev/null
}

seed_custom_field_values() {
  step "Seed demo custom field values (dev-only psql)"

  psql_exec -c "
    UPDATE lowcode.form_fields
    SET options_json = '{\"options\":[{\"value\":\"GENERAL\",\"label\":\"General\"},{\"value\":\"A\",\"label\":\"Class A\"},{\"value\":\"B\",\"label\":\"Class B\"},{\"value\":\"C\",\"label\":\"Class C\"}]}'::jsonb
    WHERE id = 'b1111111-1111-4111-8111-111111111104';

    UPDATE lowcode.form_fields
    SET options_json = '{\"options\":[{\"value\":\"LOGISTICS_FINANCE\",\"label\":\"Logistics Finance\"},{\"value\":\"FINANCE\",\"label\":\"Finance\"},{\"value\":\"OPS\",\"label\":\"Operations\"},{\"value\":\"MANAGEMENT\",\"label\":\"Management\"}]}'::jsonb
    WHERE id = 'b3333333-3333-4333-8333-333333333305';
  " >/dev/null

  local to_id sh_id br_id fr_id doc_id rfx_id
  to_id="$(lookup_entity_id "SELECT id FROM transport.transport_orders WHERE tenant_id = '${TENANT_ID}' AND order_number = 'DEMO-TO-001' LIMIT 1;")"
  sh_id="$(lookup_entity_id "SELECT id FROM transport.shipments WHERE tenant_id = '${TENANT_ID}' AND shipment_number = 'DEMO-SH-PLANNED' LIMIT 1;")"
  br_id="$(lookup_entity_id "SELECT id FROM billing.billing_registers WHERE tenant_id = '${TENANT_ID}' AND register_number = 'DEMO-BR-001' LIMIT 1;")"
  fr_id="$(lookup_entity_id "SELECT id FROM rfx.freight_requests WHERE tenant_id = '${TENANT_ID}' AND freight_request_number = 'DEMO-FR-001' LIMIT 1;")"
  doc_id="$(lookup_entity_id "SELECT id FROM documents.documents WHERE tenant_id = '${TENANT_ID}' AND document_number = 'DEMO-DOC-001' LIMIT 1;")"
  rfx_id="$(lookup_entity_id "SELECT id FROM rfx.rfx_events WHERE tenant_id = '${TENANT_ID}' AND rfx_number = 'DEMO-RFX-001' LIMIT 1;")"

  if [[ -z "$to_id" || -z "$sh_id" || -z "$br_id" ]]; then
    echo "WARN: core demo entities not found — run make seed-demo-data first" >&2
  fi

  if custom_value_exists "TRANSPORT_ORDER" "$to_id" "cargo_class"; then
    skip "custom field values exist for TRANSPORT_ORDER DEMO-TO-001"
  else
    upsert_custom_value "TRANSPORT_ORDER" "$to_id" "b1111111-1111-4111-8111-111111111102" "b1111111-1111-4111-8111-111111111104" "cargo_class" "to_jsonb('GENERAL'::text)"
    upsert_custom_value "TRANSPORT_ORDER" "$to_id" "b1111111-1111-4111-8111-111111111102" "b1111111-1111-4111-8111-111111111105" "internal_cost_center" "to_jsonb('CC-1001'::text)"
    upsert_custom_value "TRANSPORT_ORDER" "$to_id" "b1111111-1111-4111-8111-111111111102" "b1111111-1111-4111-8111-111111111106" "loading_window_note" "to_jsonb('Окно погрузки 09:00–12:00'::text)"
    pass "custom field values seeded for TRANSPORT_ORDER DEMO-TO-001"
  fi

  if custom_value_exists "SHIPMENT" "$sh_id" "temperature_mode"; then
    skip "custom field values exist for SHIPMENT DEMO-SH-PLANNED"
  else
    upsert_custom_value "SHIPMENT" "$sh_id" "b2222222-2222-4222-8222-222222222202" "b2222222-2222-4222-8222-222222222204" "temperature_mode" "to_jsonb('AMBIENT'::text)"
    upsert_custom_value "SHIPMENT" "$sh_id" "b2222222-2222-4222-8222-222222222202" "b2222222-2222-4222-8222-222222222205" "loading_contact_phone" "to_jsonb('+7 900 000-00-01'::text)"
    upsert_custom_value "SHIPMENT" "$sh_id" "b2222222-2222-4222-8222-222222222202" "b2222222-2222-4222-8222-222222222206" "driver_comment" "to_jsonb('Позвонить за 1 час до прибытия'::text)"
    pass "custom field values seeded for SHIPMENT DEMO-SH-PLANNED"
  fi

  if custom_value_exists "BILLING_REGISTER" "$br_id" "cost_allocation_code"; then
    skip "custom field values exist for BILLING_REGISTER DEMO-BR-001"
  else
    upsert_custom_value "BILLING_REGISTER" "$br_id" "b3333333-3333-4333-8333-333333333302" "b3333333-3333-4333-8333-333333333304" "cost_allocation_code" "to_jsonb('FIN-LOG-001'::text)"
    upsert_custom_value "BILLING_REGISTER" "$br_id" "b3333333-3333-4333-8333-333333333302" "b3333333-3333-4333-8333-333333333305" "approval_group" "to_jsonb('LOGISTICS_FINANCE'::text)"
    upsert_custom_value "BILLING_REGISTER" "$br_id" "b3333333-3333-4333-8333-333333333302" "b3333333-3333-4333-8333-333333333306" "payment_priority" "to_jsonb('NORMAL'::text)"
    pass "custom field values seeded for BILLING_REGISTER DEMO-BR-001"
  fi

  if [[ -n "$fr_id" ]]; then
    if custom_value_exists "FREIGHT_REQUEST" "$fr_id" "lane_priority"; then
      skip "custom field values exist for FREIGHT_REQUEST DEMO-FR-001"
    else
      upsert_custom_value "FREIGHT_REQUEST" "$fr_id" "b4444444-4444-4444-8444-444444444402" "b4444444-4444-4444-8444-444444444404" "lane_priority" "to_jsonb('HIGH'::text)"
      upsert_custom_value "FREIGHT_REQUEST" "$fr_id" "b4444444-4444-4444-8444-444444444402" "b4444444-4444-4444-8444-444444444405" "special_instructions" "to_jsonb('Требуется фиксированная ставка на квартал'::text)"
      pass "custom field values seeded for FREIGHT_REQUEST DEMO-FR-001"
    fi
  else
    echo "WARN: DEMO-FR-001 not found — skip FREIGHT_REQUEST custom field values" >&2
  fi

  if [[ -n "$doc_id" ]]; then
    if custom_value_exists "DOCUMENT" "$doc_id" "archive_reference"; then
      skip "custom field values exist for DOCUMENT DEMO-DOC-001"
    else
      upsert_custom_value "DOCUMENT" "$doc_id" "b5555555-5555-4555-8555-555555555502" "b5555555-5555-4555-8555-555555555504" "archive_reference" "to_jsonb('ARC-2026-001'::text)"
      upsert_custom_value "DOCUMENT" "$doc_id" "b5555555-5555-4555-8555-555555555502" "b5555555-5555-4555-8555-555555555505" "document_category" "to_jsonb('OPERATIONAL'::text)"
      pass "custom field values seeded for DOCUMENT DEMO-DOC-001"
    fi
  else
    echo "WARN: DEMO-DOC-001 not found — skip DOCUMENT custom field values" >&2
  fi

  if [[ -n "$rfx_id" ]]; then
    if custom_value_exists "RFX" "$rfx_id" "evaluation_criteria"; then
      skip "custom field values exist for RFX DEMO-RFX-001"
    else
      upsert_custom_value "RFX" "$rfx_id" "b6666666-6666-4666-8666-666666666602" "b6666666-6666-4666-8666-666666666604" "evaluation_criteria" "to_jsonb('Price 60%, SLA 25%, experience 15%'::text)"
      upsert_custom_value "RFX" "$rfx_id" "b6666666-6666-4666-8666-666666666602" "b6666666-6666-4666-8666-666666666605" "confidentiality_level" "to_jsonb('INTERNAL'::text)"
      pass "custom field values seeded for RFX DEMO-RFX-001"
    fi
  else
    echo "WARN: DEMO-RFX-001 not found — skip RFX custom field values" >&2
  fi
}

main() {
  echo ""
  echo "Freight Platform — seed low-code demo templates"
  echo "Tenant: ${TENANT_ID}"
  echo "Dev-only: writes to lowcode schema via psql in Docker"
  echo ""

  require_postgres
  seed_transport_order_template
  seed_shipment_template
  seed_billing_register_template
  seed_freight_request_template
  seed_document_template
  seed_rfx_template
  seed_custom_field_values
  step "Published form templates"
  print_summary
  pass "Low-code demo seed completed"
}

main "$@"
