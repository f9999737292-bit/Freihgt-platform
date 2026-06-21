<script setup lang="ts">
import type { PaginatedResponse } from '~/types/api'
import type { BillingRegister } from '~/types/billing'
import { TenantRequiredError, isApiUnavailableError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const tenantStore = useTenantStore()
const { apiGet } = useApi()
const items = ref<BillingRegister[]>([])
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
    const data = await apiGet<PaginatedResponse<BillingRegister>>('/api/v1/billing-registers', {
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
    <UiPageHeader :title="$t('billing.title')" />

    <CommonApiUnavailableState v-if="apiUnavailable" @retry="loadItems" />

    <UiTable
      v-else-if="items.length || loading"
      :columns="[
        $t('billing.registerNumber'),
        $t('billing.customer'),
        $t('billing.contractor'),
        $t('billing.totalWithVat'),
        $t('common.status'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.id">
        <td><NuxtLink :to="`/billing-registers/${item.id}`">{{ item.register_number }}</NuxtLink></td>
        <td>{{ item.customer_company_id || '—' }}</td>
        <td>{{ item.contractor_company_id || '—' }}</td>
        <td>{{ item.total_with_vat ?? '—' }}</td>
        <td><UiBadge :status="item.status" /></td>
        <td><NuxtLink :to="`/billing-registers/${item.id}`">{{ $t('common.details') }}</NuxtLink></td>
      </tr>
    </UiTable>
    <UiEmptyState v-else :title="$t('common.empty')" />
  </div>
</template>
