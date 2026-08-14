<script setup lang="ts">
import type { ControlTowerCasePreset } from '~/types/controlTower'
import { CONTROL_TOWER_CASE_PRESETS } from '~/types/controlTower'

defineProps<{ disabled?: boolean }>()

const emit = defineEmits<{ open: [caseId: string] }>()

const {
  loading,
  cases,
  activePreset,
  page,
  limit,
  total,
  hasNext,
  kpi,
  loadCases,
  loadKpi,
  goToPage,
} = useOperationalCases()

const { t } = useI18n()

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit.value)))

function presetLabel(preset: ControlTowerCasePreset): string {
  return t(`controlTower.cases.presets.${preset}`)
}

async function onPresetChange(preset: ControlTowerCasePreset) {
  await loadCases({ preset, resetPage: true })
}

onMounted(async () => {
  await Promise.all([loadCases({ resetPage: true }), loadKpi()])
})
</script>

<template>
  <section class="cases-panel" aria-labelledby="cases-panel-title">
    <div v-if="kpi" class="cases-panel__kpi">
      <span>{{ $t('controlTower.cases.kpi.open') }}: {{ kpi.openCases }}</span>
      <span>{{ $t('controlTower.cases.kpi.myOpen') }}: {{ kpi.myOpenCases }}</span>
      <span>{{ $t('controlTower.cases.kpi.critical') }}: {{ kpi.criticalCases }}</span>
      <span>{{ $t('controlTower.cases.kpi.unassigned') }}: {{ kpi.unassignedCases }}</span>
    </div>

    <div class="cases-panel__presets">
      <button
        v-for="preset in CONTROL_TOWER_CASE_PRESETS"
        :key="preset"
        type="button"
        class="cases-panel__preset"
        :class="{ 'cases-panel__preset--active': activePreset === preset }"
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
            </td>
            <td>{{ item.ownerDisplayName || $t('controlTower.cases.unassigned') }}</td>
            <td>{{ item.lastActivityAt }}</td>
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
.cases-panel__kpi {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
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
}
.cases-panel__link {
  background: none;
  border: none;
  color: var(--color-primary, #3366ff);
  cursor: pointer;
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
