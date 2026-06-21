<script setup lang="ts">
import type { Driver } from '~/types/shipment'

defineProps<{ driver: Driver }>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.driverDetails') }}</h3>
    </template>
    <div class="form-grid form-grid--2">
      <div>
        <span class="text-muted">{{ $t('users.fullName') }}</span>
        <div>{{ driver.full_name }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('users.phone') }}</span>
        <div>{{ driver.phone || '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('shipments.licenseNumber') }}</span>
        <div>{{ driver.license_number || '—' }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('common.status') }}</span>
        <div><DriversDriverStatusBadge :status="driver.status" /></div>
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
