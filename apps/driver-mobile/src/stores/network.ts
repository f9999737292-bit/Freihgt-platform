import { defineStore } from 'pinia'

export const useNetworkStore = defineStore('network', {
  state: () => ({
    online: typeof navigator !== 'undefined' ? navigator.onLine : true,
  }),

  actions: {
    setOnline(value: boolean) {
      this.online = value
    },

    async refreshFromPlatform() {
      if (typeof navigator !== 'undefined') {
        this.online = navigator.onLine
      }

      try {
        const { Network } = await import('@capacitor/network')
        const status = await Network.getStatus()
        this.online = status.connected
      } catch {
        // Capacitor network plugin unavailable in web-only runtime.
      }
    },
  },
})
