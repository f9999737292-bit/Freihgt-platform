import type {
  ControlTowerBulkActionOutcome,
  ControlTowerSavedView,
  ControlTowerWorkItem,
  ControlTowerWorkItemsResponse,
  ControlTowerWorkspaceKpi,
  ControlTowerWorkspacePreset,
} from '~/types/controlTower'
import { ApiError, formatApiErrorForUser } from '~/composables/useApi'

export function useOperatorWorkspace() {
  const { apiGet, apiPost } = useApi()
  const { pushToast } = useToast()
  const { t } = useI18n()

  const loading = ref(false)
  const actionLoading = ref(false)
  const workItems = ref<ControlTowerWorkItem[]>([])
  const selectedIds = ref<Set<string>>(new Set())
  const activePreset = ref<ControlTowerWorkspacePreset>('my_work')
  const page = ref(1)
  const limit = ref(50)
  const total = ref(0)
  const hasNext = ref(false)
  const kpi = ref<ControlTowerWorkspaceKpi | null>(null)
  const savedViews = ref<ControlTowerSavedView[]>([])
  const selectedItem = ref<ControlTowerWorkItem | null>(null)
  const drawerOpen = ref(false)
  const lastBulkOutcome = ref<ControlTowerBulkActionOutcome | null>(null)

  async function loadWorkItems(preset?: ControlTowerWorkspacePreset) {
    if (preset) activePreset.value = preset
    loading.value = true
    try {
      const query: Record<string, string | number | boolean> = {
        preset: activePreset.value,
        page: page.value,
        limit: limit.value,
      }
      if (activePreset.value === 'my_work') query.my_work = true
      if (activePreset.value === 'unassigned') query.unassigned = true
      const response = await apiGet<ControlTowerWorkItemsResponse>('/api/v1/control-tower/work-items', { query })
      workItems.value = response.items ?? []
      page.value = response.page ?? 1
      limit.value = response.limit ?? 50
      total.value = response.total ?? 0
      hasNext.value = response.hasNext ?? false
      kpi.value = response.kpi ?? null
      selectedIds.value = new Set()
    } catch (error) {
      workItems.value = []
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      loading.value = false
    }
  }

  async function loadSavedViews() {
    try {
      const response = await apiGet<{ items: ControlTowerSavedView[] }>('/api/v1/control-tower/views')
      savedViews.value = response.items ?? []
    } catch {
      savedViews.value = []
    }
  }

  async function openDetails(item: ControlTowerWorkItem) {
    drawerOpen.value = true
    selectedItem.value = item
    try {
      const detail = await apiGet<ControlTowerWorkItem>(
        `/api/v1/control-tower/work-items/${item.itemType}/${item.sourceId}`,
      )
      selectedItem.value = detail
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    }
  }

  function toggleSelection(item: ControlTowerWorkItem) {
    const next = new Set(selectedIds.value)
    if (next.has(item.id)) next.delete(item.id)
    else next.add(item.id)
    selectedIds.value = next
  }

  function selectAllVisible() {
    selectedIds.value = new Set(workItems.value.map((item) => item.id))
  }

  function clearSelection() {
    selectedIds.value = new Set()
  }

  async function claimItem(item: ControlTowerWorkItem) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/work-items/${item.itemType}/${item.sourceId}/claim`)
      pushToast(t('controlTower.workspace.claimSuccess'), 'success')
      await loadWorkItems()
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        pushToast(t('controlTower.workspace.ownershipChanged'), 'warning')
        await loadWorkItems()
        return
      }
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function assignSelected(userId: string) {
    if (!userId || selectedIds.value.size === 0) return
    actionLoading.value = true
    try {
      const items = workItems.value
        .filter((item) => selectedIds.value.has(item.id))
        .map((item) => ({ itemType: item.itemType, itemId: item.sourceId }))
      const outcome = await apiPost<ControlTowerBulkActionOutcome>('/api/v1/control-tower/work-items/bulk-action', {
        action: 'assign',
        items,
        targetUserId: userId,
      })
      lastBulkOutcome.value = outcome
      pushToast(
        t('controlTower.workspace.bulkPartialSuccess', { succeeded: outcome.succeeded, failed: outcome.failed }),
        outcome.failed > 0 ? 'warning' : 'success',
      )
      await loadWorkItems()
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function claimSelected() {
    if (selectedIds.value.size === 0) return
    actionLoading.value = true
    try {
      const items = workItems.value
        .filter((item) => selectedIds.value.has(item.id))
        .map((item) => ({ itemType: item.itemType, itemId: item.sourceId }))
      const outcome = await apiPost<ControlTowerBulkActionOutcome>('/api/v1/control-tower/work-items/bulk-action', {
        action: 'claim',
        items,
      })
      lastBulkOutcome.value = outcome
      pushToast(
        t('controlTower.workspace.bulkPartialSuccess', { succeeded: outcome.succeeded, failed: outcome.failed }),
        outcome.failed > 0 ? 'warning' : 'success',
      )
      await loadWorkItems()
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      actionLoading.value = false
    }
  }

  async function saveCurrentView(name: string) {
    await apiPost('/api/v1/control-tower/views', {
      name,
      scope: 'private',
      filters: { preset: activePreset.value },
      sort: {},
    })
    await loadSavedViews()
    pushToast(t('controlTower.workspace.savedViewCreated'), 'success')
  }

  const selectedCount = computed(() => selectedIds.value.size)

  return {
    loading,
    actionLoading,
    workItems,
    selectedIds,
    selectedCount,
    activePreset,
    page,
    total,
    hasNext,
    kpi,
    savedViews,
    selectedItem,
    drawerOpen,
    lastBulkOutcome,
    loadWorkItems,
    loadSavedViews,
    openDetails,
    toggleSelection,
    selectAllVisible,
    clearSelection,
    claimItem,
    claimSelected,
    assignSelected,
    saveCurrentView,
  }
}
