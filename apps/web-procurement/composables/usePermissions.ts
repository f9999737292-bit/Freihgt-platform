import type { AuthUser } from '~/types/api'

const TENDER_MANAGE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
] as const

const TENDER_READ_ROLES = [...TENDER_MANAGE_ROLES] as const

const CARRIER_TENDER_ROLES = [
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
] as const

const PAYMENT_READ_ROLES = [
  'PLATFORM_ADMIN',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FINANCE_MANAGER',
  'FORWARDER_MANAGER',
  'CARRIER_ADMIN',
  'CARRIER_ACCOUNTANT',
] as const

const PAYMENT_WRITE_ROLES = [
  'PLATFORM_ADMIN',
  'SHIPPER_ADMIN',
  'FINANCE_MANAGER',
  'CARRIER_ADMIN',
  'CARRIER_ACCOUNTANT',
] as const

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

const CONTRACT_LIFECYCLE_ROLES = [
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

function currentUser(): AuthUser | null {
  return useAuthStore().user
}

function userRoles(): string[] {
  return currentUser()?.roles ?? []
}

function isDevPlatformAdminFallback(): boolean {
  const config = useRuntimeConfig()
  const user = currentUser()
  if (!user?.email) return false
  return config.public.mockAuth === true && user.email.toLowerCase() === 'admin@7rights.local'
}

export function usePermissions() {
  function hasAnyRole(roles: readonly string[]): boolean {
    if (isDevPlatformAdminFallback()) return true
    return roles.some((role) => userRoles().includes(role))
  }

  function isPlatformAdmin(): boolean {
    return userRoles().includes('PLATFORM_ADMIN') || isDevPlatformAdminFallback()
  }

  function canReadTenders(): boolean {
    return isPlatformAdmin() || hasAnyRole(TENDER_READ_ROLES)
  }

  function canManageTenders(): boolean {
    return isPlatformAdmin() || hasAnyRole(TENDER_MANAGE_ROLES)
  }

  function canPublishTenders(): boolean {
    return canManageTenders()
  }

  function isBuyerRole(): boolean {
    return canManageTenders()
  }

  function isProcurementRole(): boolean {
    if (isPlatformAdmin()) return true
    return userRoles().includes('PROCUREMENT_MANAGER')
  }

  function canReadCarrierTenders(): boolean {
    return isPlatformAdmin() || hasAnyRole(CARRIER_TENDER_ROLES)
  }

  function isCarrierRole(): boolean {
    return canReadCarrierTenders()
  }

  function canReadPayments(): boolean {
    return isPlatformAdmin() || hasAnyRole(PAYMENT_READ_ROLES)
  }

  function canWritePayments(): boolean {
    return isPlatformAdmin() || hasAnyRole(PAYMENT_WRITE_ROLES)
  }

  function canReadContracts(): boolean {
    return isPlatformAdmin() || hasAnyRole(CONTRACT_READ_ROLES)
  }

  function canCreateContracts(): boolean {
    return isPlatformAdmin() || hasAnyRole(CONTRACT_MUTATE_ROLES)
  }

  function canEditDraftContracts(): boolean {
    return canCreateContracts()
  }

  function canEditContractMetadata(): boolean {
    return canCreateContracts()
  }

  function canActivateContracts(): boolean {
    return isPlatformAdmin() || hasAnyRole(CONTRACT_LIFECYCLE_ROLES)
  }

  function canSuspendContracts(): boolean {
    return canActivateContracts()
  }

  function canTerminateContracts(): boolean {
    return canActivateContracts()
  }

  function canReadRates(): boolean {
    return canReadContracts()
  }

  function canEditDraftRates(): boolean {
    return canCreateContracts()
  }

  function canActivateRateVersions(): boolean {
    return canActivateContracts()
  }

  function canSimulateRates(): boolean {
    return canReadRates()
  }

  function isCarrierContractReader(): boolean {
    if (isPlatformAdmin()) return false
    return hasAnyRole(CARRIER_READ_ROLES) && !hasAnyRole(CONTRACT_MUTATE_ROLES)
  }

  return {
    hasAnyRole,
    isPlatformAdmin,
    canReadTenders,
    canManageTenders,
    canPublishTenders,
    isBuyerRole,
    isProcurementRole,
    canReadCarrierTenders,
    isCarrierRole,
    canReadPayments,
    canWritePayments,
    canReadContracts,
    canCreateContracts,
    canEditDraftContracts,
    canEditContractMetadata,
    canActivateContracts,
    canSuspendContracts,
    canTerminateContracts,
    canReadRates,
    canEditDraftRates,
    canActivateRateVersions,
    canSimulateRates,
    isCarrierContractReader,
  }
}
