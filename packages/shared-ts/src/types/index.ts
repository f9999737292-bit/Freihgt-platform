export type AppHealthResponse = {
  status: "ok" | "degraded" | "down";
  timestamp: string;
  version?: string;
};

export type {
  CarrierCompanyMembership,
  CarrierOfficeRole,
  CarrierTabSession,
  PersistedCarrierTabSession,
  ServerUserSnapshot,
  ShipperOfficeRole,
} from "./portal";

export { CARRIER_OFFICE_ROLES, SHIPPER_OFFICE_ROLES } from "./portal";
