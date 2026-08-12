<script setup lang="ts">
import type { FreightRequest } from '~/types/rfx'
import { formatDateTime } from '~/utils/format'

defineProps<{
  request: FreightRequest
}>()
</script>

<template>
  <section class="panel">
    <h3>{{ $t('freightRequests.detail.summary') }}</h3>
    <dl class="details-grid">
      <div>
        <dt>{{ $t('freightRequests.detail.freightRequestNumber') }}</dt>
        <dd>{{ request.freight_request_number }}</dd>
      </div>
      <div>
        <dt>{{ $t('freightRequests.detail.requestType') }}</dt>
        <dd>{{ request.request_type }}</dd>
      </div>
      <div>
        <dt>{{ $t('common.status') }}</dt>
        <dd>
          <span class="status-badge" :class="`status-badge--${request.status.toLowerCase()}`">
            {{ request.status }}
          </span>
        </dd>
      </div>
      <div>
        <dt>{{ $t('freightRequests.detail.shipperCompanyId') }}</dt>
        <dd>{{ request.shipper_company_id }}</dd>
      </div>
      <div>
        <dt>{{ $t('freightRequests.detail.responseDeadline') }}</dt>
        <dd>{{ formatDateTime(request.response_deadline) }}</dd>
      </div>
      <div>
        <dt>{{ $t('freightRequests.detail.currency') }}</dt>
        <dd>{{ request.currency_code || '—' }}</dd>
      </div>
      <div>
        <dt>{{ $t('freightRequests.detail.createdAt') }}</dt>
        <dd>{{ formatDateTime(request.created_at) }}</dd>
      </div>
      <div>
        <dt>{{ $t('freightRequests.detail.updatedAt') }}</dt>
        <dd>{{ formatDateTime(request.updated_at) }}</dd>
      </div>
    </dl>
  </section>
</template>

<style scoped>
.panel {
  padding: 1rem 1.25rem;
  border-radius: 0.5rem;
  background: #fff;
  border: 1px solid #e5e7eb;
}

.panel h3 {
  margin: 0 0 1rem;
  font-size: 1rem;
}

.details-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem 1.5rem;
  margin: 0;
}

.details-grid div {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.details-grid dt {
  margin: 0;
  font-size: 0.8125rem;
  color: #6b7280;
}

.details-grid dd {
  margin: 0;
}

.status-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.status-badge--draft {
  background: #eef2f6;
  color: #475569;
}

.status-badge--published,
.status-badge--responses_open {
  background: #dbeafe;
  color: #1e40af;
}

.status-badge--awarded {
  background: #dcfce7;
  color: #166534;
}

@media (max-width: 768px) {
  .details-grid {
    grid-template-columns: 1fr;
  }
}
</style>
