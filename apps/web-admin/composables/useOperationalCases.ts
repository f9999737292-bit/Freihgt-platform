import type {
  ControlTowerCaseActionItem,
  ControlTowerCaseKpi,
  ControlTowerCasePreset,
  ControlTowerCaseResolutionCode,
  ControlTowerCasesResponse,
  ControlTowerOperationalCase,
  ControlTowerWorkItem,
} from '~/types/controlTower'
import { formatApiErrorForUser } from '~/composables/useApi'

export function useOperationalCases() {
  const { apiGet, apiPost, apiPatch, apiDelete } = useApi()
  const { pushToast } = useToast()
  const { t } = useI18n()

  const loading = useState('ct-cases-loading', () => false)
  const actionLoading = useState('ct-cases-action-loading', () => false)
  const cases = useState<ControlTowerOperationalCase[]>('ct-cases-list', () => [])
  const selectedCase = useState<ControlTowerOperationalCase | null>('ct-cases-selected', () => null)
  const drawerOpen = useState('ct-cases-drawer-open', () => false)
  const activePreset = useState<ControlTowerCasePreset>('ct-cases-preset', () => 'my_cases')
  const page = useState('ct-cases-page', () => 1)
  const limit = useState('ct-cases-limit', () => 50)
  const total = useState('ct-cases-total', () => 0)
  const hasNext = useState('ct-cases-has-next', () => false)
  const kpi = useState<ControlTowerCaseKpi | null>('ct-cases-kpi', () => null)

  function buildQuery(): Record<string, string | number | boolean> {
    return {
      preset: activePreset.value,
      page: page.value,
      limit: limit.value,
    }
  }

  async function loadCases(options?: { preset?: ControlTowerCasePreset; resetPage?: boolean }) {
    if (options?.preset) activePreset.value = options.preset
    if (options?.resetPage) page.value = 1
    loading.value = true
    try {
      const response = await apiGet<ControlTowerCasesResponse>('/api/v1/control-tower/cases', {
        query: buildQuery(),
      })
      cases.value = response.items ?? []
      page.value = response.page ?? 1
      limit.value = response.limit ?? 50
      total.value = response.total ?? 0
      hasNext.value = response.hasNext ?? false
    } catch (error) {
      cases.value = []
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      loading.value = false
    }
  }

  async function loadKpi() {
    try {
      kpi.value = await apiGet<ControlTowerCaseKpi>('/api/v1/control-tower/cases/kpi')
    } catch {
      kpi.value = null
    }
  }

  async function openCase(caseId: string) {
    drawerOpen.value = true
    try {
      selectedCase.value = await apiGet<ControlTowerOperationalCase>(`/api/v1/control-tower/cases/${caseId}`)
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
      drawerOpen.value = false
    }
  }

  async function createCase(payload: {
    title: string
    summary?: string
    severity?: string
    shipmentIds?: string[]
    workItems?: { itemType: string; itemId: string }[]
  }) {
    actionLoading.value = true
    try {
      const created = await apiPost<ControlTowerOperationalCase>('/api/v1/control-tower/cases', payload)
      pushToast(t('controlTower.cases.created'), 'success')
      await Promise.all([loadCases({ resetPage: true }), loadKpi()])
      return created
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
      return null
    } finally {
      actionLoading.value = false
    }
  }

  async function createCaseFromWorkItem(item: ControlTowerWorkItem) {
    return createCase({
      title: item.title,
      summary: item.summary,
      shipmentIds: item.shipmentId ? [item.shipmentId] : [],
      workItems: [{ itemType: item.itemType, itemId: item.sourceId }],
    })
  }

  async function addWorkItemToCase(caseId: string, item: ControlTowerWorkItem) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/links`, {
        entityType: item.itemType,
        entityId: item.sourceId,
      })
      pushToast(t('controlTower.cases.linked'), 'success')
      if (drawerOpen.value && selectedCase.value?.id === caseId) {
        await openCase(caseId)
      }
      await loadCases()
      return true
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
      return false
    } finally {
      actionLoading.value = false
    }
  }

  async function findDuplicateCandidates(item: ControlTowerWorkItem) {
    try {
      const response = await apiGet<{ items: ControlTowerOperationalCase[] }>(
        '/api/v1/control-tower/cases/duplicates',
        {
          query: {
            itemType: item.itemType,
            itemId: item.sourceId,
            shipmentId: item.shipmentId,
          },
        },
      )
      return response.items ?? []
    } catch {
      return []
    }
  }

  async function claimCase(caseId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/claim`)
      await refreshOpenCase(caseId)
      pushToast(t('controlTower.cases.claimSuccess'), 'success')
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function resolveCase(caseId: string, resolutionCode: ControlTowerCaseResolutionCode, summary?: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/resolve`, {
        resolutionCode,
        resolutionSummary: summary?.trim() || undefined,
      })
      await refreshOpenCase(caseId)
      await loadCases()
      await loadKpi()
      pushToast(t('controlTower.cases.resolveSuccess'), 'success')
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function closeCase(caseId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/close`)
      await refreshOpenCase(caseId)
      await loadCases()
      await loadKpi()
      pushToast(t('controlTower.cases.closeSuccess'), 'success')
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function reopenCase(caseId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/reopen`)
      await refreshOpenCase(caseId)
      await loadCases()
      await loadKpi()
      pushToast(t('controlTower.cases.reopenSuccess'), 'success')
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function addNote(caseId: string, body: string, mentionedUserIds: string[] = []) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/notes`, { body, mentionedUserIds })
      await refreshOpenCase(caseId)
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function createActionItem(caseId: string, title: string, description?: string, assigneeUserId?: string, dueAt?: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/actions`, {
        title,
        description,
        assigneeUserId,
        dueAt,
      })
      await refreshOpenCase(caseId)
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function completeActionItem(caseId: string, actionId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/actions/${actionId}/complete`)
      await refreshOpenCase(caseId)
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function recordDecision(caseId: string, decision: string, rationale?: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/decisions`, { decision, rationale })
      await refreshOpenCase(caseId)
      pushToast(t('controlTower.cases.decisionRecorded'), 'success')
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function refreshOpenCase(caseId: string) {
    if (drawerOpen.value && selectedCase.value?.id === caseId) {
      await openCase(caseId)
    }
  }

  async function goToPage(nextPage: number) {
    if (nextPage < 1) return
    page.value = nextPage
    await loadCases()
  }

  function isActionOverdue(item: ControlTowerCaseActionItem): boolean {
    if (!item.dueAt || item.status === 'done' || item.status === 'cancelled') return false
    return new Date(item.dueAt).getTime() < Date.now()
  }

  return {
    loading,
    actionLoading,
    cases,
    selectedCase,
    drawerOpen,
    activePreset,
    page,
    limit,
    total,
    hasNext,
    kpi,
    loadCases,
    loadKpi,
    openCase,
    createCase,
    createCaseFromWorkItem,
    addWorkItemToCase,
    findDuplicateCandidates,
    claimCase,
    resolveCase,
    closeCase,
    reopenCase,
    addNote,
    createActionItem,
    completeActionItem,
    recordDecision,
    goToPage,
    isActionOverdue,
  }
}
