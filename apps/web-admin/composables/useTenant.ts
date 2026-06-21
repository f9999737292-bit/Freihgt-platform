import { TENANT_STORAGE_KEY } from '~/stores/tenant'

export function useTenantContext() {
  const tenantStore = useTenantStore()
  const config = useRuntimeConfig()
  const { t } = useI18n()
  const router = useRouter()
  const { pushToast } = useToast()

  const tenantId = computed(() => tenantStore.tenantId)
  const hasTenant = computed(() => tenantStore.hasTenant)

  function resolveInitialTenantId(): string {
    if (import.meta.client) {
      const saved = localStorage.getItem(TENANT_STORAGE_KEY)
      if (saved?.trim()) {
        return saved.trim()
      }
    }
    return config.public.defaultTenantId || ''
  }

  function formatTenantId(id?: string | null): string {
    const value = id?.trim()
    if (!value) return '—'
    if (value.length <= 13) return value
    return `${value.slice(0, 8)}-...`
  }

  function setTenant(tenantId: string) {
    tenantStore.setTenant(tenantId)
  }

  function clearTenant() {
    tenantStore.clearTenant()
    pushToast('info', t('tenant.clear'))
  }

  async function applyTenant(tenantId: string) {
    const trimmed = tenantId.trim()
    if (!trimmed) {
      pushToast('error', t('tenant.required'))
      return false
    }
    tenantStore.setTenant(trimmed)
    pushToast('success', t('tenant.save'))
    await router.go(0)
    return true
  }

  return {
    tenantId,
    hasTenant,
    currentCompanyId: computed(() => tenantStore.currentCompanyId),
    setTenant,
    setCompany: tenantStore.setCompany,
    clearTenant,
    resolveInitialTenantId,
    formatTenantId,
    applyTenant,
    restoreTenant: tenantStore.restoreTenant,
  }
}
