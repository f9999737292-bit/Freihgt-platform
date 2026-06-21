import { defineStore } from 'pinia'
import type { ApiGatewayStatus } from '~/types/api'

export interface ToastMessage {
  id: string
  type: 'success' | 'error' | 'info'
  message: string
}

export const useUiStore = defineStore('ui', {
  state: () => ({
    sidebarCollapsed: false,
    toasts: [] as ToastMessage[],
    apiGatewayStatus: 'checking' as ApiGatewayStatus,
    lastSmokeTestStatus: 'unknown' as 'passed' | 'failed' | 'unknown',
  }),

  actions: {
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    },

    pushToast(type: ToastMessage['type'], message: string) {
      const id = `${Date.now()}-${Math.random()}`
      this.toasts.push({ id, type, message })
      if (import.meta.client) {
        setTimeout(() => this.removeToast(id), 5000)
      }
    },

    removeToast(id: string) {
      this.toasts = this.toasts.filter((t) => t.id !== id)
    },

    setApiGatewayStatus(status: ApiGatewayStatus) {
      this.apiGatewayStatus = status
    },

    setLastSmokeTestStatus(status: 'passed' | 'failed' | 'unknown') {
      this.lastSmokeTestStatus = status
    },
  },
})
