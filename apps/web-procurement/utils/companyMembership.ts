import type { UserCompanyMembership } from '~/types/company'

export const BUYER_MEMBERSHIP_ROLE_CODES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
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
