SET ROLE assignment_lock_owner;
WITH before_row AS (
    SELECT updated_at AS ts
      FROM transport.shipments
     WHERE id = '22222222-2222-2222-2222-222222222222'
), changed AS (
    UPDATE transport.shipments
       SET updated_at = clock_timestamp()
     WHERE id = '22222222-2222-2222-2222-222222222222'
    RETURNING updated_at
)
SELECT CASE
           WHEN changed.updated_at IS DISTINCT FROM before_row.ts THEN 'CHANGED'
           ELSE 'UNCHANGED'
       END AS updated_at_result
  FROM before_row, changed;

