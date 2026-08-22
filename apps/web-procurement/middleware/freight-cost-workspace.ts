export default defineNuxtRouteMiddleware((to) => {
  const { enabled } = useFreightCostFeature()
  if (!enabled.value && to.path !== '/freight-costs/unavailable') {
    return navigateTo('/freight-costs/unavailable')
  }
})
