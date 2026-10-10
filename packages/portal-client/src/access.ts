import {
  CARRIER_OFFICE_ROLES,
  SHIPPER_OFFICE_ROLES,
  type CarrierCompanyMembership,
  type CarrierTabSession,
  type ServerUserSnapshot,
} from '@freight-platform/shared-ts/types'

import { PortalClientError } from './errors'
import { assertCompanyInMembership } from './headers'

const OFFICE_ROLES = new Set<string>(CARRIER_OFFICE_ROLES)
const SHIPPER_ROLES = new Set<string>(SHIPPER_OFFICE_ROLES)

export function isCarrierOfficeRole(role: string): boolean {
  return OFFICE_ROLES.has(role.trim().toUpperCase())
}

export function canEnterCarrierPortal(
  userRoles: readonly string[],
  memberships: readonly CarrierCompanyMembership[] = [],
): boolean {
  const codes = [
    ...userRoles,
    ...memberships.flatMap((membership) => membership.roleCodes),
  ]
  return codes.some((role) => isCarrierOfficeRole(role))
}

export function carrierCompanies(
  memberships: readonly CarrierCompanyMembership[],
): CarrierCompanyMembership[] {
  return memberships.filter(
    (membership) =>
      membership.membershipStatus.trim().toUpperCase() === 'ACTIVE'
      && membership.roleCodes.some((role) => isCarrierOfficeRole(role)),
  )
}

export function reconcileSelectedCompany(
  selectedCompanyId: string | null,
  memberships: readonly CarrierCompanyMembership[],
): string | null {
  if (!selectedCompanyId) return null
  const allowed = carrierCompanies(memberships).map((membership) => membership.companyId)
  return allowed.includes(selectedCompanyId) ? selectedCompanyId : null
}

export function applyFreshMemberships(
  session: CarrierTabSession,
  freshMemberships: readonly CarrierCompanyMembership[],
): CarrierTabSession {
  const memberships = carrierCompanies(freshMemberships)
  return {
    accessToken: session.accessToken,
    user: session.user,
    tenantId: session.user.tenantId,
    selectedCompanyId: reconcileSelectedCompany(session.selectedCompanyId, memberships),
    memberships,
  }
}

export function selectCarrierCompany(
  memberships: readonly CarrierCompanyMembership[],
  companyId: string,
): string {
  const allowed = carrierCompanies(memberships).map((membership) => membership.companyId)
  assertCompanyInMembership(companyId, allowed)
  return companyId
}

interface RawMembershipRole {
  code?: string
}

interface RawMembership {
  membership_id?: string
  company_id?: string
  legal_name?: string
  company_type?: string
  membership_status?: string
  roles?: RawMembershipRole[]
}

export function mapServerMemberships(items: readonly RawMembership[]): CarrierCompanyMembership[] {
  return items
    .filter((item): item is RawMembership & { company_id: string } => typeof item.company_id === 'string' && item.company_id !== '')
    .map((item) => ({
      membershipId: item.membership_id ?? '',
      companyId: item.company_id,
      legalName: item.legal_name ?? item.company_id,
      companyType: item.company_type?.trim().toUpperCase() ?? '',
      membershipStatus: item.membership_status ?? '',
      roleCodes: (item.roles ?? [])
        .map((role) => role.code?.trim().toUpperCase() ?? '')
        .filter((code) => code !== ''),
    }))
}

export function assertSessionIdentity(
  user: ServerUserSnapshot,
  attempted?: { tenantId?: string; userId?: string },
): void {
  if (attempted?.tenantId && attempted.tenantId !== user.tenantId) {
    throw new PortalClientError('cross-tenant override rejected', 0, 'TENANT_MISMATCH')
  }
  if (attempted?.userId && attempted.userId !== user.id) {
    throw new PortalClientError('user override rejected', 0, 'USER_MISMATCH')
  }
}

export function isShipperOfficeRole(role: string): boolean {
  return SHIPPER_ROLES.has(role.trim().toUpperCase())
}

export function shipperCompanies(
  memberships: readonly CarrierCompanyMembership[],
): CarrierCompanyMembership[] {
  return memberships.filter(
    (membership) =>
      membership.membershipStatus.trim().toUpperCase() === 'ACTIVE'
      && (membership.companyType ?? '').trim().toUpperCase() === 'SHIPPER'
      && membership.roleCodes.some((role) => isShipperOfficeRole(role)),
  )
}

/** Entry requires an eligible shipper membership. A JWT role alone is not enough. */
export function canEnterShipperPortal(
  _userRoles: readonly string[],
  memberships: readonly CarrierCompanyMembership[] = [],
): boolean {
  return shipperCompanies(memberships).length > 0
}

export function reconcileSelectedShipperCompany(
  selectedCompanyId: string | null,
  memberships: readonly CarrierCompanyMembership[],
): string | null {
  if (!selectedCompanyId) return null
  const allowed = shipperCompanies(memberships).map((membership) => membership.companyId)
  return allowed.includes(selectedCompanyId) ? selectedCompanyId : null
}

export function applyFreshShipperMemberships(
  session: CarrierTabSession,
  freshMemberships: readonly CarrierCompanyMembership[],
): CarrierTabSession {
  const memberships = shipperCompanies(freshMemberships)
  return {
    accessToken: session.accessToken,
    user: session.user,
    tenantId: session.user.tenantId,
    selectedCompanyId: reconcileSelectedShipperCompany(session.selectedCompanyId, memberships),
    memberships,
  }
}

export function selectShipperCompany(
  memberships: readonly CarrierCompanyMembership[],
  companyId: string,
): string {
  const allowed = shipperCompanies(memberships).map((membership) => membership.companyId)
  assertCompanyInMembership(companyId, allowed)
  return companyId
}
