export function useAuth() {
  const router = useRouter()
  const session = useSession()

  async function login(tenantId: string, email: string, password: string) {
    const result = await session.login({
      tenant_id: tenantId.trim(),
      email: email.trim(),
      password,
    })

    const { setLocale } = useI18n()
    if (result.user.preferred_locale) {
      await setLocale(result.user.preferred_locale as 'ru-RU' | 'en-US' | 'zh-CN')
    }

    return result
  }

  async function logout() {
    session.clearSession()
    await router.push('/login')
  }

  return {
    user: computed(() => session.user.value),
    isAuthenticated: session.isAuthenticated,
    login,
    logout,
    restoreSession: session.restoreSession,
  }
}
