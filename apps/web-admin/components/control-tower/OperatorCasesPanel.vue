<script setup lang="ts">
import type { ControlTowerCasePreset, ControlTowerOperationalCase, ControlTowerSavedView } from '~/types/controlTower'
import { CONTROL_TOWER_CASE_PRESETS } from '~/types/controlTower'
import { caseSlaDisplayState, formatCaseDateTime } from '~/composables/useCaseDisplay'

defineProps<{ disabled?: boolean }>()

const emit = defineEmits<{ open: [caseId: string] }>()

const {
  loading,
  cases,
  activePreset,
  searchQuery,
  slaFilter,
  page,
  limit,
  total,
  hasNext,
  kpi,
  savedViews,
  loadCases,
  loadKpi,
  loadSavedViews,
  goToPage,
  createSavedView,
  updateSavedViewWithCurrentFilters,
  renameSavedView,
  duplicateSavedView,
  deleteSavedView,
  setDefaultSavedView,
  applySavedView,
} = useOperationalCases()

const { t } = useI18n()

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))

function presetLabel(preset: ControlTowerCasePreset): string {
  return t(`controlTower.cases.presets.${preset}`)
}

async function onPresetChange(preset: ControlTowerCasePreset) {
  slaFilter.value = 'none'
  await loadCases({ preset, resetPage: true })
}

async function onSearch() {
  await loadCases({ resetPage: true })
}

async function onSlaFilterChange(value: 'none' | 'breached' | 'warning' | 'at_risk') {
  slaFilter.value = value
  if (value === 'at_risk') {
    await loadCases({ preset: 'sla_at_risk', resetPage: true })
  } else if (value === 'breached') {
    await loadCases({ preset: 'all_active', resetPage: true })
  } else if (value === 'warning') {
    await loadCases({ preset: 'all_active', resetPage: true })
  } else {
    await loadCases({ resetPage: true })
  }
}

function slaBadgeLabel(item: ControlTowerOperationalCase): string {
  const state = caseSlaDisplayState(item.health)
  return t(`controlTower.cases.slaStates.${state}`)
}

onMounted(async () => {
  await Promise.all([loadCases({ resetPage: true }), loadKpi(), loadSavedViews()])
})
</script>

<template>
  <section class="cases-panel" aria-labelledby="cases-panel-title">
    <h2 id="cases-panel-title" class="visually-hidden">{{ $t('controlTower.cases.title') }}</h2>

    <div v-if="kpi" class="cases-panel__kpi" role="list">
      <span role="listitem">{{ $t('controlTower.cases.kpi.open') }}: {{ kpi.openCases }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.myOpen') }}: {{ kpi.myOpenCases }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.critical') }}: {{ kpi.criticalCases }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.unassigned') }}: {{ kpi.unassignedCases }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.slaBreach') }}: {{ kpi.casesWithSlaBreach ?? 0 }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.slaWarning') }}: {{ kpi.casesWithSlaWarning ?? 0 }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.slaAtRisk') }}: {{ kpi.slaAtRiskCases ?? 0 }}</span>
      <span role="listitem">{{ $t('controlTower.cases.kpi.overdueActions') }}: {{ kpi.casesWithOverdueActions ?? 0 }}</span>
    </div>

    <ControlTowerOperatorSavedViewsPanel
      :views="savedViews"
      :active-preset="activePreset"
      @apply="applySavedView($event as ControlTowerSavedView)"
      @rename="renameSavedView($event.id, $event.name)"
      @update="updateSavedViewWithCurrentFilters($event.id)"
      @duplicate="duplicateSavedView($event)"
      @delete="deleteSavedView($event.id)"
      @set-default="setDefaultSavedView($event.id)"
      @create="createSavedView({ name: $t('controlTower.cases.savedCaseViewDefaultName'), scope: 'private' })"
    />

    <div class="cases-panel__toolbar">
      <input
        v-model="searchQuery"
        type="search"
        class="cases-panel__search"
        :placeholder="$t('controlTower.cases.searchPlaceholder')"
        :aria-label="$t('controlTower.cases.searchPlaceholder')"
        @keydown.enter="onSearch"
      />
      <UiButton size="sm" variant="secondary" :disabled="loading" @click="onSearch">
        {{ $t('common.search') }}
      </UiButton>
      <label class="cases-panel__filter">
        <span>{{ $t('controlTower.cases.slaFilter') }}</span>
        <select :value="slaFilter" :disabled="loading" @change="onSlaFilterChange(($event.target as HTMLSelectElement).value as any)">
          <option value="none">{{ $t('controlTower.cases.slaStates.normal') }}</option>
          <option value="breached">{{ $t('controlTower.cases.slaStates.breached') }}</option>
          <option value="warning">{{ $t('controlTower.cases.slaStates.warning') }}</option>
          <option value="at_risk">{{ $t('controlTower.cases.presets.sla_at_risk') }}</option>
        </select>
      </label>
    </div>

    <div class="cases-panel__presets" role="tablist" :aria-label="$t('controlTower.cases.presetsLabel')">
      <button
        v-for="preset in CONTROL_TOWER_CASE_PRESETS"
        :key="preset"
        type="button"
        role="tab"
        class="cases-panel__preset"
        :class="{ 'cases-panel__preset--active': activePreset === preset }"
        :aria-selected="activePreset === preset"
        :disabled="disabled || loading"
        @click="onPresetChange(preset)"
      >
        {{ presetLabel(preset) }}
      </button>
    </div>

    <div v-if="loading" class="cases-panel__empty">{{ $t('common.loading') }}</div>
    <div v-else-if="cases.length === 0" class="cases-panel__empty">
      {{ $t(`controlTower.cases.emptyStates.${activePreset}`) }}
    </div>
    <div v-else class="cases-panel__table-wrap">
      <table class="cases-panel__table">
        <thead>
          <tr>
            <th scope="col">{{ $t('controlTower.cases.reference') }}</th>
            <th scope="col">{{ $t('controlTower.cases.titleColumn') }}</th>
            <th scope="col">{{ $t('controlTower.cases.status') }}</th>
            <th scope="col">{{ $t('controlTower.cases.severity') }}</th>
            <th scope="col">{{ $t('controlTower.cases.health.sla') }}</th>
            <th scope="col">{{ $t('controlTower.cases.health.actions') }}</th>
            <th scope="col">{{ $t('controlTower.cases.caseOwner') }}</th>
            <th scope="col">{{ $t('controlTower.cases.lastActivity') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in cases" :key="item.id">
            <td>
              <button type="button" class="cases-panel__link" @click="emit('open', item.id)">
                {{ item.reference }}
              </button>
            </td>
            <td>{{ item.title }}</td>
            <td>
              <span class="cases-panel__status" :data-status="item.status">
                {{ $t(`controlTower.cases.statuses.${item.status}`) }}
              </span>
            </td>
            <td>
              <span class="cases-panel__severity" :data-severity="item.effectiveSeverity">
                {{ $t(`controlTower.cases.severities.${item.effectiveSeverity}`) }}
              </span>
              <span v-if="item.severityOverride" class="cases-panel__badge">
                {{ $t('controlTower.cases.manualOverrideShort') }}
              </span>
            </td>
            <td>
              <span class="cases-panel__badge" :data-sla="caseSlaDisplayState(item.health)">
                {{ slaBadgeLabel(item) }}
              </span>
            </td>
            <td>
              <span class="cases-panel__meta">
                {{ $t('controlTower.cases.health.openShort', { count: item.health?.openActionCount ?? 0 }) }}
                <template v-if="(item.health?.overdueActionCount ?? 0) > 0">
                  · {{ $t('controlTower.cases.health.overdueShort', { count: item.health?.overdueActionCount ?? 0 }) }}
                </template>
                <template v-if="item.health?.activeWorkItemCount">
                  · {{ $t('controlTower.cases.health.activeWorkShort', { count: item.health.activeWorkItemCount }) }}
                </template>
              </span>
            </td>
            <td>{{ item.ownerDisplayName || $t('controlTower.cases.unassigned') }}</td>
            <td>{{ formatCaseDateTime(item.lastActivityAt) }}</td>
          </tr>
        </tbody>
      </table>

      <nav class="cases-panel__pagination" :aria-label="$t('controlTower.cases.pagination')">
        <UiButton size="sm" variant="ghost" :disabled="page <= 1" @click="goToPage(page - 1)">
          {{ $t('common.previous') }}
        </UiButton>
        <span>{{ $t('controlTower.workspace.pageOf', { page, total: totalPages }) }}</span>
        <UiButton size="sm" variant="ghost" :disabled="!hasNext" @click="goToPage(page + 1)">
          {{ $t('common.next') }}
        </UiButton>
      </nav>
    </div>
  </section>
</template>

<style scoped>
.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
.cases-panel__kpi {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem 1rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}
.cases-panel__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: end;
  margin-bottom: 1rem;
}
.cases-panel__search {
  min-width: 220px;
  padding: 0.45rem 0.6rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: var(--radius-sm, 4px);
}
.cases-panel__filter {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.8125rem;
}
.cases-panel__presets {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 1rem;
}
.cases-panel__preset {
  border: 1px solid var(--color-border, #ddd);
  background: transparent;
  border-radius: 999px;
  padding: 0.25rem 0.75rem;
  cursor: pointer;
}
.cases-panel__preset--active {
  background: var(--color-primary-soft, #eef3ff);
  border-color: var(--color-primary, #3366ff);
}
.cases-panel__table {
  width: 100%;
  border-collapse: collapse;
}
.cases-panel__table th,
.cases-panel__table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border, #eee);
  text-align: left;
  vertical-align: top;
}
.cases-panel__link {
  background: none;
  border: none;
  color: var(--color-primary, #3366ff);
  cursor: pointer;
}
.cases-panel__badge {
  display: inline-block;
  font-size: 0.75rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--color-border, #ddd);
}
.cases-panel__badge[data-sla='breached'] {
  border-color: #c62828;
}
.cases-panel__badge[data-sla='warning'] {
  border-color: #ef6c00;
}
.cases-panel__meta {
  font-size: 0.8125rem;
}
.cases-panel__empty {
  padding: 1rem 0;
  color: var(--color-text-muted, #666);
}
.cases-panel__pagination {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-top: 1rem;
  font-size: 0.875rem;
}
</style>
