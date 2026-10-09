export const CARRIER_OFFICE_ROLES = ["CARRIER_ADMIN", "CARRIER_DISPATCHER"] as const;

export type CarrierOfficeRole = (typeof CARRIER_OFFICE_ROLES)[number];

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
}

/** Tab session. Persisted only in sessionStorage by the portal client. */
export interface CarrierTabSession {
  accessToken: string;
  user: ServerUserSnapshot;
  tenantId: string;
  selectedCompanyId: string | null;
  memberships: CarrierCompanyMembership[];
}
