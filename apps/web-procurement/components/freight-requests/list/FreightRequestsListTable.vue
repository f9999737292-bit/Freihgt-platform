<script setup lang="ts">
import type { FreightRequest } from '~/types/rfx'
import { formatDateTime } from '~/utils/format'

defineProps<{
  items: FreightRequest[]
  loading?: boolean
}>()
</script>

<template>
  <div class="fr-list-table-wrap">
    <table class="fr-list-table">
      <thead>
        <tr>
          <th>{{ $t('freightRequests.list.number') }}</th>
          <th>{{ $t('freightRequests.list.requestType') }}</th>
          <th>{{ $t('freightRequests.list.status') }}</th>
          <th>{{ $t('freightRequests.list.responseDeadline') }}</th>
          <th>{{ $t('freightRequests.list.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td colspan="5" class="fr-list-table__loading">{{ $t('common.loading') }}</td>
        </tr>
        <tr v-for="item in items" v-else :key="item.id">
          <td>
            <NuxtLink :to="`/freight-requests/${item.id}`" class="fr-list-table__link">
              {{ item.freight_request_number }}
            </NuxtLink>
          </td>
          <td>
            <FreightRequestsListFreightRequestTypeBadge :type="item.request_type" />
          </td>
          <td>
            <FreightRequestsListFreightRequestStatusBadge :status="item.status" />
          </td>
          <td>{{ formatDateTime(item.response_deadline) }}</td>
          <td>
            <NuxtLink :to="`/freight-requests/${item.id}`" class="fr-list-table__link">
              {{ $t('freightRequests.list.details') }}
            </NuxtLink>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.fr-list-table-wrap {
  overflow-x: auto;
}

.fr-list-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.fr-list-table th,
.fr-list-table td {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #e5e7eb;
  text-align: left;
  vertical-align: middle;
}

.fr-list-table th {
  color: #6b7280;
  font-weight: 600;
  background: #f9fafb;
}

.fr-list-table__loading {
  color: #6b7280;
  text-align: center;
}

.fr-list-table__link {
  color: #2563eb;
  font-weight: 500;
  text-decoration: none;
}

.fr-list-table__link:hover {
  text-decoration: underline;
}
</style>
