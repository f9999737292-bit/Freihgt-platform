<script setup lang="ts">
import { Table } from '@freight-platform/ui'
import { displayValue, type ETAHistoryItem } from '../../domain/tracking'

defineProps<{
  title: string
  items: ETAHistoryItem[]
  empty: string
  testId: string
}>()

const { t } = useI18n()
</script>

<template>
  <section :data-testid="testId">
    <h4>{{ title }}</h4>
    <p v-if="items.length === 0">{{ empty }}</p>
    <Table
      v-else
      :columns="[
        t('shipper.estimatedArrival'),
        t('shipper.observedAt'),
        t('shipper.receivedAt'),
        t('shipper.quality'),
        t('shipper.source'),
      ]"
    >
      <tr v-for="(item, index) in items" :key="index">
        <td>{{ displayValue(item.estimatedArrivalAt) }}</td>
        <td>{{ displayValue(item.sourceObservedAt) }}</td>
        <td>{{ displayValue(item.receivedAt) }}</td>
        <td>{{ displayValue(item.qualityStatus) }}</td>
        <td>{{ displayValue(item.sourceType) }}</td>
      </tr>
    </Table>
  </section>
</template>
