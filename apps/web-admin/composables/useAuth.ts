import type { LoginResponse } from '~/types/api'

export function useAuth() {
  const authStore = useAuthStore()
  const tenantStore = useTenantStore()
  const { t } = useI18n()
  const { apiPost } = useApi()

  async function login(tenantId: string, email: string, password: string) {
    if (!tenantId.trim()) {
      throw new Error(t('tenant.required'))
    }

    const config = useRuntimeConfig()
    const payload = {
      tenant_id: tenantId.trim(),
      email: email.trim(),
      password,
    }

    tenantStore.setTenant(payload.tenant_id)

    let result: LoginResponse
    if (config.public.mockAuth) {
      result = await authStore.login(payload)
    } else {
      result = await apiPost<LoginResponse>('/api/v1/auth/login', payload, {
        skipAuth: true,
        skipTenant: true,
      })
      authStore.setSession(result.access_token, result.user)
      if (result.user.tenant_id) {
        tenantStore.setTenant(result.user.tenant_id)
      }
    }

    if (result.user.preferred_locale) {
      const nuxtApp = useNuxtApp()
      const i18n = nuxtApp.$i18n as { setLocale?: (locale: string) => Promise<void> } | undefined
      await i18n?.setLocale?.(result.user.preferred_locale)
    }

    return result
  }

  async function logout() {
    authStore.logout()
    await navigateTo('/login')
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
