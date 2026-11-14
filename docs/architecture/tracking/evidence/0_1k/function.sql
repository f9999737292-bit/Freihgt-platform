-- Test bodies. Neither function is installed in a BINTRANS database.
-- read_binding_assignment_0_1j is the 0.1J body: it locks the input execution
-- and then returns whichever execution the active link names.
-- read_binding_assignment adds the 0.1K checks. It joins only the active-slot
-- table, requires the input execution to be that slot, requires a non-null
-- bigint owner_epoch, and requires the current revision to be the ACTIVE row
-- for that execution and operating tenant.

SET ROLE assignment_lock_owner;

CREATE FUNCTION transport.read_binding_assignment_0_1j(p_execution_id uuid, p_shipment_id uuid)
    RETURNS TABLE (
        shipment_id uuid,
        shipment_tenant_id uuid,
        driver_id uuid,
        vehicle_id uuid,
        shipment_version integer,
        shipment_deleted boolean,
        driver_status text,
        execution_id uuid,
        operating_tenant_id uuid,
        execution_driver_id uuid,
        execution_vehicle_id uuid,
        current_revision_id uuid,
        revision_version integer
    )
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $fn$
DECLARE
    v_driver uuid;
    v_vehicle uuid;
BEGIN
    IF p_execution_id IS NOT NULL THEN
        PERFORM 1
          FROM transport.transport_executions
         WHERE id = p_execution_id
           FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing execution' USING ERRCODE = 'P0002';
        END IF;
    END IF;
    PERFORM 1
      FROM transport.shipments
     WHERE id = p_shipment_id
       AND deleted_at IS NULL
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'missing shipment' USING ERRCODE = 'P0002';
    END IF;
    SELECT s.driver_id, s.vehicle_id
      INTO v_driver, v_vehicle
      FROM transport.shipments AS s
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;
    IF v_driver IS NOT NULL THEN
        PERFORM 1 FROM transport.drivers WHERE id = v_driver FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing driver' USING ERRCODE = 'P0002';
        END IF;
    END IF;
    IF v_vehicle IS NOT NULL THEN
        PERFORM 1 FROM transport.vehicles WHERE id = v_vehicle FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing vehicle' USING ERRCODE = 'P0002';
        END IF;
    END IF;
    RETURN QUERY
    SELECT s.id, s.tenant_id, s.driver_id, s.vehicle_id, s.version, false,
           d.status::text,
           e.id, e.operating_tenant_id, e.driver_id, e.vehicle_id, e.current_revision_id,
           r.version
      FROM transport.shipments AS s
      LEFT JOIN transport.drivers AS d
        ON d.id = s.driver_id
      LEFT JOIN transport.transport_execution_active_shipments AS link
        ON link.shipment_tenant_id = s.tenant_id
       AND link.shipment_id = s.id
      LEFT JOIN transport.transport_executions AS e
        ON e.id = link.execution_id
      LEFT JOIN transport.transport_execution_revisions AS r
        ON r.id = e.current_revision_id
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;
END;
$fn$;

CREATE FUNCTION transport.read_binding_assignment(p_execution_id uuid, p_shipment_id uuid)
    RETURNS TABLE (
        shipment_id uuid,
        shipment_tenant_id uuid,
        driver_id uuid,
        vehicle_id uuid,
        shipment_version integer,
        shipment_deleted boolean,
        driver_status text,
        execution_id uuid,
        operating_tenant_id uuid,
        execution_driver_id uuid,
        execution_vehicle_id uuid,
        current_revision_id uuid,
        revision_version integer,
        owner_epoch bigint
    )
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = pg_catalog
AS $fn$
DECLARE
    v_driver uuid;
    v_vehicle uuid;
    v_link uuid;
    v_epoch bigint;
    v_revision uuid;
    v_tenant uuid;
    v_links integer;
    v_active_revisions integer;
BEGIN
    IF p_execution_id IS NOT NULL THEN
        PERFORM 1
          FROM transport.transport_executions
         WHERE id = p_execution_id
           FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing execution' USING ERRCODE = 'P0002';
        END IF;
    END IF;

    PERFORM 1
      FROM transport.shipments
     WHERE id = p_shipment_id
       AND deleted_at IS NULL
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'missing shipment' USING ERRCODE = 'P0002';
    END IF;

    SELECT s.driver_id, s.vehicle_id
      INTO v_driver, v_vehicle
      FROM transport.shipments AS s
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;

    IF v_driver IS NOT NULL THEN
        PERFORM 1
          FROM transport.drivers
         WHERE id = v_driver
           FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing driver' USING ERRCODE = 'P0002';
        END IF;
    END IF;
    IF v_vehicle IS NOT NULL THEN
        PERFORM 1
          FROM transport.vehicles
         WHERE id = v_vehicle
           FOR SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'missing vehicle' USING ERRCODE = 'P0002';
        END IF;
    END IF;

    SELECT count(*)::integer
      INTO v_links
      FROM transport.transport_execution_active_shipments AS link
      JOIN transport.shipments AS s
        ON s.id = link.shipment_id
       AND s.tenant_id = link.shipment_tenant_id
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;
    IF v_links > 1 THEN
        RAISE EXCEPTION 'ambiguous active link' USING ERRCODE = '21000';
    END IF;

    SELECT link.execution_id
      INTO v_link
      FROM transport.transport_execution_active_shipments AS link
      JOIN transport.shipments AS s
        ON s.id = link.shipment_id
       AND s.tenant_id = link.shipment_tenant_id
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL;

    IF p_execution_id IS NOT NULL THEN
        IF v_link IS NULL THEN
            RAISE EXCEPTION 'no active link for execution target' USING ERRCODE = 'P0002';
        END IF;
        IF v_link IS DISTINCT FROM p_execution_id THEN
            RAISE EXCEPTION 'input execution is not the active link' USING ERRCODE = '23514';
        END IF;

        SELECT e.owner_epoch, e.current_revision_id, e.operating_tenant_id
          INTO v_epoch, v_revision, v_tenant
          FROM transport.transport_executions AS e
         WHERE e.id = p_execution_id;
        IF v_epoch IS NULL THEN
            RAISE EXCEPTION 'owner epoch is absent' USING ERRCODE = '42501';
        END IF;
        IF v_revision IS NULL THEN
            RAISE EXCEPTION 'current revision is absent' USING ERRCODE = 'P0002';
        END IF;
        IF NOT EXISTS (
            SELECT 1
              FROM transport.transport_execution_revisions AS revision
             WHERE revision.id = v_revision
               AND revision.execution_id = p_execution_id
               AND revision.operating_tenant_id = v_tenant
               AND revision.status = 'ACTIVE'
        ) THEN
            RAISE EXCEPTION 'current revision is not authoritative' USING ERRCODE = '23514';
        END IF;
        SELECT count(*)::integer
          INTO v_active_revisions
          FROM transport.transport_execution_revisions AS revision
         WHERE revision.execution_id = p_execution_id
           AND revision.status = 'ACTIVE';
        IF v_active_revisions > 1 THEN
            RAISE EXCEPTION 'ambiguous current revision' USING ERRCODE = '21000';
        END IF;
    END IF;

    RETURN QUERY
    SELECT s.id, s.tenant_id, s.driver_id, s.vehicle_id, s.version, false,
           d.status::text,
           e.id, e.operating_tenant_id, e.driver_id, e.vehicle_id, e.current_revision_id,
           r.version,
           e.owner_epoch
      FROM transport.shipments AS s
      LEFT JOIN transport.drivers AS d
        ON d.id = s.driver_id
      LEFT JOIN transport.transport_execution_active_shipments AS link
        ON link.shipment_tenant_id = s.tenant_id
       AND link.shipment_id = s.id
      LEFT JOIN transport.transport_executions AS e
        ON e.id = link.execution_id
       AND (p_execution_id IS NULL OR e.id = p_execution_id)
      LEFT JOIN transport.transport_execution_revisions AS r
        ON r.id = e.current_revision_id
       AND r.execution_id = e.id
       AND r.operating_tenant_id = e.operating_tenant_id
       AND r.status = 'ACTIVE'
     WHERE s.id = p_shipment_id
       AND s.deleted_at IS NULL
       AND (p_execution_id IS NULL OR link.execution_id = p_execution_id);
END;
$fn$;

RESET ROLE;
