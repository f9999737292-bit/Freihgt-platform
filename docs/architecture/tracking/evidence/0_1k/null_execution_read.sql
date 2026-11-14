SELECT coalesce(execution_id::text, 'null')
  FROM transport.read_binding_assignment(
      NULL,
      '00000000-0000-0000-0000-000000000006');
