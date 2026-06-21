<script setup lang="ts">
import {
  RFX_CATEGORIES,
  RFX_STATUSES,
  RFX_TYPES,
  formatRfxDate,
  type RfxEvent,
} from '~/types/rfx'
import type { Company } from '~/types/company'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listRfxEvents, isApiUnavailableError } = useRfxApi()
const { listCompanies } = useCompanies()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<RfxEvent[]>([])
const total = ref(0)
const companies = ref<Company[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showCreateModal = ref(false)

const filters = reactive({
  search: '',
  rfx_type: '',
  category: '',
  status: '',
  owner_company_id: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const typeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...RFX_TYPES.map((v) => ({ label: v, value: v })),
])
const categoryOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...RFX_CATEGORIES.map((v) => ({ label: v, value: v })),
])
const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...RFX_STATUSES.map((v) => ({ label: v, value: v })),
])
const ownerOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
])

const companyName = (id?: string) =>
  id ? companies.value.find((c) => c.id === id)?.legal_name || id.slice(0, 8) + '...' : '—'

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

let searchTimer: ReturnType<typeof setTimeout> | undefined

async function loadCompanies() {
  try {
    const data = await listCompanies({ limit: 100 })
    companies.value = data.items
  } catch {
    companies.value = []
  }
}

async function loadRfx() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listRfxEvents({
      search: filters.search,
      rfx_type: filters.rfx_type,
      category: filters.category,
      status: filters.status,
      owner_company_id: filters.owner_company_id,
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
      pushToast('error', error instanceof Error ? error.message : t('rfx.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  loadRfx()
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(onFiltersChange, 350)
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadRfx()
}

function goNext() {
  pagination.offset += pagination.limit
  loadRfx()
}

onMounted(async () => {
  await loadCompanies()
  await loadRfx()
})
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('rfx.title')">
      <template #actions>
        <UiButton @click="showCreateModal = true">{{ $t('rfx.create') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-row">
        <UiInput
          v-model="filters.search"
          :label="$t('common.search')"
          @update:model-value="onSearchInput"
        />
        <UiSelect
          v-model="filters.rfx_type"
          :label="$t('rfx.rfxType')"
          :options="typeOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.category"
          :label="$t('rfx.category')"
          :options="categoryOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.owner_company_id"
          :label="$t('rfx.owner')"
          :options="ownerOptions"
          @update:model-value="onFiltersChange"
        />
      </div>
    </UiCard>

    <UiEmptyState v-if="loadFailed && !loading" :title="$t('rfx.loadFailed')" />
    <UiEmptyState v-else-if="!loading && !hasItems" :title="$t('rfx.noRfxFound')" />

    <UiCard v-else>
      <UiTable
        :columns="[
          $t('rfx.rfxNumber'),
          $t('rfx.rfxType'),
          $t('rfx.category'),
          $t('rfx.titleLabel'),
          $t('rfx.owner'),
          $t('rfx.responseDeadline'),
          $t('common.status'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/rfx/${item.id}`" class="link">{{ item.rfx_number }}</NuxtLink>
          </td>
          <td><RfxRfxTypeBadge :type="item.rfx_type" /></td>
          <td>{{ item.category }}</td>
          <td>{{ item.title }}</td>
          <td>{{ companyName(item.owner_company_id) }}</td>
          <td>{{ formatRfxDate(item.response_deadline) }}</td>
          <td><RfxRfxStatusBadge :status="item.status" /></td>
          <td><NuxtLink :to="`/rfx/${item.id}`">{{ $t('common.details') }}</NuxtLink></td>
        </tr>
      </UiTable>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }} RFx</span>
        <div class="pagination__actions">
          <UiButton size="sm" variant="secondary" :disabled="!canGoPrev" @click="goPrev">←</UiButton>
          <UiButton size="sm" variant="secondary" :disabled="!canGoNext" @click="goNext">→</UiButton>
        </div>
      </div>
    </UiCard>

    <RfxRfxCreateModal :open="showCreateModal" @close="showCreateModal = false" @created="loadRfx" />
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
