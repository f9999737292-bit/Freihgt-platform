<script setup lang="ts">
import type { BuyerTransportOrderItem } from '~/types/orderExecution'
import { formatMoney } from '~/types/evaluation'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listBuyerTransportOrders } = useOrderExecutionApi()
const { currentCompanyId } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(true)
const apiUnavailable = ref(false)
const items = ref<BuyerTransportOrderItem[]>([])
const total = ref(0)
const pagination = reactive({ limit: 20, offset: 0 })

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)
const missingCompany = computed(() => !currentCompanyId.value)

async function loadOrders() {
  loading.value = true
  apiUnavailable.value = false
  try {
    if (!currentCompanyId.value) {
      items.value = []
      total.value = 0
      return
    }
    const data = await listBuyerTransportOrders(
      currentCompanyId.value,
      pagination.limit,
      pagination.offset,
    )
    items.value = data.items
    total.value = data.total
  } catch (error) {
    items.value = []
    total.value = 0
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('orderExecution.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadOrders()
}

function goNext() {
  pagination.offset += pagination.limit
  loadOrders()
}

watch(currentCompanyId, () => {
  pagination.offset = 0
  loadOrders()
}, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('orderExecution.buyerTitle')" />

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState
      v-else-if="missingCompany"
      :title="t('orderExecution.missingCompany')"
    />
    <EmptyState v-else-if="apiUnavailable" :title="t('orderExecution.loadFailed')" />
    <EmptyState v-else-if="!hasItems" :title="t('orderExecution.buyerEmpty')" />

    <Card v-else>
      <div class="table-scroll">
        <Table
          :columns="[
            t('orderExecution.orderNumber'),
            t('orderExecution.orderStatus'),
            t('orderExecution.awardAmount'),
            t('orderExecution.shipmentStatus'),
            t('common.actions'),
          ]"
          :loading="loading"
        >
          <tr v-for="item in items" :key="item.transport_order_id">
            <td>
              <NuxtLink :to="`/transport-orders/${item.transport_order_id}`" class="link">
                {{ item.transport_order_number }}
              </NuxtLink>
            </td>
            <td><Badge :status="item.transport_order_status" /></td>
            <td>{{ formatMoney(item.amount, item.currency_code) }}</td>
            <td>
              <Badge v-if="item.shipment_status" :status="item.shipment_status" />
              <span v-else aria-hidden="true">—</span>
            </td>
            <td>
              <NuxtLink :to="`/transport-orders/${item.transport_order_id}`">
                {{ t('common.details') }}
              </NuxtLink>
            </td>
          </tr>
        </Table>
      </div>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }} {{ t('orderExecution.countLabel') }}</span>
        <div class="pagination__actions">
          <Button size="sm" variant="secondary" :disabled="!canGoPrev" @click="goPrev">←</Button>
          <Button size="sm" variant="secondary" :disabled="!canGoNext" @click="goNext">→</Button>
        </div>
      </div>
    </Card>
  </div>
</template>

<style scoped>
.link {
  font-weight: 500;
  text-decoration: none;
}

.link:hover {
  text-decoration: underline;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--color-border);
}

.pagination__actions {
  display: flex;
  gap: 0.5rem;
}
</style>
