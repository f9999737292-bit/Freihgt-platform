SELECT execution_id::text
    || '|' || coalesce(owner_epoch::text, 'null')
    || '|' || coalesce(current_revision_id::text, 'null')
    || '|' || coalesce(revision_version::text, 'null')
    || '|' || coalesce(operating_tenant_id::text, 'null')
  FROM transport.read_binding_assignment(:'execution'::uuid, :'shipment'::uuid);
