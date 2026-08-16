<script setup lang="ts">
import type { ControlTowerOperatorWorkload } from '~/types/controlTower'

defineProps<{
  operators: ControlTowerOperatorWorkload[]
  unassignedPool: number
  loading?: boolean
  activeFilterTarget?: string | null
}>()

const emit = defineEmits<{
  viewQueue: [target: string | null]
}>()
</script>

<template>
  <section class="team-workload" aria-labelledby="team-workload-title">
    <h3 id="team-workload-title" class="team-workload__title">
      {{ $t('controlTower.workspace.teamWorkload') }}
    </h3>
    <p v-if="loading" class="team-workload__empty">{{ $t('common.loading') }}</p>
    <div v-else class="team-workload__scroll">
      <table class="team-workload__table">
        <thead>
          <tr>
            <th scope="col">{{ $t('controlTower.workspace.operator') }}</th>
            <th scope="col">{{ $t('controlTower.workspace.activeCount') }}</th>
            <th scope="col">{{ $t('controlTower.workspace.presets.critical') }}</th>
            <th scope="col">P1</th>
            <th scope="col">P2</th>
            <th scope="col">{{ $t('controlTower.workspace.slaBreached') }}</th>
            <th scope="col">{{ $t('controlTower.workspace.slaWarning') }}</th>
            <th scope="col">{{ $t('controlTower.workspace.riskCount') }}</th>
            <th scope="col" />
          </tr>
        </thead>
        <tbody>
          <tr
            class="team-workload__unassigned"
            :class="{ 'team-workload__row--active': activeFilterTarget === 'unassigned' }"
          >
            <td>{{ $t('controlTower.workspace.unassignedPool') }}</td>
            <td colspan="7">{{ unassignedPool }}</td>
            <td>
              <UiButton size="sm" variant="ghost" @click="emit('viewQueue', 'unassigned')">
                {{ $t('controlTower.workspace.viewQueue') }}
              </UiButton>
            </td>
          </tr>
          <tr
            v-for="op in operators"
            :key="op.userId"
            :class="{ 'team-workload__row--active': activeFilterTarget === op.userId }"
          >
            <td>{{ op.displayName || op.userId }}</td>
            <td>{{ op.activeWorkItems }}</td>
            <td>{{ op.criticalWork ?? 0 }}</td>
            <td>{{ op.p1 }}</td>
            <td>{{ op.p2 }}</td>
            <td>{{ op.slaBreached }}</td>
            <td>{{ op.slaWarning }}</td>
            <td>{{ op.criticalRisks + op.highRisks }}</td>
            <td>
              <UiButton size="sm" variant="ghost" @click="emit('viewQueue', op.userId)">
                {{ $t('controlTower.workspace.viewQueue') }}
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.team-workload__title {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.team-workload__scroll {
  overflow-x: auto;
}
.team-workload__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}
.team-workload__table th,
.team-workload__table td {
  padding: 0.4rem 0.5rem;
  border-bottom: 1px solid var(--color-border, #eee);
  text-align: left;
  white-space: nowrap;
}
.team-workload__row--active {
  background: var(--color-primary-soft, #f5f8ff);
}
.team-workload__unassigned td:first-child {
  font-weight: 600;
}
</style>
