# Payload templates

Reference JSON bodies used by `smoke-test.sh`. The script builds requests dynamically; these files document the expected shape.

| File | Used in step |
|------|----------------|
| `company-shipper.json` | Create shipper company |
| `company-consignee.json` | Create consignee company |
| `company-carrier.json` | Create carrier company |
| `user.json` | Create test user |
| `membership.json` | Link user to shipper company |
| `location-origin.json` | Origin warehouse |
| `location-destination.json` | Destination warehouse |
| `cargo.json` | FMCG cargo |
| `transport-order.json` | Transport order |
| `freight-request.json` | Freight request from transport order |
| `bid.json` | Carrier bid |
| `driver.json` | Driver |
| `vehicle.json` | Truck |
| `shipment-from-bid.json` | Shipment |
| `document-pod.json` | POD document |
| `billing-register.json` | Billing register |
| `billing-item.json` | Register item |
| `upd.json` | UPD closing document |

Replace placeholder UUIDs with values exported by the smoke test script.
