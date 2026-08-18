import type { UserCompanyMembership } from '~/types/company'

export const BUYER_MEMBERSHIP_ROLE_CODES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
] as const

export const CARRIER_MEMBERSHIP_ROLE_CODES = [
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
] as const

export function isBuyerMembership(membership: Pick<UserCompanyMembership, 'membership_status' | 'roles'>): boolean {
  if (membership.membership_status !== 'ACTIVE') {
    return false
  }
  return membership.roles.some((role) =>
    BUYER_MEMBERSHIP_ROLE_CODES.includes(role.code as (typeof BUYER_MEMBERSHIP_ROLE_CODES)[number]),
  )
}

export function filterBuyerMemberships<T extends Pick<UserCompanyMembership, 'membership_status' | 'roles'>>(
  memberships: T[],
): T[] {
  return memberships.filter(isBuyerMembership)
}

export function isCarrierMembership(membership: Pick<UserCompanyMembership, 'membership_status' | 'roles' | 'company_type'>): boolean {
  if (membership.membership_status !== 'ACTIVE') {
    return false
  }
  if (membership.company_type === 'CARRIER') {
    return true
  }
  return membership.roles.some((role) =>
    CARRIER_MEMBERSHIP_ROLE_CODES.includes(role.code as (typeof CARRIER_MEMBERSHIP_ROLE_CODES)[number]),
  )
}

export function filterCarrierMemberships<T extends Pick<UserCompanyMembership, 'membership_status' | 'roles' | 'company_type'>>(
  memberships: T[],
): T[] {
  return memberships.filter(isCarrierMembership)
}

export function selectDefaultCarrierCompany(
  memberships: Array<Pick<UserCompanyMembership, 'company_id'>>,
): string {
  return memberships[0]?.company_id ?? ''
}

export function selectDefaultOwnerCompany(
  memberships: Array<Pick<UserCompanyMembership, 'company_id'>>,
): string {
  return memberships[0]?.company_id ?? ''
}

export function membershipSelectOptions(
  memberships: Array<Pick<UserCompanyMembership, 'company_id' | 'legal_name' | 'company_type'>>,
): Array<{ label: string; value: string }> {
  return memberships.map((membership) => ({
    label: `${membership.legal_name} (${membership.company_type})`,
    value: membership.company_id,
  }))
}
