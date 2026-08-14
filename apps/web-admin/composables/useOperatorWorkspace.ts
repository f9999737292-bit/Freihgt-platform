import type {
  ControlTowerBulkActionOutcome,
  ControlTowerBulkActionType,
  ControlTowerHandoff,
  ControlTowerHandoffCreateResult,
  ControlTowerQueueMode,
  ControlTowerSavedView,
  ControlTowerWorkItem,
  ControlTowerWorkItemsResponse,
  ControlTowerWorkloadResponse,
  ControlTowerWorkspaceKpi,
  ControlTowerWorkspacePreset,
} from '~/types/controlTower'
import { ApiError, formatApiErrorForUser } from '~/composables/useApi'

function mapBulkErrorMessage(raw: string | undefined, t: (key: string) => string): string {
  if (!raw) return t('controlTower.workspace.bulkReasonUnknown')
  const lower = raw.toLowerCase()
  if (lower.includes('already claimed') || lower.includes('ownership')) {
    return t('controlTower.workspace.bulkReasonOwnershipChanged')
  }
  if (lower.includes('resolved') || lower.includes('cleared') || lower.includes('materialized')) {
    return t('controlTower.workspace.bulkReasonItemCompleted')
  }
  if (lower.includes('forbidden') || lower.includes('denied')) {
    return t('controlTower.workspace.bulkReasonPermissionDenied')
  }
  if (lower.includes('not found')) {
    return t('controlTower.workspace.bulkReasonItemUnavailable')
  }
  return t('controlTower.workspace.bulkReasonUnknown')
}

export function timelineCategory(source: string, actionType: string): string {
  if (source === 'handoff' || actionType.includes('handoff')) return 'HANDOFF'
  if (source === 'risk' || actionType.startsWith('risk_')) return 'RISK'
  if (source === 'workflow' || ['acknowledged', 'assigned', 'resolved', 'claimed'].includes(actionType)) {
    return 'WORKFLOW'
  }
  if (source === 'system' || actionType.includes('sla_breached')) return 'SYSTEM'
  return 'OPERATOR'
}

export function useOperatorWorkspace() {
  const { apiGet, apiPost, apiPatch, apiDelete } = useApi()
  const { pushToast } = useToast()
  const { t } = useI18n()
  const authStore = useAuthStore()

  const currentUserId = computed(() => authStore.user?.id ?? '')

  const loading = ref(false)
  const actionLoading = ref(false)
  const workItems = ref<ControlTowerWorkItem[]>([])
  const selectedIds = ref<Set<string>>(new Set())
  const activePreset = ref<ControlTowerWorkspacePreset>('my_work')
  const queueMode = ref<ControlTowerQueueMode>('active')
  const operatorFilterUserId = ref<string | null>(null)
  const page = ref(1)
  const limit = ref(50)
  const total = ref(0)
  const hasNext = ref(false)
  const kpi = ref<ControlTowerWorkspaceKpi | null>(null)
  const savedViews = ref<ControlTowerSavedView[]>([])
  const selectedItem = ref<ControlTowerWorkItem | null>(null)
  const drawerOpen = ref(false)
  const lastBulkOutcome = ref<ControlTowerBulkActionOutcome | null>(null)
  const lastBulkAction = ref<ControlTowerBulkActionType | null>(null)
  const workload = ref<ControlTowerWorkloadResponse | null>(null)
  const handoffs = ref<ControlTowerHandoff[]>([])
  const selectedHandoff = ref<ControlTowerHandoff | null>(null)
  const handoffDetailsOpen = ref(false)

  const selectedItems = computed(() =>
    workItems.value.filter((item) => selectedIds.value.has(item.id)),
  )

  const selectedCount = computed(() => selectedIds.value.size)

  const defaultSavedView = computed(() => savedViews.value.find((v) => v.isDefault) ?? null)

  function buildWorkItemsQuery(): Record<string, string | number | boolean> {
    const query: Record<string, string | number | boolean> = {
      preset: activePreset.value,
      page: page.value,
      limit: limit.value,
    }
    if (queueMode.value === 'completed' || activePreset.value === 'completed') {
      query.include_completed = true
      query.preset = 'completed'
    } else if (activePreset.value === 'my_work') {
      query.my_work = true
    } else if (activePreset.value === 'unassigned') {
      query.unassigned = true
    }
    if (operatorFilterUserId.value) {
      query.ownerUserId = operatorFilterUserId.value
      delete query.my_work
      delete query.unassigned
    }
    return query
  }

  async function loadWorkItems(options?: {
    preset?: ControlTowerWorkspacePreset
    resetPage?: boolean
    mode?: ControlTowerQueueMode
  }) {
    if (options?.preset) activePreset.value = options.preset
    if (options?.mode) queueMode.value = options.mode
    if (options?.resetPage) page.value = 1

    loading.value = true
    try {
      const response = await apiGet<ControlTowerWorkItemsResponse>('/api/v1/control-tower/work-items', {
        query: buildWorkItemsQuery(),
      })
      workItems.value = response.items ?? []
      page.value = response.page ?? 1
      limit.value = response.limit ?? 50
      total.value = response.total ?? 0
      hasNext.value = response.hasNext ?? false
      kpi.value = response.kpi ?? null

      if (workItems.value.length === 0 && page.value > 1 && total.value > 0) {
        page.value = Math.max(1, Math.ceil(total.value / limit.value))
        return loadWorkItems()
      }

      const staleSelection = [...selectedIds.value].filter(
        (id) => !workItems.value.some((item) => item.id === id),
      )
      if (staleSelection.length > 0) {
        const next = new Set(selectedIds.value)
        staleSelection.forEach((id) => next.delete(id))
        selectedIds.value = next
      }
    } catch (error) {
      workItems.value = []
      pushToast(formatApiErrorForUser(error), 'error')
    } finally {
      loading.value = false
    }
  }

  async function loadWorkload() {
    try {
      workload.value = await apiGet<ControlTowerWorkloadResponse>('/api/v1/control-tower/workload')
    } catch {
      workload.value = null
    }
  }

  async function loadHandoffs() {
    try {
      const response = await apiGet<{ items: ControlTowerHandoff[] }>('/api/v1/control-tower/handoffs', {
        query: { limit: 20 },
      })
      handoffs.value = response.items ?? []
    } catch {
      handoffs.value = []
    }
  }

  async function loadHandoffDetails(handoffId: string) {
    selectedHandoff.value = await apiGet<ControlTowerHandoff>(`/api/v1/control-tower/handoffs/${handoffId}`)
    handoffDetailsOpen.value = true
  }

  async function loadSavedViews() {
    try {
      const response = await apiGet<{ items: ControlTowerSavedView[] }>('/api/v1/control-tower/views', {
        query: { workspaceScope: 'work_items' },
      })
      savedViews.value = response.items ?? []
    } catch {
      savedViews.value = []
    }
  }

  async function refreshWorkspace(options?: { keepSelection?: boolean }) {
    await Promise.all([loadWorkItems(), loadWorkload(), loadHandoffs(), loadSavedViews()])
    if (!options?.keepSelection) {
      // selection reconciled in loadWorkItems
    }
    if (drawerOpen.value && selectedItem.value) {
      try {
        const detail = await apiGet<ControlTowerWorkItem>(
          `/api/v1/control-tower/work-items/${selectedItem.value.itemType}/${selectedItem.value.sourceId}`,
        )
        selectedItem.value = detail
      } catch {
        drawerOpen.value = false
        selectedItem.value = null
      }
    }
  }

  async function applyInitialView() {
    await loadSavedViews()
    const def = defaultSavedView.value
    if (def?.filters?.preset && typeof def.filters.preset === 'string') {
      await loadWorkItems({
        preset: def.filters.preset as ControlTowerWorkspacePreset,
        resetPage: true,
      })
    } else {
      await loadWorkItems({ preset: 'my_work', resetPage: true })
    }
  }

  async function openDetails(item: ControlTowerWorkItem) {
    drawerOpen.value = true
    selectedItem.value = item
    try {
      selectedItem.value = await apiGet<ControlTowerWorkItem>(
        `/api/v1/control-tower/work-items/${item.itemType}/${item.sourceId}`,
      )
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
    }
  }

  async function openDetailsByKey(itemType: ControlTowerWorkItem['itemType'], sourceId: string) {
    await openDetails({
      id: `${itemType}:${sourceId}`,
      itemType,
      sourceId,
    } as ControlTowerWorkItem)
  }

  async function openLinkedException(eventId: string) {
    drawerOpen.value = true
    try {
      selectedItem.value = await apiGet<ControlTowerWorkItem>(
        `/api/v1/control-tower/work-items/exception/${eventId}`,
      )
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

  function supportedBulkActions(items: ControlTowerWorkItem[]): ControlTowerBulkActionType[] {
    if (items.length === 0) return []
    const sets = items.map((item) => new Set(item.availableActions))
    const intersection = ['claim', 'assign', 'unassign', 'acknowledge'].filter((action) =>
      sets.every((s) => s.has(action)),
    ) as ControlTowerBulkActionType[]
    return intersection
  }

  const bulkActionsAvailable = computed(() => supportedBulkActions(selectedItems.value))

  const workloadFilterTarget = computed(() => {
    if (operatorFilterUserId.value) return operatorFilterUserId.value
    if (queueMode.value === 'active' && activePreset.value === 'unassigned') return 'unassigned'
    return null
  })

  async function runBulkAction(
    action: ControlTowerBulkActionType,
    targetUserId?: string,
    itemsOverride?: ControlTowerWorkItem[],
  ) {
    const items = (itemsOverride ?? selectedItems.value).map((item) => ({
      itemType: item.itemType,
      itemId: item.sourceId,
    }))
    if (items.length === 0) return null

    actionLoading.value = true
    lastBulkAction.value = action
    try {
      const body: Record<string, unknown> = { action, items }
      if (targetUserId) body.targetUserId = targetUserId
      const outcome = await apiPost<ControlTowerBulkActionOutcome>(
        '/api/v1/control-tower/work-items/bulk-action',
        body,
      )
      lastBulkOutcome.value = outcome
      pushToast(
        t('controlTower.workspace.bulkSummary', {
          requested: outcome.requested,
          succeeded: outcome.succeeded,
          failed: outcome.failed,
        }),
        outcome.failed > 0 ? 'warning' : 'success',
      )
      await refreshWorkspace()
      return outcome
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
      return null
    } finally {
      actionLoading.value = false
    }
  }

  async function retryFailedBulk() {
    if (!lastBulkOutcome.value || !lastBulkAction.value) return
    const failedItems = lastBulkOutcome.value.results
      .filter((r) => !r.success)
      .map(
        (r) =>
          ({
            id: `${r.itemType}:${r.itemId}`,
            itemType: r.itemType,
            sourceId: r.itemId,
          }) as ControlTowerWorkItem,
      )
    if (failedItems.length === 0) return
    await runBulkAction(lastBulkAction.value, undefined, failedItems)
  }

  async function claimItem(item: ControlTowerWorkItem) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/work-items/${item.itemType}/${item.sourceId}/claim`)
      pushToast(t('controlTower.workspace.claimSuccess'), 'success')
      await refreshWorkspace({ keepSelection: true })
    } catch (error) {
      handleOwnershipConflict(error)
    } finally {
      actionLoading.value = false
    }
  }

  function handleOwnershipConflict(error: unknown) {
    if (error instanceof ApiError && error.status === 409) {
      pushToast(t('controlTower.workspace.ownershipChanged'), 'warning')
      void refreshWorkspace()
      return
    }
    pushToast(formatApiErrorForUser(error), 'error')
  }

  async function createHandoff(toUserId: string, note: string | undefined, items: ControlTowerWorkItem[]) {
    actionLoading.value = true
    try {
      const result = await apiPost<ControlTowerHandoffCreateResult>('/api/v1/control-tower/handoffs', {
        toUserId,
        note: note?.trim() || undefined,
        items: items.map((item) => ({ itemType: item.itemType, itemId: item.sourceId })),
      })
      clearSelection()
      await refreshWorkspace()
      return result
    } catch (error) {
      pushToast(formatApiErrorForUser(error), 'error')
      return null
    } finally {
      actionLoading.value = false
    }
  }

  async function createSavedView(payload: {
    name: string
    scope: 'private' | 'shared'
    isDefault?: boolean
  }) {
    const view = await apiPost<ControlTowerSavedView>('/api/v1/control-tower/views', {
      name: payload.name.trim(),
      scope: payload.scope,
      workspaceScope: 'work_items',
      filters: { preset: activePreset.value, queueMode: queueMode.value },
      sort: {},
    })
    if (payload.isDefault) {
      await apiPost(`/api/v1/control-tower/views/${view.id}/set-default`)
    }
    await loadSavedViews()
    pushToast(t('controlTower.workspace.savedViewCreated'), 'success')
  }

  async function updateSavedView(viewId: string, patch: Record<string, unknown>) {
    await apiPatch<ControlTowerSavedView>(`/api/v1/control-tower/views/${viewId}`, patch)
    await loadSavedViews()
    pushToast(t('controlTower.workspace.savedViewUpdated'), 'success')
  }

  async function updateSavedViewWithCurrentFilters(viewId: string) {
    await updateSavedView(viewId, {
      filters: { preset: activePreset.value, queueMode: queueMode.value },
    })
  }

  async function renameSavedView(viewId: string, name: string) {
    await updateSavedView(viewId, { name: name.trim() })
  }

  async function duplicateSavedView(view: ControlTowerSavedView) {
    await apiPost<ControlTowerSavedView>('/api/v1/control-tower/views', {
      name: `${view.name} (${t('controlTower.workspace.duplicateSuffix')})`,
      scope: 'private',
      workspaceScope: view.workspaceScope ?? 'work_items',
      filters: view.filters ?? { preset: activePreset.value, queueMode: queueMode.value },
      sort: view.sort ?? {},
    })
    await loadSavedViews()
    pushToast(t('controlTower.workspace.savedViewCreated'), 'success')
  }

  async function deleteSavedView(viewId: string) {
    await apiDelete(`/api/v1/control-tower/views/${viewId}`)
    await loadSavedViews()
    if (defaultSavedView.value?.id === viewId) {
      await loadWorkItems({ preset: 'my_work', resetPage: true })
    }
    pushToast(t('controlTower.workspace.savedViewDeleted'), 'success')
  }

  async function setDefaultSavedView(viewId: string) {
    await apiPost(`/api/v1/control-tower/views/${viewId}/set-default`)
    await loadSavedViews()
    pushToast(t('controlTower.workspace.defaultViewSet'), 'success')
  }

  async function applySavedView(view: ControlTowerSavedView) {
    const preset = (view.filters?.preset as ControlTowerWorkspacePreset | undefined) ?? 'all_active'
    const mode = (view.filters?.queueMode as ControlTowerQueueMode | undefined) ?? 'active'
    operatorFilterUserId.value = null
    await loadWorkItems({ preset, mode, resetPage: true })
  }

  async function viewOperatorQueue(target: string | null) {
    if (target === 'unassigned') {
      operatorFilterUserId.value = null
      activePreset.value = 'unassigned'
      queueMode.value = 'active'
    } else if (target) {
      operatorFilterUserId.value = target
      activePreset.value = 'all_active'
      queueMode.value = 'active'
    } else {
      operatorFilterUserId.value = null
      activePreset.value = 'my_work'
      queueMode.value = 'active'
    }
    await loadWorkItems({ resetPage: true })
  }

  async function setQueueMode(mode: ControlTowerQueueMode) {
    queueMode.value = mode
    if (mode === 'completed') {
      activePreset.value = 'completed'
    } else if (activePreset.value === 'completed') {
      activePreset.value = 'my_work'
    }
    operatorFilterUserId.value = null
    await loadWorkItems({ resetPage: true })
  }

  async function goToPage(nextPage: number) {
    if (nextPage < 1) return
    page.value = nextPage
    await loadWorkItems()
  }

  function ownershipLabel(item: ControlTowerWorkItem): 'unassigned' | 'mine' | 'other' {
    if (!item.ownerUserId) return 'unassigned'
    if (item.ownerUserId === currentUserId.value) return 'mine'
    return 'other'
  }

  function formatBulkResult(error?: string): string {
    return mapBulkErrorMessage(error, t)
  }

  function emptyStateKey(): string {
    if (operatorFilterUserId.value) return 'operatorFiltered'
    if (queueMode.value === 'completed') return 'completed'
    switch (activePreset.value) {
      case 'my_work':
        return 'myWork'
      case 'unassigned':
        return 'unassigned'
      case 'sla_breached':
        return 'slaBreached'
      case 'critical':
        return 'critical'
      default:
        return 'generic'
    }
  }

  return {
    currentUserId,
    loading,
    actionLoading,
    workItems,
    selectedIds,
    selectedItems,
    selectedCount,
    activePreset,
    queueMode,
    operatorFilterUserId,
    page,
    limit,
    total,
    hasNext,
    kpi,
    savedViews,
    defaultSavedView,
    selectedItem,
    drawerOpen,
    lastBulkOutcome,
    lastBulkAction,
    workload,
    handoffs,
    selectedHandoff,
    handoffDetailsOpen,
    bulkActionsAvailable,
    workloadFilterTarget,
    loadWorkItems,
    loadWorkload,
    loadHandoffs,
    loadHandoffDetails,
    loadSavedViews,
    refreshWorkspace,
    applyInitialView,
    openDetails,
    openDetailsByKey,
    openLinkedException,
    toggleSelection,
    selectAllVisible,
    clearSelection,
    supportedBulkActions,
    runBulkAction,
    retryFailedBulk,
    claimItem,
    createHandoff,
    createSavedView,
    updateSavedViewWithCurrentFilters,
    renameSavedView,
    duplicateSavedView,
    deleteSavedView,
    setDefaultSavedView,
    applySavedView,
    viewOperatorQueue,
    setQueueMode,
    goToPage,
    ownershipLabel,
    formatBulkResult,
    emptyStateKey,
    handleOwnershipConflict,
  }
}
