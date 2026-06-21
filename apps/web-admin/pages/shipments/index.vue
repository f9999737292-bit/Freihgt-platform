<script setup lang="ts">
import {
  SHIPMENT_STATUSES,
  formatShipmentDate,
  shortId,
  type Shipment,
} from '~/types/shipment'
import type { Company } from '~/types/company'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { listShipments, isApiUnavailableError } = useShipmentsApi()
const { listCompanies } = useCompanies()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<Shipment[]>([])
const total = ref(0)
const companies = ref<Company[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showBidModal = ref(false)
const showOrderModal = ref(false)
const initialBidId = ref('')
const initialTransportOrderId = ref('')

const filters = reactive({
  search: '',
  status: '',
  shipper_company_id: '',
  consignee_company_id: '',
  carrier_company_id: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...SHIPMENT_STATUSES.map((v) => ({ label: v, value: v })),
])

const companyFilterOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
])

const companyName = (id?: string | null) =>
  id ? companies.value.find((c) => c.id === id)?.legal_name || shortId(id) : '—'

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

let searchTimer: ReturnType<typeof setTimeout> | undefined

async function load() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listShipments({
      search: filters.search,
      status: filters.status,
      shipper_company_id: filters.shipper_company_id,
      consignee_company_id: filters.consignee_company_id,
      carrier_company_id: filters.carrier_company_id,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items ?? []
    total.value = data.total ?? items.value.length
  } catch (error) {
    items.value = []
    total.value = 0
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('shipments.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  load()
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(onFiltersChange, 350)
}

function openBidModal(bidId = '', transportOrderId = '') {
  initialBidId.value = bidId
  initialTransportOrderId.value = transportOrderId
  showBidModal.value = true
}

function onCreated() {
  load()
}

onMounted(async () => {
  try {
    companies.value = (await listCompanies({ limit: 100 })).items
  } catch {
    companies.value = []
  }
  await load()

  const bidId = String(route.query.bid_id || '')
  const transportOrderId = String(route.query.transport_order_id || '')
  if (bidId || transportOrderId) {
    openBidModal(bidId, transportOrderId)
  }
})
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('shipments.title')">
      <template #actions>
        <UiButton variant="secondary" @click="showOrderModal = true">
          {{ $t('shipments.createFromTransportOrder') }}
        </UiButton>
        <UiButton @click="openBidModal()">{{ $t('shipments.createFromBid') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-row">
        <UiInput v-model="filters.search" :label="$t('common.search')" @update:model-value="onSearchInput" />
        <UiSelect
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.shipper_company_id"
          :label="$t('transportOrders.shipper')"
          :options="companyFilterOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.consignee_company_id"
          :label="$t('transportOrders.consignee')"
          :options="companyFilterOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.carrier_company_id"
          :label="$t('shipments.carrier')"
          :options="companyFilterOptions"
          @update:model-value="onFiltersChange"
        />
      </div>
    </UiCard>

    <UiEmptyState v-if="loadFailed && !loading" :title="$t('shipments.loadFailed')" />
    <UiEmptyState v-else-if="!loading && !hasItems" :title="$t('shipments.noShipmentsFound')" />

    <UiCard v-else>
      <UiTable
        :columns="[
          $t('shipments.shipmentNumber'),
          $t('transportOrders.shipper'),
          $t('transportOrders.consignee'),
          $t('shipments.carrier'),
          $t('shipments.driver'),
          $t('shipments.vehicle'),
          $t('shipments.plannedPickup'),
          $t('shipments.plannedDelivery'),
          $t('common.status'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/shipments/${item.id}`" class="link">
              {{ item.shipment_number }}
            </NuxtLink>
          </td>
          <td>{{ companyName(item.shipper_company_id) }}</td>
          <td>{{ companyName(item.consignee_company_id) }}</td>
          <td>{{ companyName(item.carrier_company_id) }}</td>
          <td>{{ item.driver_id ? shortId(item.driver_id) : '—' }}</td>
          <td>{{ item.vehicle_id ? shortId(item.vehicle_id) : '—' }}</td>
          <td>{{ formatShipmentDate(item.planned_pickup_at) }}</td>
          <td>{{ formatShipmentDate(item.planned_delivery_at) }}</td>
          <td><ShipmentsShipmentStatusBadge :status="item.status" /></td>
          <td><NuxtLink :to="`/shipments/${item.id}`">{{ $t('common.details') }}</NuxtLink></td>
        </tr>
      </UiTable>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }}</span>
        <div class="pagination__actions">
          <UiButton
            size="sm"
            variant="secondary"
            :disabled="!canGoPrev"
            @click="pagination.offset -= pagination.limit; load()"
          >
            ←
          </UiButton>
          <UiButton
            size="sm"
            variant="secondary"
            :disabled="!canGoNext"
            @click="pagination.offset += pagination.limit; load()"
          >
            →
          </UiButton>
        </div>
      </div>
    </UiCard>

    <ShipmentsShipmentCreateFromBidModal
      :open="showBidModal"
      :initial-bid-id="initialBidId"
      :initial-transport-order-id="initialTransportOrderId"
      @close="showBidModal = false"
      @created="onCreated"
    />

    <ShipmentsShipmentCreateFromTransportOrderModal
      :open="showOrderModal"
      @close="showOrderModal = false"
      @created="onCreated"
    />
  </div>
</template>

<style scoped>
.link {
  font-weight: 500;
  text-decoration: none;
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
