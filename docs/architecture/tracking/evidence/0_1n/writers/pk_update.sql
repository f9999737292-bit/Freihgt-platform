-- UPDATE of the referenced execution primary key. Product Go does not do this.
-- Children use ON UPDATE NO ACTION, so the statement must fail with 23503.
BEGIN;
UPDATE transport.transport_executions
   SET id = 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee'
 WHERE id = '11111111-1111-1111-1111-111111111111';
COMMIT;
