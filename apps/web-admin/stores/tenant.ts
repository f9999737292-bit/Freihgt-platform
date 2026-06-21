import { defineStore } from 'pinia'

export const TENANT_STORAGE_KEY = 'freight_admin_tenant_id'

export const useTenantStore = defineStore('tenant', {
  state: () => ({
    tenantId: '' as string,
    currentCompanyId: null as string | null,
    restored: false,
  }),

  getters: {
    hasTenant: (state) => Boolean(state.tenantId?.trim()),
  },

  actions: {
    setTenant(tenantId: string) {
      this.tenantId = tenantId.trim()
      if (import.meta.client) {
        localStorage.setItem(TENANT_STORAGE_KEY, this.tenantId)
      }
    },

    setCompany(companyId: string | null) {
      this.currentCompanyId = companyId
    },

    restoreTenant() {
      if (!import.meta.client) {
        this.restored = true
        return
      }

      const saved = localStorage.getItem(TENANT_STORAGE_KEY)
      if (saved?.trim()) {
        this.tenantId = saved.trim()
      }
      this.restored = true
    },

    clearTenant() {
      this.tenantId = ''
      this.currentCompanyId = null
      if (import.meta.client) {
        localStorage.removeItem(TENANT_STORAGE_KEY)
      }
    },
  },
})
