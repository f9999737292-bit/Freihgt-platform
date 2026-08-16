import { defineStore } from 'pinia'
import { HttpClient, clearPersistedSession, loadPersistedSession, persistSession } from '@/api/client'
import { createDriverApi } from '@/api/driverApi'
import { getPilotTenantId } from '@/config/env'
import { useNetworkStore } from '@/stores/network'
import type { AuthUser, LoginRequest } from '@/types/auth'

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
    createApi(isOnline: () => boolean) {
      const http = new HttpClient({
        getToken: () => this.token,
        isOnline,
      })
      return createDriverApi(http)
    },

    async restoreSession() {
      try {
        const raw = await loadPersistedSession()
        if (raw) {
          const data = JSON.parse(raw) as SessionData
          this.token = data.token
          this.user = data.user
        }
      } catch {
        await clearPersistedSession()
      } finally {
        this.restored = true
      }
    },

    async login(credentials: LoginRequest) {
      const tenantId = getPilotTenantId()
      if (!tenantId) {
        throw new Error('Pilot tenant is not configured (VITE_PILOT_TENANT_ID)')
      }

      const networkStore = useNetworkStore()
      const api = this.createApi(() => networkStore.online)
      const result = await api.login({
        tenant_id: tenantId,
        email: credentials.email.trim(),
        password: credentials.password,
      })

      if (result.outcome !== 'SUCCESS' || !result.data) {
        if (result.error) {
          throw result.error
        }
        if (result.outcome === 'REQUEST_NOT_SENT') {
          throw new Error('offline')
        }
        throw new Error('unknown')
      }

      this.setSession(result.data.access_token, result.data.user)
      return result.data
    },

    async setSession(token: string, user: AuthUser) {
      this.token = token
      this.user = user
      await persistSession(JSON.stringify({ token, user } satisfies SessionData))
    },

    async logout() {
      this.token = null
      this.user = null
      await clearPersistedSession()
    },
  },
})
