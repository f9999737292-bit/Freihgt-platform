export default defineNuxtRouteMiddleware(() => {
  const { enabled } = useContractRateFeature()
  if (!enabled.value) {
    return navigateTo('/contracts/unavailable')
  }
})
