<script setup lang="ts">
import { formatRfxDate, type FreightRequest } from '~/types/rfx'

defineProps<{ request: FreightRequest; companyName?: string }>()
</script>

<template>
  <UiCard>
    <template #header>
      <div class="details-card__header">
        <div>
          <h2>{{ request.freight_request_number }}</h2>
          <p class="text-muted">{{ $t('freightRequests.details') }}</p>
        </div>
        <FreightRequestsFreightRequestStatusBadge :status="request.status" />
      </div>
    </template>

    <div class="details-grid">
      <div class="details-item">
        <span class="details-item__label">{{ $t('freightRequests.requestType') }}</span>
        <FreightRequestsFreightRequestTypeBadge :type="request.request_type" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('common.status') }}</span>
        <FreightRequestsFreightRequestStatusBadge :status="request.status" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('freightRequests.shipper') }}</span>
        <span>{{ companyName || request.shipper_company_id }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('freightRequests.transportOrder') }}</span>
        <NuxtLink v-if="request.transport_order_id" :to="`/transport-orders/${request.transport_order_id}`">
          {{ request.transport_order_id }}
        </NuxtLink>
        <span v-else>—</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.responseDeadline') }}</span>
        <span>{{ formatRfxDate(request.response_deadline) }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.currency') }}</span>
        <span>{{ request.currency_code || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('freightRequests.createdAt') }}</span>
        <span>{{ formatRfxDate(request.created_at) }}</span>
      </div>
    </div>
  </UiCard>
</template>

<style scoped>
.details-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.details-card__header h2 {
  margin: 0;
  font-size: 1.125rem;
}

.details-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem 1.5rem;
}

.details-item {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.details-item__label {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

@media (max-width: 768px) {
  .details-grid {
    grid-template-columns: 1fr;
  }
}
</style>
