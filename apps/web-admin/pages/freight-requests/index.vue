<script setup lang="ts">
import {
  FREIGHT_REQUEST_STATUSES,
  FREIGHT_REQUEST_TYPES,
  formatRfxDate,
  type FreightRequest,
} from '~/types/rfx'
import type { Company } from '~/types/company'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listFreightRequests, isApiUnavailableError } = useFreightRequestsApi()
const { listCompanies } = useCompanies()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<FreightRequest[]>([])
const total = ref(0)
const companies = ref<Company[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showCreateModal = ref(false)

const filters = reactive({
  search: '',
  request_type: '',
  status: '',
  shipper_company_id: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const typeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...FREIGHT_REQUEST_TYPES.map((v) => ({ label: v, value: v })),
])
const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...FREIGHT_REQUEST_STATUSES.map((v) => ({ label: v, value: v })),
])
const shipperOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
])

const companyName = (id?: string) =>
  id ? companies.value.find((c) => c.id === id)?.legal_name || id.slice(0, 8) + '...' : '—'

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
    const data = await listFreightRequests({
      search: filters.search,
      request_type: filters.request_type,
      status: filters.status,
      shipper_company_id: filters.shipper_company_id,
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
      pushToast('error', error instanceof Error ? error.message : t('freightRequests.loadFailed'))
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

onMounted(async () => {
  try {
    companies.value = (await listCompanies({ limit: 100 })).items
  } catch {
    companies.value = []
  }
  await load()
})
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('freightRequests.title')">
      <template #actions>
        <UiButton @click="showCreateModal = true">{{ $t('freightRequests.createFromOrder') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-row">
        <UiInput v-model="filters.search" :label="$t('common.search')" @update:model-value="onSearchInput" />
        <UiSelect
          v-model="filters.request_type"
          :label="$t('freightRequests.requestType')"
          :options="typeOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.shipper_company_id"
          :label="$t('freightRequests.shipper')"
          :options="shipperOptions"
          @update:model-value="onFiltersChange"
        />
      </div>
    </UiCard>

    <UiEmptyState v-if="loadFailed && !loading" :title="$t('freightRequests.loadFailed')" />
    <UiEmptyState v-else-if="!loading && !hasItems" :title="$t('freightRequests.noRequestsFound')" />

    <UiCard v-else>
      <UiTable
        :columns="[
          $t('freightRequests.number'),
          $t('freightRequests.requestType'),
          $t('freightRequests.shipper'),
          $t('freightRequests.transportOrder'),
          $t('rfx.responseDeadline'),
          $t('rfx.currency'),
          $t('common.status'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/freight-requests/${item.id}`" class="link">
              {{ item.freight_request_number }}
            </NuxtLink>
          </td>
          <td><FreightRequestsFreightRequestTypeBadge :type="item.request_type" /></td>
          <td>{{ companyName(item.shipper_company_id) }}</td>
          <td>
            <NuxtLink v-if="item.transport_order_id" :to="`/transport-orders/${item.transport_order_id}`">
              {{ item.transport_order_id.slice(0, 8) }}...
            </NuxtLink>
            <span v-else>—</span>
          </td>
          <td>{{ formatRfxDate(item.response_deadline) }}</td>
          <td>{{ item.currency_code || '—' }}</td>
          <td><FreightRequestsFreightRequestStatusBadge :status="item.status" /></td>
          <td><NuxtLink :to="`/freight-requests/${item.id}`">{{ $t('common.details') }}</NuxtLink></td>
        </tr>
      </UiTable>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }}</span>
        <div class="pagination__actions">
          <UiButton size="sm" variant="secondary" :disabled="!canGoPrev" @click="pagination.offset -= pagination.limit; load()">←</UiButton>
          <UiButton size="sm" variant="secondary" :disabled="!canGoNext" @click="pagination.offset += pagination.limit; load()">→</UiButton>
        </div>
      </div>
    </UiCard>

    <FreightRequestsFreightRequestCreateFromTransportOrderModal
      :open="showCreateModal"
      @close="showCreateModal = false"
      @created="load"
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
