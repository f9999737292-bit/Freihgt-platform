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
      <dt>{{ t('shipper.windowStatus') }}</dt>
      <dd data-testid="window-status">{{ displayValue(target.windowStatus) }}</dd>
    </div>
    <div v-if="target.slotStatus">
      <dt>{{ t('shipper.slotStatus') }}</dt>
      <dd>{{ target.slotStatus }}</dd>
    </div>
    <div v-if="target.windowStart">
      <dt>{{ t('shipper.windowStart') }}</dt>
      <dd data-testid="window-start">{{ target.windowStart }}</dd>
    </div>
    <div v-if="target.windowEnd">
      <dt>{{ t('shipper.windowEnd') }}</dt>
      <dd data-testid="window-end">{{ target.windowEnd }}</dd>
    </div>
    <div v-if="target.timezone">
      <dt>{{ t('shipper.timezone') }}</dt>
      <dd data-testid="window-timezone">{{ target.timezone }}</dd>
    </div>
    <div>
      <dt>{{ t('shipper.quality') }}</dt>
      <dd>{{ displayValue(target.qualityStatus) }}</dd>
    </div>
    <div v-if="target.arrivalProjection">
      <dt>{{ t('shipper.arrivalProjection') }}</dt>
      <dd>{{ target.arrivalProjection }}</dd>
    </div>
    <div v-if="target.etaRelation">
      <dt>{{ t('shipper.etaRelation') }}</dt>
      <dd>{{ target.etaRelation }}</dd>
    </div>
    <div v-if="target.sourceType">
      <dt>{{ t('shipper.source') }}</dt>
      <dd>{{ target.sourceType }}</dd>
    </div>
    <div v-if="target.bookedAt">
      <dt>{{ t('shipper.bookedAt') }}</dt>
      <dd>{{ target.bookedAt }}</dd>
    </div>
    <div v-if="target.confirmedAt">
      <dt>{{ t('shipper.confirmedAt') }}</dt>
      <dd>{{ target.confirmedAt }}</dd>
    </div>
  </dl>
</template>
