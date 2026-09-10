/** Buyer-only RFx template and version management permissions. */

const BUYER_MANAGE_ROLES = [
  'PLATFORM_ADMIN',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
  'PROCUREMENT_MANAGER',
] as const

const CARRIER_ROLES = ['CARRIER_ADMIN', 'CARRIER_DISPATCHER'] as const

export function useRfxBuyerPermissions() {
  const { hasAnyRole, isPlatformAdmin } = usePermissions()

  function isDevAdminFallback(): boolean {
    const config = useRuntimeConfig()
    const authStore = useAuthStore()
    return config.public.mockAuth === true && authStore.user?.email?.toLowerCase() === 'admin@bintrans.local'
  }

  function canManageRfxTemplates(): boolean {
    if (isDevAdminFallback() || isPlatformAdmin()) return true
    return hasAnyRole([...BUYER_MANAGE_ROLES])
  }

  function canReadRfxTemplates(): boolean {
    return canManageRfxTemplates()
  }

  function isCarrierWithoutBuyerAccess(): boolean {
    if (isDevAdminFallback() || isPlatformAdmin()) return false
    const { hasRole } = usePermissions()
    const isCarrier = CARRIER_ROLES.some((role) => hasRole(role))
    return isCarrier && !canManageRfxTemplates()
  }

  function canManageEventVersions(): boolean {
    return canManageRfxTemplates()
  }

  return {
    canManageRfxTemplates,
    canReadRfxTemplates,
    isCarrierWithoutBuyerAccess,
    canManageEventVersions,
  }
}
