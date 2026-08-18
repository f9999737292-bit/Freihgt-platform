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
  }
}
