import type { AuthUser, LoginRequest, LoginResponse } from '~/types/api'

const STORAGE_KEY = 'freight_procurement_session'

interface SessionData {
  token: string
  user: AuthUser
}

export function useSession() {
  const token = useState<string | null>('procurement-token', () => null)
  const user = useState<AuthUser | null>('procurement-user', () => null)
  const restored = useState<boolean>('procurement-session-restored', () => false)

  const isAuthenticated = computed(() => Boolean(token.value && user.value))

  function persistSession(accessToken: string, authUser: AuthUser) {
    token.value = accessToken
    user.value = authUser
    if (import.meta.client) {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({ token: accessToken, user: authUser } satisfies SessionData),
      )
    }
  }

  function clearSession() {
    token.value = null
    user.value = null
    if (import.meta.client) {
      localStorage.removeItem(STORAGE_KEY)
    }
  }

  function restoreSession() {
    if (!import.meta.client || restored.value) {
      restored.value = true
      return
    }

    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (raw) {
        const data = JSON.parse(raw) as SessionData
        token.value = data.token
        user.value = data.user
      }
    } catch {
      localStorage.removeItem(STORAGE_KEY)
    } finally {
      restored.value = true
    }
  }

  async function login(payload: LoginRequest) {
    const config = useRuntimeConfig()

    if (config.public.mockAuth) {
      const mockUser: AuthUser = {
        id: '8541a3a3-bde7-4fed-9501-37b9953bf904',
        tenant_id: payload.tenant_id.trim(),
        email: payload.email.trim(),
        full_name: 'Demo Procurement',
        preferred_locale: 'ru-RU',
        status: 'ACTIVE',
        roles: ['PROCUREMENT_MANAGER'],
      }
      persistSession(`mock-token-${Date.now()}`, mockUser)
      return {
        access_token: token.value!,
        token_type: 'Bearer',
        expires_in: 3600,
        user: mockUser,
      } satisfies LoginResponse
    }

    const { apiPost } = useApi()
    const response = await apiPost<LoginResponse>('/api/v1/auth/login', payload, { skipAuth: true })
    persistSession(response.access_token, response.user)
    return response
  }

  return {
    token,
    user,
    restored,
    isAuthenticated,
    login,
    clearSession,
    restoreSession,
  }
}
