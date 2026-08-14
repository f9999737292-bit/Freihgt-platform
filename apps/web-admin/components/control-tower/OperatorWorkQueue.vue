<script setup lang="ts">
import type { User } from '~/types/user'
import type { ControlTowerWorkItem, ControlTowerWorkspacePreset } from '~/types/controlTower'
import { CONTROL_TOWER_WORKSPACE_PRESETS } from '~/types/controlTower'

const props = defineProps<{
  disabled?: boolean
  tenantUsers?: User[]
}>()

const { t } = useI18n()
const {
  loading,
  actionLoading,
  workItems,
  selectedIds,
  selectedCount,
  activePreset,
  total,
  kpi,
  savedViews,
  selectedItem,
  drawerOpen,
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
} = useOperatorWorkspace()

const assignUserId = ref('')
const saveViewName = ref('')
const showSaveView = ref(false)

const presetOptions = CONTROL_TOWER_WORKSPACE_PRESETS.filter((p) => p !== 'completed')

function presetLabel(preset: ControlTowerWorkspacePreset): string {
  return t(`controlTower.workspace.presets.${preset}`)
}

function itemTypeLabel(item: ControlTowerWorkItem): string {
  return item.itemType === 'risk'
    ? t('controlTower.workspace.predictiveRisk')
    : t('controlTower.workspace.actualException')
}

function urgencyLabel(urgency: string): string {
  return t(`controlTower.workspace.urgencyLevels.${urgency}`)
}

function ownerLabel(item: ControlTowerWorkItem): string {
  if (item.ownerDisplayName) return item.ownerDisplayName
  if (item.ownerUserId) return item.ownerUserId
  return t('controlTower.workspace.unassigned')
}

async function onPresetChange(preset: ControlTowerWorkspacePreset) {
  await loadWorkItems(preset)
}

async function onRefresh() {
  await loadWorkItems()
}

async function onSaveView() {
  if (!saveViewName.value.trim()) return
  await saveCurrentView(saveViewName.value.trim())
  saveViewName.value = ''
  showSaveView.value = false
}

onMounted(async () => {
  if (!props.disabled) {
    await Promise.all([loadWorkItems('my_work'), loadSavedViews()])
  }
})

watch(
  () => props.disabled,
  (disabled) => {
    if (!disabled && workItems.value.length === 0) {
      void loadWorkItems('my_work')
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

    <div v-if="kpi" class="operator-workspace__kpi">
      <span>{{ $t('controlTower.workspace.myActiveWork') }}: {{ kpi.myActiveWork }}</span>
      <span>{{ $t('controlTower.workspace.unassignedWork') }}: {{ kpi.unassignedWork }}</span>
      <span>{{ $t('controlTower.workspace.slaBreached') }}: {{ kpi.slaBreachedWork }}</span>
      <span>{{ $t('controlTower.workspace.criticalRiskWork') }}: {{ kpi.criticalRiskWork }}</span>
    </div>

    <div class="operator-workspace__presets">
      <button
        v-for="preset in presetOptions"
        :key="preset"
        type="button"
        class="operator-workspace__preset"
        :class="{ 'operator-workspace__preset--active': activePreset === preset }"
        :disabled="disabled || loading"
        @click="onPresetChange(preset)"
      >
        {{ presetLabel(preset) }}
      </button>
    </div>

    <div v-if="savedViews.length" class="operator-workspace__saved-views">
      <span class="operator-workspace__saved-label">{{ $t('controlTower.workspace.savedViews') }}:</span>
      <button
        v-for="view in savedViews"
        :key="view.id"
        type="button"
        class="operator-workspace__saved-chip"
        @click="view.filters?.preset && onPresetChange(view.filters.preset as ControlTowerWorkspacePreset)"
      >
        {{ view.name }}
        <small v-if="view.isDefault">({{ $t('controlTower.workspace.defaultView') }})</small>
      </button>
    </div>

    <div v-if="selectedCount > 0" class="operator-workspace__bulk">
      <span>{{ $t('controlTower.workspace.selected', { count: selectedCount }) }}</span>
      <UiButton size="sm" :disabled="actionLoading" @click="claimSelected">
        {{ $t('controlTower.workspace.claim') }}
      </UiButton>
      <select v-model="assignUserId" class="operator-workspace__select" :aria-label="$t('controlTower.workspace.assign')">
        <option value="">{{ $t('controlTower.workspace.assign') }}</option>
        <option v-for="user in tenantUsers ?? []" :key="user.id" :value="user.id">
          {{ user.full_name }}
        </option>
      </select>
      <UiButton
        size="sm"
        variant="secondary"
        :disabled="!assignUserId || actionLoading"
        @click="assignSelected(assignUserId)"
      >
        {{ $t('controlTower.workspace.reassign') }}
      </UiButton>
      <UiButton size="sm" variant="ghost" @click="clearSelection">{{ $t('common.cancel') }}</UiButton>
    </div>

    <div class="operator-workspace__toolbar">
      <UiButton size="sm" variant="ghost" :disabled="loading" @click="selectAllVisible">
        {{ $t('controlTower.workspace.selectAll') }}
      </UiButton>
      <UiButton size="sm" variant="ghost" @click="showSaveView = !showSaveView">
        {{ $t('controlTower.workspace.saveView') }}
      </UiButton>
      <div v-if="showSaveView" class="operator-workspace__save-form">
        <input v-model="saveViewName" type="text" :placeholder="$t('controlTower.workspace.viewName')" />
        <UiButton size="sm" @click="onSaveView">{{ $t('common.save') }}</UiButton>
      </div>
    </div>

    <div v-if="loading" class="operator-workspace__empty">{{ $t('common.loading') }}</div>
    <div v-else-if="workItems.length === 0" class="operator-workspace__empty">
      {{ $t('controlTower.workspace.empty') }}
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
                {{ urgencyLabel(item.urgency) }}
              </span>
            </td>
            <td>{{ ownerLabel(item) }}</td>
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
      <p class="operator-workspace__count">{{ $t('controlTower.workspace.total', { total }) }}</p>
    </div>

    <aside v-if="drawerOpen && selectedItem" class="operator-workspace__drawer" role="complementary">
      <header class="operator-workspace__drawer-header">
        <h3>{{ selectedItem.title }}</h3>
        <button type="button" :aria-label="$t('common.close')" @click="drawerOpen = false">×</button>
      </header>
      <dl class="operator-workspace__drawer-meta">
        <dt>{{ $t('controlTower.workspace.type') }}</dt>
        <dd>{{ itemTypeLabel(selectedItem) }}</dd>
        <dt>{{ $t('controlTower.workspace.shipment') }}</dt>
        <dd>{{ selectedItem.shipmentNumber || selectedItem.shipmentId }}</dd>
        <dt>{{ $t('controlTower.workspace.owner') }}</dt>
        <dd>{{ ownerLabel(selectedItem) }}</dd>
        <dt>{{ $t('controlTower.workspace.urgencyColumn') }}</dt>
        <dd>{{ urgencyLabel(selectedItem.urgency) }}</dd>
      </dl>
      <section v-if="selectedItem.timeline?.length">
        <h4>{{ $t('controlTower.workspace.timeline') }}</h4>
        <ul>
          <li v-for="(entry, idx) in selectedItem.timeline" :key="idx">
            {{ entry.actionType }} — {{ entry.occurredAt }}
          </li>
        </ul>
      </section>
    </aside>
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
.operator-workspace__drawer {
  position: fixed;
  top: 0;
  right: 0;
  width: min(420px, 100vw);
  height: 100vh;
  background: var(--color-surface, #fff);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.08);
  padding: 1rem;
  z-index: 40;
  overflow: auto;
}
.operator-workspace__empty {
  padding: 1rem 0;
  color: var(--color-text-muted, #666);
}
</style>
