export default defineNuxtRouteMiddleware(() => {
  const authStore = useAuthStore()

  if (!authStore.restored) {
    authStore.restoreSession()
    useTenantStore().restoreTenant()
  }

  if (authStore.isAuthenticated) {
    return navigateTo('/tenders')
  }
})
