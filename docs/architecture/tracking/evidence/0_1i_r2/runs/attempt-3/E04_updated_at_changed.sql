SELECT CASE
           WHEN updated_at > TIMESTAMPTZ '2000-01-02 00:00:00+00' THEN 'CHANGED'
           ELSE 'UNCHANGED'
       END AS updated_at_result
  FROM transport.shipments
 WHERE id = '22222222-2222-2222-2222-222222222222';

