export default defineNuxtRouteMiddleware(() => {
  const { enabled } = useRfxVersioningFeature()
  const { canManageRfxTemplates, isCarrierWithoutBuyerAccess } = useRfxBuyerPermissions()

  if (!enabled.value) {
    return navigateTo('/rfx')
  }

  if (isCarrierWithoutBuyerAccess() || !canManageRfxTemplates()) {
    return navigateTo('/rfx')
  }
})
