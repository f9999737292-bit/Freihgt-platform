SET ROLE binding_authority_fence;
SELECT fn_user || '|' || fn_session || '|' || fn_path || '|' || driver_id::text AS identity_row
  FROM transport.read_binding_assignment('22222222-2222-2222-2222-222222222222');

