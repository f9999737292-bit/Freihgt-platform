import { defineStore } from 'pinia'

export const TENANT_STORAGE_KEY = 'freight_procurement_tenant_id'
export const COMPANY_STORAGE_KEY = 'freight_procurement_company_id'

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
      if (import.meta.client) {
        if (companyId) {
          localStorage.setItem(COMPANY_STORAGE_KEY, companyId)
        } else {
          localStorage.removeItem(COMPANY_STORAGE_KEY)
        }
      }
    },

    restoreTenant() {
      if (!import.meta.client) {
        this.restored = true
        return
      }

      const savedTenant = localStorage.getItem(TENANT_STORAGE_KEY)
      if (savedTenant?.trim()) {
        this.tenantId = savedTenant.trim()
      }
      const savedCompany = localStorage.getItem(COMPANY_STORAGE_KEY)
      if (savedCompany?.trim()) {
        this.currentCompanyId = savedCompany.trim()
      }
      this.restored = true
    },

    clearTenant() {
      this.tenantId = ''
      this.currentCompanyId = null
      if (import.meta.client) {
        localStorage.removeItem(TENANT_STORAGE_KEY)
        localStorage.removeItem(COMPANY_STORAGE_KEY)
      }
    },
  },
})
