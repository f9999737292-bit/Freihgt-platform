<script setup lang="ts">
import { displayValue, type SlotTarget } from '../../domain/tracking'

defineProps<{
  target: SlotTarget
}>()

const { t } = useI18n()
</script>

<template>
  <dl class="facts">
    <div>
      <dt>{{ t('carrier.windowStatus') }}</dt>
      <dd data-testid="window-status">{{ displayValue(target.windowStatus) }}</dd>
    </div>
    <div v-if="target.slotStatus">
      <dt>{{ t('carrier.slotStatus') }}</dt>
      <dd>{{ target.slotStatus }}</dd>
    </div>
    <div v-if="target.windowStart">
      <dt>{{ t('carrier.windowStart') }}</dt>
      <dd data-testid="window-start">{{ target.windowStart }}</dd>
    </div>
    <div v-if="target.windowEnd">
      <dt>{{ t('carrier.windowEnd') }}</dt>
      <dd data-testid="window-end">{{ target.windowEnd }}</dd>
    </div>
    <div v-if="target.timezone">
      <dt>{{ t('carrier.timezone') }}</dt>
      <dd data-testid="window-timezone">{{ target.timezone }}</dd>
    </div>
    <div>
      <dt>{{ t('carrier.quality') }}</dt>
      <dd>{{ displayValue(target.qualityStatus) }}</dd>
    </div>
    <div v-if="target.arrivalProjection">
      <dt>{{ t('carrier.arrivalProjection') }}</dt>
      <dd>{{ target.arrivalProjection }}</dd>
    </div>
    <div v-if="target.etaRelation">
      <dt>{{ t('carrier.etaRelation') }}</dt>
      <dd>{{ target.etaRelation }}</dd>
    </div>
    <div v-if="target.sourceType">
      <dt>{{ t('carrier.source') }}</dt>
      <dd>{{ target.sourceType }}</dd>
    </div>
    <div v-if="target.bookedAt">
      <dt>{{ t('carrier.bookedAt') }}</dt>
      <dd>{{ target.bookedAt }}</dd>
    </div>
    <div v-if="target.confirmedAt">
      <dt>{{ t('carrier.confirmedAt') }}</dt>
      <dd>{{ target.confirmedAt }}</dd>
    </div>
  </dl>
</template>
