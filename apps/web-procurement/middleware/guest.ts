export default defineNuxtRouteMiddleware(() => {
  const { restoreSession, isAuthenticated, restored } = useSession()

  if (!restored.value) {
    restoreSession()
  }

  if (isAuthenticated.value) {
    return navigateTo('/')
  }
})
