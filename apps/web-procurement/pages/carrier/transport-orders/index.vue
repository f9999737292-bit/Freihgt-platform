<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { CarrierTransportOrderItem } from '~/types/orderExecution'
import { formatMoney } from '~/types/evaluation'
import {
  filterCarrierMemberships,
  membershipSelectOptions,
  selectDefaultCarrierCompany,
} from '~/utils/companyMembership'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listCarrierTransportOrders } = useOrderExecutionApi()
const { getUserCompanies } = useCompanies()
const authStore = useAuthStore()
const { t } = useI18n()

const loading = ref(true)
const apiUnavailable = ref(false)
const items = ref<CarrierTransportOrderItem[]>([])
const memberships = ref<UserCompanyMembership[]>([])
const selectedCarrierId = ref('')

const carrierOptions = computed(() => membershipSelectOptions(filterCarrierMemberships(memberships.value)))

async function loadOrders() {
  loading.value = true
  apiUnavailable.value = false
  try {
    if (!selectedCarrierId.value) {
      items.value = []
      return
    }
    items.value = await listCarrierTransportOrders(selectedCarrierId.value)
  } catch {
    items.value = []
    apiUnavailable.value = isApiUnavailableError(new Error('failed'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  memberships.value = await getUserCompanies(authStore.user?.id || '')
  selectedCarrierId.value = selectDefaultCarrierCompany(filterCarrierMemberships(memberships.value))
  await loadOrders()
})

watch(selectedCarrierId, loadOrders)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('orderExecution.carrierTitle')" />

    <Card>
      <label class="field-label" for="carrier-select">{{ t('orderExecution.winningCarrier') }}</label>
      <select id="carrier-select" v-model="selectedCarrierId" class="input">
        <option v-for="opt in carrierOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
      </select>
    </Card>

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
              <NuxtLink :to="`/carrier/transport-orders/${item.transport_order_id}`">
                {{ t('orderExecution.title') }}
              </NuxtLink>
            </td>
          </tr>
        </Table>
      </div>
    </Card>
  </div>
</template>
