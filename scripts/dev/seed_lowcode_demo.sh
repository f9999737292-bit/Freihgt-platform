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
  '{"options":[{"value":"A","label":"Class A"},{"value":"B","label":"Class B"},{"value":"C","label":"Class C"}]}'::jsonb,
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
  '{"options":[{"value":"FINANCE","label":"Finance"},{"value":"OPS","label":"Operations"},{"value":"MANAGEMENT","label":"Management"}]}'::jsonb,
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

print_summary() {
  psql_exec -c "
    SELECT entity_type, code, status, version
    FROM lowcode.form_templates
    WHERE tenant_id = '${TENANT_ID}'
      AND status = 'PUBLISHED'
    ORDER BY entity_type, code;
  "
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
  step "Published form templates"
  print_summary
  pass "Low-code demo seed completed"
}

main "$@"
