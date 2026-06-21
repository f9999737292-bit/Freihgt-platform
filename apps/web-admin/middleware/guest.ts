export default defineNuxtRouteMiddleware(() => {
  const authStore = useAuthStore()

  if (!authStore.restored) {
    authStore.restoreSession()
  }

  if (authStore.isAuthenticated) {
    return navigateTo('/dashboard')
  }
})
