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

