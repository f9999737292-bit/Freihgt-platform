import type { AuthUser } from '~/types/api'

const FREIGHT_COST_READ_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
  'FINANCE_MANAGER',
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
  'CARRIER_ACCOUNTANT',
] as const

const FREIGHT_COST_BUYER_ANALYTICS_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
  'FINANCE_MANAGER',
] as const

const CARRIER_READ_ROLES = [
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
  'CARRIER_ACCOUNTANT',
] as const

const BUYER_MUTATE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'FORWARDER_MANAGER',
] as const

function rolesFromUser(user: AuthUser | null): string[] {
  return user?.roles ?? []
}

export function canReadFreightCostsForRoles(roles: string[]): boolean {
  return roles.includes('PLATFORM_ADMIN') || FREIGHT_COST_READ_ROLES.some((role) => roles.includes(role))
}

export function canSeeBuyerInternalFreightCostFieldsForRoles(roles: string[]): boolean {
  return roles.includes('PLATFORM_ADMIN')
    || FREIGHT_COST_BUYER_ANALYTICS_ROLES.some((role) => roles.includes(role))
}

export function canSeeVarianceAnalysisNavForRoles(roles: string[]): boolean {
  return canSeeBuyerInternalFreightCostFieldsForRoles(roles)
}

export function canSeeAccessorialAnalyticsNavForRoles(roles: string[]): boolean {
  return canSeeBuyerInternalFreightCostFieldsForRoles(roles)
}

export function isCarrierFreightCostReaderForRoles(roles: string[]): boolean {
  if (roles.includes('PLATFORM_ADMIN')) return false
  return CARRIER_READ_ROLES.some((role) => roles.includes(role))
    && !BUYER_MUTATE_ROLES.some((role) => roles.includes(role))
    && !roles.includes('FINANCE_MANAGER')
    && !roles.includes('SHIPPER_LOGIST')
}

export function shouldShowFreightCostsNav(
  featureEnabled: boolean,
  roles: string[],
): boolean {
  return featureEnabled && canReadFreightCostsForRoles(roles)
}

export function resolveFreightCostActorFromRoles(roles: string[]): 'BUYER' | 'CARRIER' {
  if (isCarrierFreightCostReaderForRoles(roles)) return 'CARRIER'
  return 'BUYER'
}

export function rolesFromAuthUser(user: AuthUser | null): string[] {
  return rolesFromUser(user)
}
