-- Removes only TMS-MSTOP-0.1C driver stop tasks.
-- Notice transport.driver_tasks and 0.1A/0.1B execution tables stay.

DROP TABLE IF EXISTS transport.driver_stop_tasks;
