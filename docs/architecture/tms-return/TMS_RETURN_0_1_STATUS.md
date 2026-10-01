# TMS-RETURN-0.1 status

TMS_RETURN_0_1_IMPLEMENTED=YES
FULL_REJECTION_SUPPORTED=YES
PARTIAL_REJECTION_SUPPORTED=YES
RETURN_TO_ORIGIN_SUPPORTED=YES
REDIRECT_SUPPORTED=YES
HOLD_PENDING_DISPOSITION_SUPPORTED=YES
RETURN_FROM_ANY_DELIVERY_STOP=YES
MULTIPLE_RETURN_ORIGINS=YES
OTHER_STOPS_CONTINUE=YES
ONBOARD_CONTINUITY=YES
QUANTITY_CONSERVATION=YES
SUCCESSOR_REVISION_INTEGRATION=YES
COMPLETED_HISTORY_IMMUTABLE=YES
CONTROL_TOWER_PROJECTION=YES
DRIVER_REJECTION_REPORTING=YES
DRIVER_RETURN_AUTHORIZATION=NO
DRIVER_REDIRECT_AUTHORIZATION=NO
BILLING_RUNTIME_CHANGED=NO
EDO_RUNTIME_CHANGED=NO
NLO_0_4D_STARTED=NO
SHIPMENT_FSM_CHANGED=NO
READY_FOR_PRODUCTION_EXECUTION=NO
HANDLING_UNIT_ID_SUPPORT=DEFERRED
OPENAPI_CHANGE=NO
OPENAPI_REASON=INTERNAL_ONLY

Canonical quantity for v0.1 is `cargo.pallet_count` with UOM `PALLET`.
There is no handling-unit or pallet identity master. Those IDs stay null.
Rejected quantity remains onboard until a completed execution action resolves the return or redirect.
Hold does not create a successor revision.
Return and redirect call the TMS-MSTOP-0.1F successor writer in the same transaction and must include every still-open stop.
Terminal completed stops and actions are not rewritten and are not copied into the successor.
Control Tower projects disposition events and does not write execution revision, stop status, or current stop.
