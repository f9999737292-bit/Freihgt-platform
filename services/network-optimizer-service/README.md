# Network optimizer service

BNO-0.1A foundation for load opportunities and manual capacity.

Public routes are exposed by the API gateway at `/api/v1/network/...`. This process serves the same paths with the `/api` prefix removed, which is the existing gateway rewrite.

The service trusts `X-Tenant-ID` and `X-User-ID` only after the gateway has replaced client-supplied identity headers from the verified JWT. It does not read shipment tables across tenants. Publication of a transport order or shipment performs one id lookup with the caller's tenant.

`DATABASE_URL` is required at process start and must not be logged. `TRANSPORT_ORDER_SERVICE_URL` and `SHIPMENT_SERVICE_URL` are used for that ownership lookup. Predictive sources are rejected.
