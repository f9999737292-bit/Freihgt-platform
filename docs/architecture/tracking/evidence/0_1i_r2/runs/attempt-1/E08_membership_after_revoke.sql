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

