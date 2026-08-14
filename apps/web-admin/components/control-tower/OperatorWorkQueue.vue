<script setup lang="ts">

import type { User } from '~/types/user'

import type { ControlTowerHandoffCreateResult, ControlTowerSavedView, ControlTowerWorkspacePreset, ControlTowerWorkspaceSection } from '~/types/controlTower'

import { CONTROL_TOWER_ACTIVE_PRESETS } from '~/types/controlTower'



const props = defineProps<{

  disabled?: boolean

  tenantUsers?: User[]

  canViewTeamWorkload?: boolean

}>()



const {

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

  refreshWorkspace,

  applyInitialView,

  openDetails,

  openDetailsByKey,

  openLinkedException,

  toggleSelection,

  selectAllVisible,

  clearSelection,

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

} = useOperatorWorkspace()



const { t } = useI18n()



const handoffModalOpen = ref(false)

const saveViewModalOpen = ref(false)

const assignUserId = ref('')

const handoffResult = ref<ControlTowerHandoffCreateResult | null>(null)
const workspaceSection = ref<ControlTowerWorkspaceSection>('work')
const caseFromWorkItemOpen = ref(false)
const caseFromWorkItem = ref<import('~/types/controlTower').ControlTowerWorkItem | null>(null)

const casesUi = useOperationalCases()



const presetOptions = computed(() =>

  props.disabled ? [] : [...CONTROL_TOWER_ACTIVE_PRESETS],

)



const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))



function presetLabel(preset: ControlTowerWorkspacePreset): string {

  return t(`controlTower.workspace.presets.${preset}`)

}



function itemTypeLabel(item: import('~/types/controlTower').ControlTowerWorkItem): string {

  return item.itemType === 'risk'

    ? t('controlTower.workspace.predictiveRisk')

    : t('controlTower.workspace.actualException')

}



function ownerCellLabel(item: import('~/types/controlTower').ControlTowerWorkItem): string {

  const kind = ownershipLabel(item)

  if (kind === 'mine') return t('controlTower.workspace.assignedToMe')

  if (kind === 'other') {

    return item.ownerDisplayName || t('controlTower.workspace.assignedToOther')

  }

  return t('controlTower.workspace.unassigned')

}



function dismissHandoffResult() {

  handoffResult.value = null

}



function dismissBulkOutcome() {

  lastBulkOutcome.value = null

}



function closeDrawer() {

  drawerOpen.value = false

}



function onOpenCase(caseId: string) {

  workspaceSection.value = 'cases'

  void casesUi.openCase(caseId)

}



function onCreateCaseFromWorkItem(item: import('~/types/controlTower').ControlTowerWorkItem) {

  caseFromWorkItem.value = item

  caseFromWorkItemOpen.value = true

}



function onCaseFromWorkItemDone(caseId: string) {
  caseFromWorkItemOpen.value = false
  caseFromWorkItem.value = null
  void refreshWorkspace({ keepSelection: true })
  void casesUi.refreshCaseWorkspace()
  void onOpenCase(caseId)
}



function closeCaseDrawer() {

  casesUi.drawerOpen = false

}



function closeHandoffDetails() {

  handoffDetailsOpen.value = false

}



async function onRefresh() {

  await refreshWorkspace()

}



async function onPresetChange(preset: ControlTowerWorkspacePreset) {

  operatorFilterUserId.value = null

  await loadWorkItems({ preset, mode: 'active', resetPage: true })

}



async function onQueueModeChange(mode: 'active' | 'completed') {

  await setQueueMode(mode)

}



async function onHandoffConfirm(toUserId: string, note: string | undefined) {

  const result = await createHandoff(toUserId, note, selectedItems.value)

  if (result) {

    handoffResult.value = result

    handoffModalOpen.value = false

  }

}



async function onSaveViewSubmit(payload: { name: string; scope: 'private' | 'shared'; isDefault: boolean }) {

  await createSavedView(payload)

  saveViewModalOpen.value = false

}



async function onDeleteView(view: ControlTowerSavedView) {

  if (!confirm(t('controlTower.workspace.deleteConfirm', { name: view.name }))) return

  await deleteSavedView(view.id)

}



onMounted(async () => {

  if (!props.disabled) {

    await applyInitialView()

    await Promise.all([loadWorkload(), loadHandoffs()])

  }

})



watch(

  () => props.disabled,

  (disabled) => {

    if (!disabled && workItems.value.length === 0) {

      void applyInitialView()

    }

  },

)

</script>



<template>

  <UiCard class="operator-workspace">

    <div class="operator-workspace__header">

      <div>

        <h2 class="operator-workspace__title">{{ $t('controlTower.workspace.title') }}</h2>

        <p class="operator-workspace__subtitle">{{ $t('controlTower.workspace.subtitle') }}</p>

      </div>

      <UiButton variant="secondary" size="sm" :disabled="disabled || loading" @click="onRefresh">

        {{ $t('controlTower.refresh') }}

      </UiButton>

    </div>



    <div class="operator-workspace__section-tabs" role="tablist">
      <button
        type="button"
        role="tab"
        :aria-selected="workspaceSection === 'work'"
        class="operator-workspace__mode-tab"
        :class="{ 'operator-workspace__mode-tab--active': workspaceSection === 'work' }"
        @click="workspaceSection = 'work'"
      >
        {{ $t('controlTower.cases.sections.work') }}
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="workspaceSection === 'cases'"
        class="operator-workspace__mode-tab"
        :class="{ 'operator-workspace__mode-tab--active': workspaceSection === 'cases' }"
        @click="workspaceSection = 'cases'"
      >
        {{ $t('controlTower.cases.sections.cases') }}
      </button>
    </div>



    <template v-if="workspaceSection === 'work'">

    <div v-if="kpi" class="operator-workspace__kpi">

      <span>{{ $t('controlTower.workspace.myActiveWork') }}: {{ kpi.myActiveWork }}</span>

      <span>{{ $t('controlTower.workspace.unassignedWork') }}: {{ kpi.unassignedWork }}</span>

      <span>{{ $t('controlTower.workspace.slaBreached') }}: {{ kpi.slaBreachedWork }}</span>

      <span>{{ $t('controlTower.workspace.criticalRiskWork') }}: {{ kpi.criticalRiskWork }}</span>

    </div>



    <div class="operator-workspace__mode-tabs" role="tablist">

      <button

        type="button"

        role="tab"

        :aria-selected="queueMode === 'active'"

        class="operator-workspace__mode-tab"

        :class="{ 'operator-workspace__mode-tab--active': queueMode === 'active' }"

        @click="onQueueModeChange('active')"

      >

        {{ $t('controlTower.workspace.active') }}

      </button>

      <button

        type="button"

        role="tab"

        :aria-selected="queueMode === 'completed'"

        class="operator-workspace__mode-tab"

        :class="{ 'operator-workspace__mode-tab--active': queueMode === 'completed' }"

        @click="onQueueModeChange('completed')"

      >

        {{ $t('controlTower.workspace.completed') }}

      </button>

    </div>



    <div v-if="queueMode === 'active'" class="operator-workspace__presets">

      <button

        v-for="preset in presetOptions"

        :key="preset"

        type="button"

        class="operator-workspace__preset"

        :class="{ 'operator-workspace__preset--active': activePreset === preset && !operatorFilterUserId }"

        :disabled="disabled || loading"

        @click="onPresetChange(preset)"

      >

        {{ presetLabel(preset) }}

      </button>

    </div>



    <p v-if="operatorFilterUserId" class="operator-workspace__filter-banner">

      {{ $t('controlTower.workspace.viewingOperatorQueue') }}

      <UiButton size="sm" variant="ghost" @click="viewOperatorQueue(null)">

        {{ $t('controlTower.workspace.clearOperatorFilter') }}

      </UiButton>

    </p>



    <ControlTowerOperatorTeamWorkloadPanel

      v-if="canViewTeamWorkload && !disabled"

      :operators="workload?.operators ?? []"

      :unassigned-pool="workload?.unassignedPool ?? 0"

      :loading="loading"

      :active-filter-target="workloadFilterTarget"

      @view-queue="viewOperatorQueue"

    />



    <ControlTowerOperatorSavedViewsPanel

      v-if="!disabled"

      :views="savedViews"

      :active-preset="activePreset"

      @apply="applySavedView"

      @rename="(v) => renameSavedView(v.id, v.name)"

      @update="(v) => updateSavedViewWithCurrentFilters(v.id)"

      @duplicate="duplicateSavedView"

      @delete="onDeleteView"

      @set-default="(v) => setDefaultSavedView(v.id)"

      @create="saveViewModalOpen = true"

    />



    <div v-if="selectedCount > 0" class="operator-workspace__bulk">

      <span>{{ $t('controlTower.workspace.selected', { count: selectedCount }) }}</span>

      <UiButton

        v-if="bulkActionsAvailable.includes('claim')"

        size="sm"

        :disabled="actionLoading"

        @click="runBulkAction('claim')"

      >

        {{ $t('controlTower.workspace.claim') }}

      </UiButton>

      <UiButton

        v-if="bulkActionsAvailable.includes('acknowledge')"

        size="sm"

        variant="secondary"

        :disabled="actionLoading"

        @click="runBulkAction('acknowledge')"

      >

        {{ $t('controlTower.workspace.acknowledge') }}

      </UiButton>

      <UiButton

        v-if="bulkActionsAvailable.includes('unassign')"

        size="sm"

        variant="secondary"

        :disabled="actionLoading"

        @click="runBulkAction('unassign')"

      >

        {{ $t('controlTower.workspace.unassign') }}

      </UiButton>

      <select v-model="assignUserId" class="operator-workspace__select" :aria-label="$t('controlTower.workspace.assign')">

        <option value="">{{ $t('controlTower.workspace.assign') }}</option>

        <option v-for="user in tenantUsers ?? []" :key="user.id" :value="user.id">

          {{ user.full_name }}

        </option>

      </select>

      <UiButton

        v-if="bulkActionsAvailable.includes('assign')"

        size="sm"

        variant="secondary"

        :disabled="!assignUserId || actionLoading"

        @click="runBulkAction('assign', assignUserId)"

      >

        {{ $t('controlTower.workspace.reassign') }}

      </UiButton>

      <UiButton

        size="sm"

        variant="secondary"

        :disabled="actionLoading || selectedItems.length === 0"

        @click="handoffModalOpen = true"

      >

        {{ $t('controlTower.workspace.handoff') }}

      </UiButton>

      <UiButton size="sm" variant="ghost" @click="clearSelection()">{{ $t('common.cancel') }}</UiButton>

    </div>



    <ControlTowerOperatorBulkResultPanel

      :outcome="lastBulkOutcome"

      :action="lastBulkAction"

      :format-reason="formatBulkResult"

      @retry="retryFailedBulk()"

      @dismiss="dismissBulkOutcome"

    />



    <section v-if="handoffResult" class="operator-workspace__handoff-result" aria-live="polite">

      <p>

        {{ $t('controlTower.workspace.transferred', { count: handoffResult.outcome.succeeded }) }}

        <template v-if="handoffResult.outcome.failed > 0">

          · {{ $t('controlTower.workspace.failed', { count: handoffResult.outcome.failed }) }}

        </template>

      </p>

      <table v-if="handoffResult.outcome.failed > 0" class="operator-workspace__handoff-failures">

        <thead>

          <tr>

            <th scope="col">{{ $t('controlTower.workspace.type') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.reference') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.reason') }}</th>

          </tr>

        </thead>

        <tbody>

          <tr

            v-for="row in handoffResult.outcome.results.filter((r) => !r.success)"

            :key="`${row.itemType}:${row.itemId}`"

          >

            <td>{{ row.itemType }}</td>

            <td>{{ row.itemId }}</td>

            <td>{{ formatBulkResult(row.error) }}</td>

          </tr>

        </tbody>

      </table>

      <UiButton size="sm" variant="ghost" @click="dismissHandoffResult">{{ $t('common.close') }}</UiButton>

    </section>



    <div class="operator-workspace__toolbar">

      <UiButton size="sm" variant="ghost" :disabled="loading" @click="selectAllVisible()">

        {{ $t('controlTower.workspace.selectAll') }}

      </UiButton>

    </div>



    <div v-if="loading" class="operator-workspace__empty">{{ $t('common.loading') }}</div>

    <div v-else-if="workItems.length === 0" class="operator-workspace__empty">

      {{ $t(`controlTower.workspace.emptyStates.${emptyStateKey()}`) }}

    </div>

    <div v-else class="operator-workspace__table-wrap">

      <table class="operator-workspace__table">

        <thead>

          <tr>

            <th scope="col" />

            <th scope="col">{{ $t('controlTower.workspace.type') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.shipment') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.titleColumn') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.urgencyColumn') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.owner') }}</th>

            <th scope="col">{{ $t('controlTower.workspace.actions') }}</th>

          </tr>

        </thead>

        <tbody>

          <tr

            v-for="item in workItems"

            :key="item.id"

            :class="{ 'operator-workspace__row--selected': selectedIds.has(item.id) }"

          >

            <td>

              <input

                type="checkbox"

                :checked="selectedIds.has(item.id)"

                :aria-label="$t('controlTower.workspace.selectItem', { title: item.title })"

                @change="toggleSelection(item)"

              />

            </td>

            <td>{{ itemTypeLabel(item) }}</td>

            <td>{{ item.shipmentNumber || item.shipmentId }}</td>

            <td>

              <button type="button" class="operator-workspace__link" @click="openDetails(item)">

                {{ item.title }}

              </button>

            </td>

            <td>

              <span class="operator-workspace__urgency" :data-urgency="item.urgency">

                {{ $t(`controlTower.workspace.urgencyLevels.${item.urgency}`) }}

              </span>

            </td>

            <td>

              <span class="operator-workspace__owner" :data-ownership="ownershipLabel(item)">

                {{ ownerCellLabel(item) }}

              </span>

            </td>

            <td>

              <UiButton

                v-if="item.availableActions.includes('claim')"

                size="sm"

                variant="ghost"

                :disabled="actionLoading"

                @click="claimItem(item)"

              >

                {{ $t('controlTower.workspace.claim') }}

              </UiButton>

            </td>

          </tr>

        </tbody>

      </table>



      <nav class="operator-workspace__pagination" :aria-label="$t('controlTower.workspace.pagination')">

        <UiButton size="sm" variant="ghost" :disabled="page <= 1" @click="goToPage(page - 1)">

          {{ $t('common.previous') }}

        </UiButton>

        <span>{{ $t('controlTower.workspace.pageOf', { page, total: totalPages }) }}</span>

        <UiButton size="sm" variant="ghost" :disabled="!hasNext" @click="goToPage(page + 1)">

          {{ $t('common.next') }}

        </UiButton>

        <span class="operator-workspace__count">{{ $t('controlTower.workspace.total', { total }) }}</span>

      </nav>

    </div>



    <ControlTowerOperatorHandoffHistoryPanel

      v-if="!disabled"

      :handoffs="handoffs"

      :users="tenantUsers ?? []"

      @open="loadHandoffDetails"

    />



    <ControlTowerOperatorWorkItemDrawer

      :open="drawerOpen"

      :item="selectedItem"

      :action-loading="actionLoading"

      :ownership-label="ownershipLabel"

      @close="closeDrawer"

      @claim="claimItem"

      @open-linked-exception="openLinkedException"

      @open-case="onOpenCase"

      @create-case="onCreateCaseFromWorkItem"

    />



    <ControlTowerOperatorCaseDetailsDrawer

      :open="casesUi.drawerOpen"

      :case-item="casesUi.selectedCase"

      :action-loading="casesUi.actionLoading"

      :tenant-users="tenantUsers ?? []"

      @close="closeCaseDrawer"

    />



    <ControlTowerOperatorCaseFromWorkItemModal

      :open="caseFromWorkItemOpen"

      :item="caseFromWorkItem"

      @close="caseFromWorkItemOpen = false"

      @created="onCaseFromWorkItemDone"

      @added="onCaseFromWorkItemDone"

    />



    <ControlTowerOperatorHandoffModal

      :open="handoffModalOpen"

      :items="selectedItems"

      :users="tenantUsers ?? []"

      :loading="actionLoading"

      @close="handoffModalOpen = false"

      @confirm="onHandoffConfirm"

    />



    <ControlTowerOperatorHandoffDetailsModal

      :open="handoffDetailsOpen"

      :handoff="selectedHandoff"

      :users="tenantUsers ?? []"

      :format-reason="formatBulkResult"

      @close="closeHandoffDetails"

      @open-work-item="openDetailsByKey"

    />



    <ControlTowerOperatorSavedViewCreateModal

      :open="saveViewModalOpen"

      :loading="actionLoading"

      :can-share="canViewTeamWorkload"

      @close="saveViewModalOpen = false"

      @submit="onSaveViewSubmit"

    />

    </template>



    <ControlTowerOperatorCasesPanel

      v-else-if="workspaceSection === 'cases'"

      :disabled="disabled"

      @open="onOpenCase"

    />

  </UiCard>

</template>



<style scoped>

.operator-workspace__header {

  display: flex;

  justify-content: space-between;

  gap: 1rem;

  margin-bottom: 1rem;

}

.operator-workspace__title {

  margin: 0;

}

.operator-workspace__subtitle {

  margin: 0.25rem 0 0;

  color: var(--color-text-muted, #666);

}

.operator-workspace__kpi {

  display: flex;

  flex-wrap: wrap;

  gap: 1rem;

  margin-bottom: 1rem;

  font-size: 0.875rem;

}

.operator-workspace__mode-tabs {

  display: flex;

  gap: 0.5rem;

  margin-bottom: 1rem;

}

.operator-workspace__mode-tab {

  padding: 0.35rem 0.85rem;

  border: 1px solid var(--color-border, #ddd);

  background: transparent;

  border-radius: var(--radius-sm, 4px);

  cursor: pointer;

}

.operator-workspace__mode-tab--active {

  background: var(--color-primary-soft, #eef3ff);

  border-color: var(--color-primary, #3366ff);

}

.operator-workspace__presets {

  display: flex;

  flex-wrap: wrap;

  gap: 0.5rem;

  margin-bottom: 1rem;

}

.operator-workspace__preset {

  border: 1px solid var(--color-border, #ddd);

  background: transparent;

  border-radius: 999px;

  padding: 0.25rem 0.75rem;

  cursor: pointer;

}

.operator-workspace__preset--active {

  background: var(--color-primary-soft, #eef3ff);

  border-color: var(--color-primary, #3366ff);

}

.operator-workspace__bulk,

.operator-workspace__toolbar {

  display: flex;

  flex-wrap: wrap;

  align-items: center;

  gap: 0.5rem;

  margin-bottom: 1rem;

}

.operator-workspace__table {

  width: 100%;

  border-collapse: collapse;

}

.operator-workspace__table th,

.operator-workspace__table td {

  padding: 0.5rem;

  border-bottom: 1px solid var(--color-border, #eee);

  text-align: left;

}

.operator-workspace__row--selected {

  background: var(--color-primary-soft, #f5f8ff);

}

.operator-workspace__link {

  background: none;

  border: none;

  color: var(--color-primary, #3366ff);

  cursor: pointer;

  text-align: left;

}

.operator-workspace__owner[data-ownership='unassigned'] {

  font-style: italic;

}

.operator-workspace__pagination {

  display: flex;

  align-items: center;

  gap: 0.75rem;

  margin-top: 1rem;

  font-size: 0.875rem;

}

.operator-workspace__empty {

  padding: 1rem 0;

  color: var(--color-text-muted, #666);

}

.operator-workspace__filter-banner {

  display: flex;

  align-items: center;

  gap: 0.5rem;

  font-size: 0.875rem;

  margin-bottom: 1rem;

}

.operator-workspace__handoff-result {

  font-size: 0.875rem;

  margin-bottom: 1rem;

  padding: 0.75rem 1rem;

  border: 1px solid var(--color-border, #ddd);

  border-radius: var(--radius-sm, 4px);

  background: var(--color-surface-muted, #fafafa);

}

.operator-workspace__handoff-failures {

  width: 100%;

  border-collapse: collapse;

  font-size: 0.8125rem;

  margin: 0.5rem 0;

}

.operator-workspace__handoff-failures th,

.operator-workspace__handoff-failures td {

  padding: 0.35rem 0.5rem;

  border-bottom: 1px solid var(--color-border, #eee);

  text-align: left;

}

</style>

