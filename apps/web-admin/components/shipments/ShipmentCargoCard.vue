<script setup lang="ts">
import type { CargoSummary } from '~/types/shipment'

defineProps<{
  cargo?: CargoSummary | null
  loading?: boolean
}>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.cargo') }}</h3>
    </template>
    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
    <div v-else-if="!cargo" class="text-muted">—</div>
    <div v-else class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('rfx.description') }}</span>
        <div>{{ cargo.description || '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.weight') }}</span>
        <div>{{ cargo.weight_kg != null ? `${cargo.weight_kg} kg` : '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.volume') }}</span>
        <div>{{ cargo.volume_m3 != null ? `${cargo.volume_m3} m³` : '—' }}</div>
      </div>
    </div>
  </UiCard>
</template>

<style scoped>
.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}
</style>
