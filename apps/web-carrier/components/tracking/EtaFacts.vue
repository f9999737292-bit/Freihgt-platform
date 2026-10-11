<script setup lang="ts">
import { displayValue, type ETATarget } from '../../domain/tracking'

defineProps<{
  target: ETATarget
}>()

const { t } = useI18n()
</script>

<template>
  <dl class="facts">
    <div>
      <dt>{{ t('carrier.status') }}</dt>
      <dd data-testid="eta-status">{{ displayValue(target.status) }}</dd>
    </div>
    <div>
      <dt>{{ t('carrier.freshness') }}</dt>
      <dd data-testid="eta-freshness">{{ displayValue(target.freshnessStatus) }}</dd>
    </div>
    <div>
      <dt>{{ t('carrier.quality') }}</dt>
      <dd>{{ displayValue(target.qualityStatus) }}</dd>
    </div>
    <div v-if="target.estimatedArrivalAt">
      <dt>{{ t('carrier.estimatedArrival') }}</dt>
      <dd data-testid="eta-arrival">{{ target.estimatedArrivalAt }}</dd>
    </div>
    <div v-if="target.ageSeconds !== undefined">
      <dt>{{ t('carrier.ageSeconds') }}</dt>
      <dd>{{ displayValue(target.ageSeconds) }}</dd>
    </div>
    <div v-if="target.plannedArrivalAt">
      <dt>{{ t('carrier.plannedArrival') }}</dt>
      <dd>{{ target.plannedArrivalAt }}</dd>
    </div>
    <div v-if="target.projectedDeviationSeconds !== undefined">
      <dt>{{ t('carrier.deviation') }}</dt>
      <dd>{{ displayValue(target.projectedDeviationSeconds) }}</dd>
    </div>
    <div v-if="target.deliveryLagSeconds !== undefined">
      <dt>{{ t('carrier.deliveryDelay') }}</dt>
      <dd>{{ displayValue(target.deliveryLagSeconds) }}</dd>
    </div>
    <div v-if="target.arrivalProjection">
      <dt>{{ t('carrier.arrivalProjection') }}</dt>
      <dd>{{ target.arrivalProjection }}</dd>
    </div>
    <div v-if="target.sourceType">
      <dt>{{ t('carrier.source') }}</dt>
      <dd>{{ target.sourceType }}</dd>
    </div>
  </dl>
</template>
