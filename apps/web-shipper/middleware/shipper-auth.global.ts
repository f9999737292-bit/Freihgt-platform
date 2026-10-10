export default defineNuxtRouteMiddleware((to) => {
  const office = useShipperOffice()
  office.hydrate()
  const isLogin = to.path === '/login' || to.path.endsWith('/login')
  if (import.meta.server) return
  if (!office.session.value && !isLogin) return navigateTo('/login')
  if (office.session.value && isLogin) return navigateTo('/')
})
