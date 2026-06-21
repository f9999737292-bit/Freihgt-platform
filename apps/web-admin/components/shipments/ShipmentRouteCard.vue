<script setup lang="ts">
import type { LocationSummary } from '~/types/shipment'

defineProps<{
  origin?: LocationSummary | null
  destination?: LocationSummary | null
  loading?: boolean
}>()

function locationLabel(location?: LocationSummary | null) {
  if (!location) return '—'
  const parts = [location.name, location.city, location.country_code].filter(Boolean)
  return parts.length ? parts.join(', ') : location.id
}
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.route') }}</h3>
    </template>
    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
    <div v-else class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('shipments.origin') }}</span>
        <div>{{ locationLabel(origin) }}</div>
        <div v-if="origin?.address_line" class="text-sm text-muted">{{ origin.address_line }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.destination') }}</span>
        <div>{{ locationLabel(destination) }}</div>
        <div v-if="destination?.address_line" class="text-sm text-muted">{{ destination.address_line }}</div>
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
