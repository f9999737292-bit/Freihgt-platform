<script setup lang="ts">
import type { V3ScoreExplanation } from '~/types/rfx-score'

defineProps<{
  explanations: V3ScoreExplanation[]
  loading: boolean
}>()

const { t } = useI18n()
</script>

<template>
  <div class="explain-panel" data-testid="v3-explanation-panel">
    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="explanations.length === 0" :title="t('tenders.evaluation.noExplanation')" />
    <table v-else class="explain-table">
      <thead>
        <tr>
          <th>{{ t('tenders.evaluation.explainCriterion') }}</th>
          <th>{{ t('tenders.evaluation.explainSource') }}</th>
          <th>{{ t('tenders.evaluation.explainRaw') }}</th>
          <th>{{ t('tenders.evaluation.explainNormalized') }}</th>
          <th>{{ t('tenders.evaluation.explainWeight') }}</th>
          <th>{{ t('tenders.evaluation.explainContribution') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, idx) in explanations" :key="`${row.criterion_code}-${idx}`" data-testid="v3-explanation-row">
          <td>{{ row.criterion_code }}</td>
          <td>{{ row.source }}</td>
          <td>{{ row.raw_score ?? '—' }}</td>
          <td>{{ row.normalized_score ?? '—' }}</td>
          <td>{{ row.criterion_weight ?? '—' }}</td>
          <td>{{ row.weighted_contribution ?? '—' }}</td>
        </tr>
      </tbody>
    </table>
    <p
      v-for="(row, idx) in explanations.filter((e) => e.knockout)"
      :key="`ko-${idx}`"
      class="knockout-reason"
      data-testid="v3-knockout-reason"
    >
      {{ t('tenders.evaluation.knockoutReason') }}: {{ row.knockout_reason || row.criterion_code }}
    </p>
  </div>
</template>

<style scoped>
.explain-panel { padding: 0.5rem 0; }
.explain-table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
.explain-table th, .explain-table td { border: 1px solid var(--color-border); padding: 0.375rem 0.5rem; text-align: left; }
.knockout-reason { margin-top: 0.75rem; color: var(--color-danger); font-size: 0.875rem; }
</style>
