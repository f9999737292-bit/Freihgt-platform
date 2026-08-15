<script setup lang="ts">
import type { CarrierScoreResult } from '~/types/tender'

defineProps<{
  score?: CarrierScoreResult
  qualification?: { result: string; reasons: string[] }
}>()
</script>

<template>
  <div v-if="score || qualification" class="score-breakdown">
    <div v-if="qualification" class="score-breakdown__qual">
      <strong>{{ qualification.result }}</strong>
      <span v-if="qualification.reasons.length">
        — {{ qualification.reasons.join('; ') }}
      </span>
    </div>
    <dl v-if="score" class="score-breakdown__grid">
      <div><dt>{{ $t('tender.totalScore') }}</dt><dd>{{ score.total_score.toFixed(2) }}</dd></div>
      <div><dt>{{ $t('tender.price') }}</dt><dd>{{ score.price_score.toFixed(2) }}</dd></div>
      <div><dt>{{ $t('tender.sla') }}</dt><dd>{{ score.sla_score.toFixed(2) }}</dd></div>
      <div><dt>{{ $t('tender.kpi') }}</dt><dd>{{ score.carrier_kpi_score.toFixed(2) }}</dd></div>
      <div><dt>{{ $t('tender.capacity') }}</dt><dd>{{ score.capacity_score.toFixed(2) }}</dd></div>
      <div><dt>{{ $t('tender.reliability') }}</dt><dd>{{ score.reliability_score.toFixed(2) }}</dd></div>
      <div><dt>{{ $t('tender.transitTime') }}</dt><dd>{{ score.transit_time_score.toFixed(2) }}</dd></div>
    </dl>
    <table v-if="score?.contributions?.length" class="score-breakdown__table">
      <thead>
        <tr>
          <th>{{ $t('tender.factor') }}</th>
          <th>{{ $t('tender.weight') }}</th>
          <th>{{ $t('tender.rawScore') }}</th>
          <th>{{ $t('tender.contribution') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in score.contributions" :key="row.factor">
          <td>{{ row.factor }}</td>
          <td>{{ row.weight }}%</td>
          <td>{{ row.raw_score.toFixed(2) }}</td>
          <td>{{ row.contribution.toFixed(2) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.score-breakdown {
  padding: 0.75rem;
  background: var(--color-surface-muted, #f8fafc);
  border-radius: var(--radius-sm);
}

.score-breakdown__qual {
  margin-bottom: 0.75rem;
}

.score-breakdown__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.5rem;
  margin: 0 0 0.75rem;
}

.score-breakdown__grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.score-breakdown__grid dd {
  margin: 0.1rem 0 0;
}

.score-breakdown__table {
  width: 100%;
  border-collapse: collapse;
}

.score-breakdown__table th,
.score-breakdown__table td {
  padding: 0.35rem 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
}
</style>
