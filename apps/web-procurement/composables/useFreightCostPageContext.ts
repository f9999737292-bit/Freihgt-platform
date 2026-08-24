import { ApiError } from '~/utils/apiClient'
import { isApiUnavailableError } from '~/utils/apiError'
import { resolveFreightCostActorFromRoles } from '~/utils/freightCostPermissions'
import { isFreightCostLiveUnavailableError } from '~/utils/freightCostDataSource'

export function useFreightCostPageContext() {
  const { getFreightCostSummary } = useFreightCostsApi()
  const { currentCompanyId } = useTenantContext()
  const { user } = useAuth()
  const roles = computed(() => user.value?.roles ?? [])

  const actor = computed(() => resolveFreightCostActorFromRoles(roles.value))
  const loading = ref(false)
  const forbidden = ref(false)
  const apiUnavailable = ref(false)
  const liveUnavailable = ref(false)
  const missingCompany = computed(() => !currentCompanyId.value)

  async function runLoad<T>(loader: () => Promise<T>): Promise<T | null> {
    loading.value = true
    forbidden.value = false
    apiUnavailable.value = false
    liveUnavailable.value = false
    try {
      if (!currentCompanyId.value) return null
      return await loader()
    } catch (error) {
      if (error instanceof ApiError && error.status === 403) {
        forbidden.value = true
      } else if (isFreightCostLiveUnavailableError(error)) {
        liveUnavailable.value = true
      } else if (isApiUnavailableError(error)) {
        apiUnavailable.value = true
      }
      return null
    } finally {
      loading.value = false
    }
  }

  async function probeSummary() {
    if (!currentCompanyId.value) {
      return
    }
    try {
      await getFreightCostSummary({ company_id: currentCompanyId.value! })
    } catch (error) {
      if (error instanceof ApiError && error.status === 403) {
        forbidden.value = true
      } else if (isFreightCostLiveUnavailableError(error)) {
        liveUnavailable.value = true
      } else if (isApiUnavailableError(error)) {
        apiUnavailable.value = true
      }
    }
  }

  onMounted(() => {
    void probeSummary()
  })

  return {
    actor,
    roles,
    loading,
    forbidden,
    apiUnavailable,
    liveUnavailable,
    missingCompany,
    currentCompanyId,
    runLoad,
  }
}
