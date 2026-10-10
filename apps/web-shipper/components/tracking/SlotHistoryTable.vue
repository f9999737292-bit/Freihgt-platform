<script setup lang="ts">
import { Table } from '@freight-platform/ui'
import { displayValue, type SlotHistoryItem } from '../../domain/tracking'

defineProps<{
  title: string
  items: SlotHistoryItem[]
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
        t('shipper.windowStart'),
        t('shipper.windowEnd'),
        t('shipper.timezone'),
        t('shipper.slotStatus'),
        t('shipper.quality'),
      ]"
    >
      <tr v-for="(item, index) in items" :key="index">
        <td>{{ displayValue(item.windowStart) }}</td>
        <td>{{ displayValue(item.windowEnd) }}</td>
        <td>{{ displayValue(item.timezone) }}</td>
        <td>{{ displayValue(item.slotStatus) }}</td>
        <td>{{ displayValue(item.qualityStatus) }}</td>
      </tr>
    </Table>
  </section>
</template>
