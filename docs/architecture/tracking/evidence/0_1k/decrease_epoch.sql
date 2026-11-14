UPDATE transport.transport_executions
   SET owner_epoch = owner_epoch - 1,
       updated_at = clock_timestamp()
 WHERE id = '11111111-1111-1111-1111-111111111111';
