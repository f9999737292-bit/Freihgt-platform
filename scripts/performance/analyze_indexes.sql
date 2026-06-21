\echo '=== Table sizes (largest first) ==='
SELECT schemaname, relname, pg_size_pretty(pg_total_relation_size(relid)) AS size
FROM pg_catalog.pg_statio_user_tables
ORDER BY pg_total_relation_size(relid) DESC;

\echo ''
\echo '=== Indexes in application schemas ==='
SELECT schemaname, tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname IN ('core', 'transport', 'rfx', 'documents', 'billing')
ORDER BY schemaname, tablename, indexname;

\echo ''
\echo '=== Tables in application schemas without tenant_id index ==='
SELECT t.schemaname, t.tablename
FROM pg_tables t
WHERE t.schemaname IN ('core', 'transport', 'rfx', 'documents', 'billing')
  AND EXISTS (
    SELECT 1
    FROM information_schema.columns c
    WHERE c.table_schema = t.schemaname
      AND c.table_name = t.tablename
      AND c.column_name = 'tenant_id'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM pg_indexes i
    WHERE i.schemaname = t.schemaname
      AND i.tablename = t.tablename
      AND i.indexdef ILIKE '%(tenant_id%'
  )
ORDER BY t.schemaname, t.tablename;

\echo ''
\echo '=== Expected filter indexes (missing = action required) ==='
WITH expected(schema_name, table_name, column_name) AS (
  VALUES
    ('core', 'companies', 'tenant_id'),
    ('core', 'companies', 'company_type'),
    ('core', 'companies', 'status'),
    ('core', 'companies', 'tax_id'),
    ('core', 'users', 'tenant_id'),
    ('core', 'users', 'email'),
    ('core', 'users', 'status'),
    ('transport', 'transport_orders', 'tenant_id'),
    ('transport', 'transport_orders', 'shipper_company_id'),
    ('transport', 'transport_orders', 'consignee_company_id'),
    ('transport', 'transport_orders', 'status'),
    ('transport', 'transport_orders', 'requested_pickup_date'),
    ('rfx', 'freight_requests', 'tenant_id'),
    ('rfx', 'freight_requests', 'shipper_company_id'),
    ('rfx', 'freight_requests', 'status'),
    ('rfx', 'freight_requests', 'request_type'),
    ('rfx', 'bids', 'tenant_id'),
    ('rfx', 'bids', 'freight_request_id'),
    ('rfx', 'bids', 'carrier_company_id'),
    ('rfx', 'bids', 'status'),
    ('transport', 'shipments', 'tenant_id'),
    ('transport', 'shipments', 'shipper_company_id'),
    ('transport', 'shipments', 'consignee_company_id'),
    ('transport', 'shipments', 'carrier_company_id'),
    ('transport', 'shipments', 'status'),
    ('documents', 'documents', 'tenant_id'),
    ('documents', 'documents', 'document_type'),
    ('documents', 'documents', 'document_status'),
    ('documents', 'documents', 'related_entity_type'),
    ('documents', 'documents', 'related_entity_id'),
    ('billing', 'billing_registers', 'tenant_id'),
    ('billing', 'billing_registers', 'customer_company_id'),
    ('billing', 'billing_registers', 'contractor_company_id'),
    ('billing', 'billing_registers', 'status'),
    ('billing', 'billing_registers', 'period_from'),
    ('billing', 'billing_registers', 'period_to'),
    ('billing', 'billing_register_items', 'tenant_id'),
    ('billing', 'billing_register_items', 'register_id'),
    ('billing', 'billing_register_items', 'shipment_id')
)
SELECT e.schema_name, e.table_name, e.column_name, 'MISSING' AS status
FROM expected e
WHERE NOT EXISTS (
  SELECT 1
  FROM pg_indexes i
  WHERE i.schemaname = e.schema_name
    AND i.tablename = e.table_name
    AND (
      i.indexdef ILIKE '%(' || e.column_name || '%'
      OR i.indexdef ILIKE '%(' || e.column_name || ',%'
      OR i.indexdef ILIKE '%, ' || e.column_name || '%'
      OR i.indexdef ILIKE '%, ' || e.column_name || ')%'
    )
)
ORDER BY e.schema_name, e.table_name, e.column_name;
