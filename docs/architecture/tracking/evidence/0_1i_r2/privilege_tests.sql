-- E02, E03, E04, and E08. Expected errors are marked expect=42501.
-- The runner executes each block in its own psql session.

-- @test id=E02_column_select_for_share role=column_probe expect=42501
SET ROLE column_select_only;
SELECT id
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   FOR SHARE;
-- @end

-- @test id=E03_table_select_for_share role=table_probe expect=42501
SET ROLE table_select_only;
SELECT id
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   FOR SHARE;
-- @end

-- @admin id=E04_grant_update_updated_at
GRANT UPDATE (updated_at) ON transport.shipments TO assignment_lock_owner;
-- @end

-- @test id=E04_for_share_after_update_grant role=lock_probe expect=OK contains=22222222-2222-2222-2222-222222222222
SET ROLE assignment_lock_owner;
SELECT id
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222'
   FOR SHARE;
-- @end

-- @admin id=E04_pin_updated_at
UPDATE transport.shipments
   SET updated_at = '2000-01-01 00:00:00+00'
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @test id=E04_update_updated_at role=lock_probe expect=OK contains=UPDATE 1
SET ROLE assignment_lock_owner;
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @admin id=E04_updated_at_changed expect=OK contains=CHANGED
SELECT CASE
           WHEN updated_at > TIMESTAMPTZ '2000-01-02 00:00:00+00' THEN 'CHANGED'
           ELSE 'UNCHANGED'
       END AS updated_at_result
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @test id=E04_update_driver_id_denied role=lock_probe expect=42501
SET ROLE assignment_lock_owner;
UPDATE transport.shipments
   SET driver_id = driver_id
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @admin id=E08_grant_inherit_true
GRANT assignment_lock_owner TO inherit_probe WITH INHERIT TRUE, SET FALSE;
-- @end

-- @test id=E08_inherit_true_updates_updated_at role=inherit_probe expect=OK contains=UPDATE
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @admin id=E08_revoke_inherit
REVOKE assignment_lock_owner FROM inherit_probe;
-- @end

-- @test id=E08_inherit_after_revoke role=inherit_probe expect=42501
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @admin id=E08_grant_set_true
GRANT assignment_lock_owner TO set_probe WITH INHERIT FALSE, SET TRUE;
-- @end

-- @test id=E08_set_true_without_set_role role=set_probe expect=42501
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @test id=E08_set_role_then_update role=set_probe expect=OK contains=UPDATE
SET ROLE assignment_lock_owner;
UPDATE transport.shipments
   SET updated_at = clock_timestamp()
 WHERE id = '22222222-2222-2222-2222-222222222222';
-- @end

-- @admin id=E08_revoke_set_true
REVOKE assignment_lock_owner FROM set_probe;
-- @end

-- @test id=E08_set_role_after_revoke role=set_probe expect=42501
SET ROLE assignment_lock_owner;
-- @end

-- @admin id=E08_membership_after_revoke expect=OK contains=NO_MEMBERSHIP
SELECT CASE
           WHEN count(*) = 0 THEN 'NO_MEMBERSHIP'
           ELSE 'HAS_MEMBERSHIP'
       END AS membership_result
  FROM pg_auth_members AS member_row
  JOIN pg_roles AS granted_role ON granted_role.oid = member_row.roleid
  JOIN pg_roles AS member_role ON member_role.oid = member_row.member
 WHERE granted_role.rolname = 'assignment_lock_owner'
   AND member_role.rolname IN (
           'tracking_runtime',
           'binding_authority_fence',
           'inherit_probe',
           'set_probe',
           'runtime_login',
           'fence_probe'
       );
-- @end
