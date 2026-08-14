<script setup lang="ts">
import type { ControlTowerBulkActionOutcome, ControlTowerBulkActionType } from '~/types/controlTower'

const props = defineProps<{
  outcome: ControlTowerBulkActionOutcome | null
  action: ControlTowerBulkActionType | null
  formatReason: (error?: string) => string
}>()

const emit = defineEmits<{
  retry: []
  dismiss: []
}>()

const show = computed(() => props.outcome != null && props.outcome.requested > 0)
</script>

<template>
  <section v-if="show && outcome" class="bulk-result" aria-live="polite">
    <h4 class="bulk-result__title">{{ $t('controlTower.workspace.bulkAction') }}</h4>
    <p>
      {{ $t('controlTower.workspace.bulkSummary', {
        requested: outcome.requested,
        succeeded: outcome.succeeded,
        failed: outcome.failed,
      }) }}
    </p>
    <table v-if="outcome.failed > 0" class="bulk-result__table">
      <thead>
        <tr>
          <th scope="col">{{ $t('controlTower.workspace.type') }}</th>
          <th scope="col">{{ $t('controlTower.workspace.reference') }}</th>
          <th scope="col">{{ $t('controlTower.workspace.reason') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in outcome.results.filter((r) => !r.success)" :key="`${row.itemType}:${row.itemId}`">
          <td>{{ row.itemType }}</td>
          <td>{{ row.itemId }}</td>
          <td>{{ formatReason(row.error) }}</td>
        </tr>
      </tbody>
    </table>
    <div class="bulk-result__actions">
      <UiButton
        v-if="outcome.failed > 0 && action"
        size="sm"
        variant="secondary"
        @click="emit('retry')"
      >
        {{ $t('controlTower.workspace.retryFailed') }}
      </UiButton>
      <UiButton size="sm" variant="ghost" @click="emit('dismiss')">
        {{ $t('common.close') }}
      </UiButton>
    </div>
  </section>
</template>

<style scoped>
.bulk-result {
  margin: 1rem 0;
  padding: 0.75rem 1rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: var(--radius-sm, 4px);
  background: var(--color-surface-muted, #fafafa);
}
.bulk-result__title {
  margin: 0 0 0.5rem;
  font-size: 0.9375rem;
}
.bulk-result__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
  margin: 0.5rem 0;
}
.bulk-result__table th,
.bulk-result__table td {
  padding: 0.35rem 0.5rem;
  border-bottom: 1px solid var(--color-border, #eee);
  text-align: left;
}
.bulk-result__actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
</style>
