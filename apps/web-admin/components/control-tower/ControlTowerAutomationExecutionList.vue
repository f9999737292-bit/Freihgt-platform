<script setup lang="ts">
import type { PlaybookExecution } from '~/types/automation'
import { formatLowCodeDate } from '~/types/lowCode'

defineProps<{
  executions: PlaybookExecution[]
  loading?: boolean
  loadFailed?: boolean
}>()

const emit = defineEmits<{ refresh: [] }>()

const { t } = useI18n()

function executionStatusLabel(status: string): string {
  if (status === 'completed') return t('controlTower.automation.completed')
  if (status === 'cancelled') return t('controlTower.automation.cancelled')
  return t(`controlTower.automation.executionStatuses.${status}`, status)
}
</script>

<template>
  <div class="execution-list">
    <div v-if="loadFailed" class="execution-list__error">
      <p>{{ $t('common.apiUnavailable') }}</p>
      <p class="execution-list__hint">{{ $t('common.apiUnavailableHint') }}</p>
      <UiButton variant="secondary" size="sm" @click="emit('refresh')">
        {{ $t('common.refresh') }}
      </UiButton>
    </div>

    <UiTable
      v-else
      :loading="loading"
      :columns="[
        $t('controlTower.automation.playbook'),
        $t('controlTower.automation.status'),
        $t('controlTower.automation.executionProgress', 'Progress'),
        $t('controlTower.automation.executionsCreatedAt', 'Created'),
        $t('common.actions'),
      ]"
    >
      <tr v-if="!loading && !executions.length">
        <td colspan="5">
          <UiEmptyState
            :title="$t('controlTower.automation.noExecutions', 'No executions yet')"
          />
        </td>
      </tr>
      <template v-else-if="!loading">
        <tr v-for="execution in executions" :key="execution.id">
          <td>{{ execution.playbookName || execution.playbookId }}</td>
          <td>{{ executionStatusLabel(execution.status) }}</td>
          <td>{{ $t('controlTower.automation.playbookProgress', { done: execution.progressDone, total: execution.progressTotal }) }}</td>
          <td>{{ formatLowCodeDate(execution.createdAt) }}</td>
          <td>
            <NuxtLink :to="`/control-tower?execution=${execution.id}`">
              {{ $t('common.details') }}
            </NuxtLink>
          </td>
        </tr>
      </template>
    </UiTable>
  </div>
</template>

<style scoped>
.execution-list__error {
  padding: 1.5rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.execution-list__hint {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
