<script setup lang="ts">
import type { BuyerTransportOrderItem } from '~/types/orderExecution'
import { formatMoney } from '~/types/evaluation'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listBuyerTransportOrders } = useOrderExecutionApi()
const { currentCompanyId } = useTenantContext()
const { t } = useI18n()

const loading = ref(true)
const apiUnavailable = ref(false)
const items = ref<BuyerTransportOrderItem[]>([])

async function loadOrders() {
  loading.value = true
  apiUnavailable.value = false
  try {
    if (!currentCompanyId.value) {
      items.value = []
      return
    }
    items.value = await listBuyerTransportOrders(currentCompanyId.value)
  } catch {
    items.value = []
    apiUnavailable.value = isApiUnavailableError(new Error('failed'))
  } finally {
    loading.value = false
  }
}

watch(currentCompanyId, loadOrders, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('orderExecution.buyerTitle')" />

    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="apiUnavailable" :title="t('orderExecution.loadFailed')" />
    <EmptyState v-else-if="items.length === 0" :title="t('orderExecution.empty')" />

    <Card v-else>
      <div class="table-scroll">
        <Table
          :columns="[
            t('orderExecution.orderNumber'),
            t('orderExecution.orderStatus'),
            t('orderExecution.awardAmount'),
            t('orderExecution.shipmentStatus'),
            '',
          ]"
        >
          <tr v-for="item in items" :key="item.transport_order_id">
            <td>{{ item.transport_order_number }}</td>
            <td><Badge :status="item.transport_order_status" /></td>
            <td>{{ formatMoney(item.amount, item.currency_code) }}</td>
            <td><Badge v-if="item.shipment_status" :status="item.shipment_status" /><span v-else>—</span></td>
            <td>
              <NuxtLink :to="`/transport-orders/${item.transport_order_id}`">
                {{ t('orderExecution.title') }}
              </NuxtLink>
            </td>
          </tr>
        </Table>
      </div>
    </Card>
  </div>
</template>
