<script setup lang="ts">
import type { ControlTowerOperationRow } from '~/types/controlTower'

defineProps<{
  rows: ControlTowerOperationRow[]
  loading?: boolean
}>()

const { t } = useI18n()

function statusLabel(status: ControlTowerOperationRow['status']): string {
  if (status === 'ok') return t('controlTower.status.ok')
  if (status === 'warning') return t('controlTower.status.warning')
  return t('controlTower.status.down')
}

function statusTone(status: ControlTowerOperationRow['status']) {
  if (status === 'ok') return 'ok'
  if (status === 'warning') return 'warning'
  return 'down'
}
</script>

<template>
  <UiTable
    :columns="[
      $t('controlTower.operations.area'),
      $t('common.status'),
      $t('controlTower.operations.count'),
      $t('common.actions'),
    ]"
    :loading="loading"
  >
    <tr v-for="row in rows" :key="row.key">
      <td>{{ $t(row.areaKey) }}</td>
      <td>
        <ControlTowerStatusBadge :label="statusLabel(row.status)" :tone="statusTone(row.status)" />
      </td>
      <td>{{ row.status === 'down' ? '—' : row.count }}</td>
      <td>
        <NuxtLink :to="row.link">{{ $t('common.details') }}</NuxtLink>
      </td>
    </tr>
  </UiTable>
</template>
