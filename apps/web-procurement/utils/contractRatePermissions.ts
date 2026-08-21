import type { AuthUser } from '~/types/api'

const CONTRACT_READ_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
  'CARRIER_ACCOUNTANT',
] as const

const CONTRACT_MUTATE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'FORWARDER_MANAGER',
] as const

const CARRIER_READ_ROLES = [
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
  'CARRIER_ACCOUNTANT',
] as const

function rolesFromUser(user: AuthUser | null): string[] {
  return user?.roles ?? []
}

export function canReadContractsForRoles(roles: string[]): boolean {
  return roles.includes('PLATFORM_ADMIN') || CONTRACT_READ_ROLES.some((role) => roles.includes(role))
}

export function canCreateContractsForRoles(roles: string[]): boolean {
  return roles.includes('PLATFORM_ADMIN') || CONTRACT_MUTATE_ROLES.some((role) => roles.includes(role))
}

export function canEditContractMetadataForRoles(roles: string[]): boolean {
  return canCreateContractsForRoles(roles)
}

export function isCarrierContractReaderForRoles(roles: string[]): boolean {
  if (roles.includes('PLATFORM_ADMIN')) return false
  return CARRIER_READ_ROLES.some((role) => roles.includes(role))
    && !CONTRACT_MUTATE_ROLES.some((role) => roles.includes(role))
}

export function shouldShowContractsNav(
  featureEnabled: boolean,
  roles: string[],
): boolean {
  return featureEnabled && canReadContractsForRoles(roles)
}
