export const CARRIER_OFFICE_ROLES = ["CARRIER_ADMIN", "CARRIER_DISPATCHER"] as const;

export type CarrierOfficeRole = (typeof CARRIER_OFFICE_ROLES)[number];

export const SHIPPER_OFFICE_ROLES = ["SHIPPER_ADMIN", "SHIPPER_LOGIST"] as const;

export type ShipperOfficeRole = (typeof SHIPPER_OFFICE_ROLES)[number];

/** Server login user. Tenant id is display and login-contract metadata, never an authority header. */
export interface ServerUserSnapshot {
  id: string;
  tenantId: string;
  email: string;
  fullName: string;
  roles: string[];
}

export interface CarrierCompanyMembership {
  membershipId: string;
  companyId: string;
  legalName: string;
  membershipStatus: string;
  roleCodes: string[];
  /** Server company type. Absent values are not treated as SHIPPER. */
  companyType?: string;
}

/** Fields allowed in sessionStorage. Memberships are not persisted. */
export interface PersistedCarrierTabSession {
  accessToken: string;
  user: ServerUserSnapshot;
  tenantId: string;
  selectedCompanyId: string | null;
}

/** In-memory office session. Memberships come from a fresh gateway read. */
export interface CarrierTabSession extends PersistedCarrierTabSession {
  memberships: CarrierCompanyMembership[];
}
