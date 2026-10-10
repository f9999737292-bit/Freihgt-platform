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
      <dt>{{ t('shipper.status') }}</dt>
      <dd data-testid="eta-status">{{ displayValue(target.status) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.freshness') }}</dt>
      <dd data-testid="eta-freshness">{{ displayValue(target.freshnessStatus) }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.quality') }}</dt>
      <dd>{{ displayValue(target.qualityStatus) }}</dd>
    </div>
    <div v-if="target.estimatedArrivalAt">
      <dt>{{ t('shipper.estimatedArrival') }}</dt>
      <dd data-testid="eta-arrival">{{ target.estimatedArrivalAt }}</dd>
    </div>
    <div v-if="target.ageSeconds !== undefined">
      <dt>{{ t('shipper.ageSeconds') }}</dt>
      <dd>{{ displayValue(target.ageSeconds) }}</dd>
    </div>
    <div v-if="target.plannedArrivalAt">
      <dt>{{ t('shipper.plannedArrival') }}</dt>
      <dd>{{ target.plannedArrivalAt }}</dd>
    </div>
    <div v-if="target.projectedDeviationSeconds !== undefined">
      <dt>{{ t('shipper.deviation') }}</dt>
      <dd>{{ displayValue(target.projectedDeviationSeconds) }}</dd>
    </div>
    <div v-if="target.deliveryLagSeconds !== undefined">
      <dt>{{ t('shipper.deliveryDelay') }}</dt>
      <dd>{{ displayValue(target.deliveryLagSeconds) }}</dd>
    </div>
    <div v-if="target.arrivalProjection">
      <dt>{{ t('shipper.arrivalProjection') }}</dt>
      <dd>{{ target.arrivalProjection }}</dd>
    </div>
    <div v-if="target.sourceType">
      <dt>{{ t('shipper.source') }}</dt>
      <dd>{{ target.sourceType }}</dd>
    </div>
  </dl>
</template>
