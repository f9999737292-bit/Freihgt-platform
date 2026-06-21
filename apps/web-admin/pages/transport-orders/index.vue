<script setup lang="ts">
import type { PaginatedResponse } from '~/types/api'
import type { TransportOrder } from '~/types/transportOrder'
import { TenantRequiredError, isApiUnavailableError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const tenantStore = useTenantStore()
const { apiGet } = useApi()

const items = ref<TransportOrder[]>([])
const loading = ref(true)
const apiUnavailable = ref(false)

async function loadItems() {
  if (!tenantStore.tenantId) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  apiUnavailable.value = false
  try {
    const data = await apiGet<PaginatedResponse<TransportOrder>>('/api/v1/transport-orders', {
      query: { tenant_id: tenantStore.tenantId, limit: 100, offset: 0 },
    })
    items.value = data.items ?? []
  } catch (error) {
    items.value = []
    if (error instanceof TenantRequiredError) {
      return
    }
    apiUnavailable.value = isApiUnavailableError(error)
  } finally {
    loading.value = false
  }
}

onMounted(loadItems)
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('transportOrders.title')">
      <template #actions>
        <UiButton>{{ $t('transportOrders.create') }}</UiButton>
      </template>
    </UiPageHeader>

    <CommonApiUnavailableState v-if="apiUnavailable" @retry="loadItems" />

    <UiTable
      v-else-if="items.length || loading"
      :columns="[
        '#',
        $t('common.status'),
        $t('transportOrders.pickupDate'),
        $t('transportOrders.deliveryDate'),
        $t('transportOrders.equipmentType'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.id">
        <td><NuxtLink :to="`/transport-orders/${item.id}`">{{ item.order_number || item.id.slice(0, 8) }}</NuxtLink></td>
        <td><UiBadge :status="item.status" /></td>
        <td>{{ item.pickup_date || '—' }}</td>
        <td>{{ item.delivery_date || '—' }}</td>
        <td>{{ item.equipment_type || '—' }}</td>
        <td><NuxtLink :to="`/transport-orders/${item.id}`">{{ $t('common.details') }}</NuxtLink></td>
      </tr>
    </UiTable>
    <UiEmptyState v-else :title="$t('common.empty')" />
  </div>
</template>
