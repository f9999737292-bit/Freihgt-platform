import { defineStore } from 'pinia'

export interface ToastMessage {
  id: string
  type: 'success' | 'error' | 'info'
  message: string
}

export const useUiStore = defineStore('ui', {
  state: () => ({
    toasts: [] as ToastMessage[],
  }),

  actions: {
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
  },
})
