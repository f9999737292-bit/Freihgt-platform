export function useAuth() {
  const authStore = useAuthStore()
  const tenantStore = useTenantStore()
  const router = useRouter()
  const { pushToast } = useToast()
  const { t } = useI18n()

  async function login(tenantId: string, email: string, password: string) {
    if (!tenantId.trim()) {
      pushToast('error', t('tenant.required'))
      throw new Error(t('tenant.required'))
    }

    const result = await authStore.login({
      tenant_id: tenantId.trim(),
      email: email.trim(),
      password,
    })

    tenantStore.setTenant(tenantId.trim())

    const { setLocale } = useI18n()
    if (result.user.preferred_locale) {
      await setLocale(result.user.preferred_locale as 'ru-RU' | 'en-US' | 'zh-CN')
    }
    pushToast('success', 'Login successful')
    await router.push('/dashboard')
  }

  async function logout() {
    authStore.logout()
    await router.push('/login')
  }

  function clearSession() {
    authStore.clearSession()
  }

  return {
    user: computed(() => authStore.user),
    isAuthenticated: computed(() => authStore.isAuthenticated),
    login,
    logout,
    clearSession,
    restoreSession: authStore.restoreSession,
  }
}
