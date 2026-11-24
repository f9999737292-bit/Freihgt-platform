CREATE OR REPLACE FUNCTION transport.privilege_identity()
RETURNS TABLE (fn_user text, fn_session text, fn_path text, driver_id uuid)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $fn$
BEGIN
  RETURN QUERY
  SELECT current_user::text,
         session_user::text,
         current_setting('search_path'),
         s.driver_id
    FROM transport.shipments AS s
   WHERE s.id = '22222222-2222-2222-2222-222222222222'::uuid;
END;
$fn$;
ALTER FUNCTION transport.privilege_identity() OWNER TO shipment_assignment_reader;
REVOKE ALL ON FUNCTION transport.privilege_identity() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION transport.privilege_identity() TO binding_authority_fence;
