export default defineNuxtPlugin(() => {
  const authStore = useAuthStore()
  const tenantStore = useTenantStore()
  authStore.restoreSession()
  tenantStore.restoreTenant()
})
