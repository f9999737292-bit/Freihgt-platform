<script setup lang="ts">
import type { ControlTowerActivityItem } from '~/types/controlTower'

defineProps<{
  items: ControlTowerActivityItem[]
  loading?: boolean
}>()

function formatDate(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
</script>

<template>
  <UiTable
    v-if="items.length || loading"
    :columns="[
      $t('controlTower.recentActivity.time'),
      $t('controlTower.recentActivity.type'),
      $t('controlTower.recentActivity.title'),
      $t('common.status'),
      $t('common.actions'),
    ]"
    :loading="loading"
  >
    <tr v-for="item in items" :key="item.id">
      <td>{{ formatDate(item.timestamp) }}</td>
      <td>{{ $t(item.typeKey) }}</td>
      <td>{{ item.title }}</td>
      <td><UiBadge :status="item.status" /></td>
      <td><NuxtLink :to="item.link">{{ $t('common.details') }}</NuxtLink></td>
    </tr>
  </UiTable>
  <UiEmptyState v-else :title="$t('controlTower.recentActivity.empty')" />
</template>
