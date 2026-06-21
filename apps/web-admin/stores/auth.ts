import { defineStore } from 'pinia'
import type { AuthUser, LoginRequest, LoginResponse } from '~/types/api'

const STORAGE_KEY = 'freight_admin_session'

interface SessionData {
  token: string
  user: AuthUser
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: null as string | null,
    user: null as AuthUser | null,
    restored: false,
  }),

  getters: {
    isAuthenticated: (state) => Boolean(state.token && state.user),
  },

  actions: {
    async login(payload: LoginRequest) {
      const config = useRuntimeConfig()
      const tenantStore = useTenantStore()

      if (!payload.tenant_id?.trim()) {
        throw new Error('Tenant ID is required')
      }

      tenantStore.setTenant(payload.tenant_id)

      if (config.public.mockAuth) {
        const mockUser: AuthUser = {
          id: 'demo-admin-user',
          tenant_id: payload.tenant_id.trim(),
          email: payload.email.trim(),
          full_name: 'Demo Admin',
          preferred_locale: 'ru-RU',
          status: 'ACTIVE',
        }
        this.setSession('mock-token-' + Date.now(), mockUser)
        return { access_token: this.token!, user: mockUser } as LoginResponse
      }

      const { apiPost } = useApi()
      const response = await apiPost<LoginResponse>('/api/v1/auth/login', payload, {
        skipAuth: true,
        skipTenant: true,
      })
      this.setSession(response.access_token, response.user)
      if (response.user.tenant_id) {
        tenantStore.setTenant(response.user.tenant_id)
      }
      return response
    },

    setSession(token: string, user: AuthUser) {
      this.token = token
      this.user = user
      if (import.meta.client) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ token, user } satisfies SessionData))
      }
      if (user.tenant_id) {
        useTenantStore().setTenant(user.tenant_id)
      }
    },

    logout() {
      this.token = null
      this.user = null
      if (import.meta.client) {
        localStorage.removeItem(STORAGE_KEY)
      }
    },

    clearSession() {
      this.logout()
    },

    restoreSession() {
      if (!import.meta.client) {
        this.restored = true
        return
      }
      try {
        const raw = localStorage.getItem(STORAGE_KEY)
        if (raw) {
          const data = JSON.parse(raw) as SessionData
          this.token = data.token
          this.user = data.user
          if (data.user.tenant_id) {
            useTenantStore().setTenant(data.user.tenant_id)
          }
        }
      } catch {
        localStorage.removeItem(STORAGE_KEY)
      } finally {
        this.restored = true
      }
    },
  },
})
