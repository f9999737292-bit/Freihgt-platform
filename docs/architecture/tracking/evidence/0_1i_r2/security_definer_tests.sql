-- E06 and E09. The function body is static SQL.
-- It is not the full design RETURN QUERY over executions, drivers, vehicles, and revisions.

-- @admin id=E06_create_function
GRANT CREATE ON SCHEMA transport TO assignment_lock_owner;
SET ROLE assignment_lock_owner;
CREATE FUNCTION transport.read_binding_assignment(p_shipment_id uuid)
RETURNS TABLE (fn_user text, fn_session text, fn_path text, driver_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
    PERFORM 1
      FROM transport.shipments
     WHERE id = p_shipment_id
       AND deleted_at IS NULL
       FOR SHARE;
    RETURN QUERY
    SELECT current_user::text,
           session_user::text,
           current_setting('search_path'),
           s.driver_id
      FROM transport.shipments AS s
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;
END;
$fn$;
RESET ROLE;
REVOKE CREATE ON SCHEMA transport FROM assignment_lock_owner;
REVOKE ALL ON FUNCTION transport.read_binding_assignment(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.read_binding_assignment(uuid) TO binding_authority_fence;
-- @end

-- @test id=E06_definer_identity role=fence_probe expect=OK contains=assignment_lock_owner|fence_probe|pg_catalog|33333333-3333-3333-3333-333333333333
SET ROLE binding_authority_fence;
SELECT fn_user || '|' || fn_session || '|' || fn_path || '|' || driver_id::text AS identity_row
  FROM transport.read_binding_assignment('22222222-2222-2222-2222-222222222222');
-- @end

-- @test id=E06_runtime_cannot_set_role role=runtime_login expect=42501
SET ROLE assignment_lock_owner;
-- @end

-- @test id=E06_runtime_cannot_update role=runtime_login expect=42501
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @test id=E09_public_execute_denied role=nopriv_login expect=42501
SELECT fn_user
  FROM transport.read_binding_assignment('22222222-2222-2222-2222-222222222222');
-- @end

-- @test id=E09_runtime_execute_denied role=runtime_login expect=42501
SELECT fn_user
  FROM transport.read_binding_assignment('22222222-2222-2222-2222-222222222222');
-- @end

-- @test id=E09_fence_cannot_set_role_owner role=fence_probe expect=42501
SET ROLE binding_authority_fence;
SET ROLE assignment_lock_owner;
-- @end

-- @test id=E09_no_schema_usage role=no_usage_probe expect=42501
SET ROLE no_usage_role;
SELECT id
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end
