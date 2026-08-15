import type {
  ControlTowerCaseActionItem,
  ControlTowerCaseKpi,
  ControlTowerCasePreset,
  ControlTowerCaseResolutionCode,
  ControlTowerCaseSeverity,
  ControlTowerCaseTimelineEntry,
  ControlTowerCaseTimelineResponse,
  ControlTowerCasesResponse,
  ControlTowerCaseDuplicateCandidate,
  ControlTowerOperationalCase,
  ControlTowerSavedView,
  ControlTowerWorkItem,
} from '~/types/controlTower'
import { ApiError, formatApiErrorForUser } from '~/composables/useApi'
import { isSeverityDecrease } from '~/composables/useCaseDisplay'

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
  const searchQuery = useState('ct-cases-search', () => '')
  const slaFilter = useState<'none' | 'breached' | 'warning' | 'at_risk'>('ct-cases-sla-filter', () => 'none')
  const page = useState('ct-cases-page', () => 1)
  const limit = useState('ct-cases-limit', () => 50)
  const total = useState('ct-cases-total', () => 0)
  const hasNext = useState('ct-cases-has-next', () => false)
  const kpi = useState<ControlTowerCaseKpi | null>('ct-cases-kpi', () => null)
  const savedViews = useState<ControlTowerSavedView[]>('ct-cases-saved-views', () => [])
  const timelineEntries = useState<ControlTowerCaseTimelineEntry[]>('ct-cases-timeline', () => [])
  const timelinePage = useState('ct-cases-timeline-page', () => 1)
  const timelineHasNext = useState('ct-cases-timeline-has-next', () => false)
  const timelineLoading = useState('ct-cases-timeline-loading', () => false)

  const defaultSavedView = computed(() => savedViews.value.find((v) => v.isDefault) ?? null)

  function buildQuery(): Record<string, string | number | boolean> {
    const query: Record<string, string | number | boolean> = {
      preset: activePreset.value,
      page: page.value,
      limit: limit.value,
    }
    if (searchQuery.value.trim()) {
      query.search = searchQuery.value.trim()
    }
    if (slaFilter.value === 'breached') query.hasSlaBreach = true
    if (slaFilter.value === 'warning') query.hasSlaWarning = true
    if (slaFilter.value === 'at_risk') query.preset = 'sla_at_risk'
    return query
  }

  function handleConflict(error: unknown, refreshFn?: () => Promise<void>) {
    if (error instanceof ApiError && error.status === 409) {
      pushToast('warning', t('controlTower.cases.conflictRefresh'))
      if (refreshFn) void refreshFn()
      return true
    }
    return false
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
      pushToast('error', formatApiErrorForUser(error))
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

  async function loadSavedViews() {
    try {
      const response = await apiGet<{ items: ControlTowerSavedView[] }>('/api/v1/control-tower/views', {
        query: { workspaceScope: 'cases' },
      })
      savedViews.value = response.items ?? []
    } catch {
      savedViews.value = []
    }
  }

  async function refreshCaseWorkspace(options?: { keepDrawer?: boolean }) {
    await Promise.all([loadCases(), loadKpi(), loadSavedViews()])
    if (options?.keepDrawer !== false && drawerOpen.value && selectedCase.value) {
      await openCase(selectedCase.value.id)
    }
  }

  async function openCase(caseId: string) {
    drawerOpen.value = true
    timelineEntries.value = []
    timelinePage.value = 1
    try {
      selectedCase.value = await apiGet<ControlTowerOperationalCase>(`/api/v1/control-tower/cases/${caseId}`)
      await loadTimeline(caseId, 1, false)
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
      drawerOpen.value = false
    }
  }

  async function loadTimeline(caseId: string, nextPage = 1, append = false) {
    timelineLoading.value = true
    try {
      const response = await apiGet<ControlTowerCaseTimelineResponse>(
        `/api/v1/control-tower/cases/${caseId}/timeline`,
        { query: { page: nextPage, limit: 30 } },
      )
      const items = response.items ?? []
      timelineEntries.value = append ? [...timelineEntries.value, ...items] : items
      timelinePage.value = response.page ?? nextPage
      timelineHasNext.value = response.hasNext ?? false
    } catch (error) {
      if (!append) timelineEntries.value = []
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      timelineLoading.value = false
    }
  }

  async function loadOlderTimeline(caseId: string) {
    if (!timelineHasNext.value || timelineLoading.value) return
    await loadTimeline(caseId, timelinePage.value + 1, true)
  }

  async function createCase(payload: {
    title: string
    summary?: string
    severity?: string
    shipmentIds?: string[]
    workItems?: { itemType: string; itemId: string }[]
    participantUserIds?: string[]
  }) {
    actionLoading.value = true
    try {
      const created = await apiPost<ControlTowerOperationalCase>('/api/v1/control-tower/cases', payload)
      pushToast('success', t('controlTower.cases.created'))
      await refreshCaseWorkspace({ keepDrawer: false })
      return created
    } catch (error) {
      if (!handleConflict(error, () => refreshCaseWorkspace({ keepDrawer: false }))) {
        pushToast('error', formatApiErrorForUser(error))
      }
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
      pushToast('success', t('controlTower.cases.linked'))
      await refreshCaseWorkspace()
      return true
    } catch (error) {
      if (!handleConflict(error, () => refreshCaseWorkspace())) {
        pushToast('error', formatApiErrorForUser(error))
      }
      return false
    } finally {
      actionLoading.value = false
    }
  }

  async function findDuplicateCandidates(item: ControlTowerWorkItem): Promise<ControlTowerCaseDuplicateCandidate[]> {
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
      return (response.items ?? []).map((item) => ({
        id: (item as { caseId?: string; id?: string }).caseId ?? item.id,
        reference: item.reference,
        title: item.title,
        status: item.status,
        effectiveSeverity: item.effectiveSeverity,
        ownerDisplayName: item.ownerDisplayName,
      }))
    } catch {
      return []
    }
  }

  async function updateCase(caseId: string, patch: Record<string, unknown>, version: number) {
    actionLoading.value = true
    try {
      const updated = await apiPatch<ControlTowerOperationalCase>(`/api/v1/control-tower/cases/${caseId}`, {
        ...patch,
        version,
      })
      selectedCase.value = updated
      await refreshCaseWorkspace()
      return updated
    } catch (error) {
      if (!handleConflict(error, () => openCase(caseId))) {
        pushToast('error', formatApiErrorForUser(error))
      }
      return null
    } finally {
      actionLoading.value = false
    }
  }

  async function setSeverityOverride(caseId: string, severity: ControlTowerCaseSeverity, current: ControlTowerOperationalCase) {
    if (
      isSeverityDecrease(current.effectiveSeverity, severity) &&
      !window.confirm(t('controlTower.cases.severityOverrideConfirm'))
    ) {
      return null
    }
    return updateCase(caseId, { severity }, current.version)
  }

  async function clearSeverityOverride(caseId: string, current: ControlTowerOperationalCase) {
    return updateCase(caseId, { clearSeverityOverride: true }, current.version)
  }

  async function addParticipant(caseId: string, userId: string, role: 'collaborator' | 'observer') {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/participants`, { userId, role })
      pushToast('success', t('controlTower.cases.participantAdded'))
      await refreshCaseWorkspace()
      return true
    } catch (error) {
      if (!handleConflict(error, () => openCase(caseId))) {
        pushToast('error', formatApiErrorForUser(error))
      }
      return false
    } finally {
      actionLoading.value = false
    }
  }

  async function updateParticipantRole(caseId: string, userId: string, role: 'collaborator' | 'observer') {
    actionLoading.value = true
    try {
      await apiPatch(`/api/v1/control-tower/cases/${caseId}/participants/${userId}`, { role })
      pushToast('success', t('controlTower.cases.participantRoleChanged'))
      await refreshCaseWorkspace()
      return true
    } catch (error) {
      if (!handleConflict(error, () => openCase(caseId))) {
        pushToast('error', formatApiErrorForUser(error))
      }
      return false
    } finally {
      actionLoading.value = false
    }
  }

  async function removeParticipant(caseId: string, userId: string) {
    actionLoading.value = true
    try {
      await apiDelete(`/api/v1/control-tower/cases/${caseId}/participants/${userId}`)
      pushToast('success', t('controlTower.cases.participantRemoved'))
      await refreshCaseWorkspace()
      return true
    } catch (error) {
      if (!handleConflict(error, () => openCase(caseId))) {
        pushToast('error', formatApiErrorForUser(error))
      }
      return false
    } finally {
      actionLoading.value = false
    }
  }

  async function claimCase(caseId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/claim`)
      await refreshCaseWorkspace()
      pushToast('success', t('controlTower.cases.claimSuccess'))
    } catch (error) {
      if (!handleConflict(error, () => refreshCaseWorkspace())) {
        pushToast('error', formatApiErrorForUser(error))
      }
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
      await refreshCaseWorkspace()
      pushToast('success', t('controlTower.cases.resolveSuccess'))
    } catch (error) {
      if (!handleConflict(error, () => refreshCaseWorkspace())) {
        pushToast('error', formatApiErrorForUser(error))
      }
    } finally {
      actionLoading.value = false
    }
  }

  async function closeCase(caseId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/close`)
      await refreshCaseWorkspace()
      pushToast('success', t('controlTower.cases.closeSuccess'))
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function reopenCase(caseId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/reopen`)
      await refreshCaseWorkspace()
      pushToast('success', t('controlTower.cases.reopenSuccess'))
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function addNote(caseId: string, body: string, mentionedUserIds: string[] = []) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/notes`, { body, mentionedUserIds })
      await refreshCaseWorkspace()
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function createActionItem(
    caseId: string,
    title: string,
    description?: string,
    assigneeUserId?: string,
    dueAt?: string,
  ) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/actions`, {
        title,
        description,
        assigneeUserId,
        dueAt,
      })
      await refreshCaseWorkspace()
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function updateActionItem(caseId: string, actionId: string, patch: Record<string, unknown>) {
    actionLoading.value = true
    try {
      await apiPatch(`/api/v1/control-tower/cases/${caseId}/actions/${actionId}`, patch)
      await refreshCaseWorkspace()
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function completeActionItem(caseId: string, actionId: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/actions/${actionId}/complete`)
      await refreshCaseWorkspace()
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function recordDecision(caseId: string, decision: string, rationale?: string) {
    actionLoading.value = true
    try {
      await apiPost(`/api/v1/control-tower/cases/${caseId}/decisions`, { decision, rationale })
      await refreshCaseWorkspace()
      pushToast('success', t('controlTower.cases.decisionRecorded'))
    } catch (error) {
      pushToast('error', formatApiErrorForUser(error))
    } finally {
      actionLoading.value = false
    }
  }

  async function createSavedView(payload: { name: string; scope: 'private' | 'shared'; isDefault?: boolean }) {
    const view = await apiPost<ControlTowerSavedView>('/api/v1/control-tower/views', {
      name: payload.name.trim(),
      scope: payload.scope,
      workspaceScope: 'cases',
      filters: { preset: activePreset.value, search: searchQuery.value.trim() || undefined, slaFilter: slaFilter.value },
      sort: {},
    })
    if (payload.isDefault) {
      await apiPost(`/api/v1/control-tower/views/${view.id}/set-default`)
    }
    await loadSavedViews()
    pushToast('success', t('controlTower.workspace.savedViewCreated'))
  }

  async function updateSavedView(viewId: string, patch: Record<string, unknown>) {
    await apiPatch<ControlTowerSavedView>(`/api/v1/control-tower/views/${viewId}`, patch)
    await loadSavedViews()
    pushToast('success', t('controlTower.workspace.savedViewUpdated'))
  }

  async function updateSavedViewWithCurrentFilters(viewId: string) {
    await updateSavedView(viewId, {
      filters: { preset: activePreset.value, search: searchQuery.value.trim() || undefined, slaFilter: slaFilter.value },
    })
  }

  async function renameSavedView(viewId: string, name: string) {
    await updateSavedView(viewId, { name: name.trim() })
  }

  async function duplicateSavedView(view: ControlTowerSavedView) {
    await apiPost<ControlTowerSavedView>('/api/v1/control-tower/views', {
      name: `${view.name} (${t('controlTower.workspace.duplicateSuffix')})`,
      scope: 'private',
      workspaceScope: 'cases',
      filters: view.filters ?? { preset: activePreset.value },
      sort: view.sort ?? {},
    })
    await loadSavedViews()
    pushToast('success', t('controlTower.workspace.savedViewCreated'))
  }

  async function deleteSavedView(viewId: string) {
    await apiDelete(`/api/v1/control-tower/views/${viewId}`)
    await loadSavedViews()
    pushToast('success', t('controlTower.workspace.savedViewDeleted'))
  }

  async function setDefaultSavedView(viewId: string) {
    await apiPost(`/api/v1/control-tower/views/${viewId}/set-default`)
    await loadSavedViews()
    pushToast('success', t('controlTower.workspace.defaultViewSet'))
  }

  async function applySavedView(view: ControlTowerSavedView) {
    const preset = (view.filters?.preset as ControlTowerCasePreset | undefined) ?? 'my_cases'
    searchQuery.value = typeof view.filters?.search === 'string' ? view.filters.search : ''
    const sf = view.filters?.slaFilter
    slaFilter.value = sf === 'breached' || sf === 'warning' || sf === 'at_risk' ? sf : 'none'
    await loadCases({ preset, resetPage: true })
  }

  async function applyInitialCaseView() {
    await loadSavedViews()
    const def = defaultSavedView.value
    if (def) {
      await applySavedView(def)
    } else {
      await loadCases({ preset: 'my_cases', resetPage: true })
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
    searchQuery,
    slaFilter,
    page,
    limit,
    total,
    hasNext,
    kpi,
    savedViews,
    defaultSavedView,
    timelineEntries,
    timelinePage,
    timelineHasNext,
    timelineLoading,
    loadCases,
    loadKpi,
    loadSavedViews,
    refreshCaseWorkspace,
    openCase,
    loadTimeline,
    loadOlderTimeline,
    createCase,
    createCaseFromWorkItem,
    addWorkItemToCase,
    findDuplicateCandidates,
    updateCase,
    setSeverityOverride,
    clearSeverityOverride,
    addParticipant,
    updateParticipantRole,
    removeParticipant,
    claimCase,
    resolveCase,
    closeCase,
    reopenCase,
    addNote,
    createActionItem,
    updateActionItem,
    completeActionItem,
    recordDecision,
    createSavedView,
    updateSavedViewWithCurrentFilters,
    renameSavedView,
    duplicateSavedView,
    deleteSavedView,
    setDefaultSavedView,
    applySavedView,
    applyInitialCaseView,
    goToPage,
    isActionOverdue,
  }
}
