export default defineNuxtRouteMiddleware((to) => {
  const { restoreSession, isAuthenticated, restored } = useSession()

  if (!restored.value) {
    restoreSession()
  }

  if (!isAuthenticated.value) {
    return navigateTo({
      path: '/login',
      query: { redirect: to.fullPath },
    })
  }
})
