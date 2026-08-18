<script setup lang="ts">
import { RFX_STATUSES, RFX_TYPES, formatRfxDate, type RfxEvent } from '~/types/rfx'
import type { Company } from '~/types/company'
import { TenantRequiredError } from '~/utils/apiClient'
import { buildStatusFilterOptions } from '~/utils/rfxStatusFilters'
import { shouldShowNotFound } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listRfxEvents, isApiUnavailableError } = useRfxApi()
const { listCompanies } = useCompanies()
const { hasTenant } = useTenantContext()
const { canManageTenders } = usePermissions()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<RfxEvent[]>([])
const total = ref(0)
const companies = ref<Company[]>([])
const loading = ref(true)
const loadFailed = ref(false)

const filters = reactive({
  status: '',
  owner_company_id: '',
  rfx_type: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const statusOptions = computed(() =>
  buildStatusFilterOptions(RFX_STATUSES, t('common.all')),
)
const typeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...RFX_TYPES.map((value) => ({ label: value, value })),
])
const ownerOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...companies.value.map((company) => ({ label: company.legal_name, value: company.id })),
])

const companyName = (id?: string) =>
  id ? companies.value.find((company) => company.id === id)?.legal_name || `${id.slice(0, 8)}…` : '—'

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

async function loadCompanies() {
  try {
    const data = await listCompanies({ limit: 100, status: 'ACTIVE' })
    companies.value = data.items
  } catch {
    companies.value = []
  }
}

async function loadTenders() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listRfxEvents({
      status: filters.status,
      owner_company_id: filters.owner_company_id,
      rfx_type: filters.rfx_type,
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
    if (!loadFailed.value && !shouldShowNotFound(error)) {
      pushToast('error', error instanceof Error ? error.message : t('tenders.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  loadTenders()
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadTenders()
}

function goNext() {
  pagination.offset += pagination.limit
  loadTenders()
}

onMounted(async () => {
  await loadCompanies()
  await loadTenders()
})
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="$t('tenders.title')">
      <template #actions>
        <Button v-if="canManageTenders()" @click="$router.push('/tenders/new')">
          {{ $t('tenders.create') }}
        </Button>
      </template>
    </PageHeader>

    <Card>
      <div class="filters-row">
        <Select
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <Select
          v-model="filters.owner_company_id"
          :label="$t('tenders.ownerCompany')"
          :options="ownerOptions"
          @update:model-value="onFiltersChange"
        />
        <Select
          v-model="filters.rfx_type"
          :label="$t('tenders.type')"
          :options="typeOptions"
          @update:model-value="onFiltersChange"
        />
      </div>
    </Card>

    <EmptyState v-if="loadFailed && !loading" :title="$t('tenders.loadFailed')" />
    <EmptyState v-else-if="!loading && !hasItems" :title="$t('tenders.empty')" />

    <Card v-else>
      <Table
        :columns="[
          $t('tenders.number'),
          $t('tenders.type'),
          $t('tenders.titleLabel'),
          $t('tenders.ownerCompany'),
          $t('tenders.deadline'),
          $t('common.status'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/tenders/${item.id}`" class="link">{{ item.rfx_number }}</NuxtLink>
          </td>
          <td>{{ item.rfx_type }}</td>
          <td>{{ item.title }}</td>
          <td>{{ companyName(item.owner_company_id) }}</td>
          <td>{{ formatRfxDate(item.response_deadline) }}</td>
          <td><Badge :status="item.status" /></td>
          <td>
            <NuxtLink :to="`/tenders/${item.id}`">{{ $t('common.details') }}</NuxtLink>
          </td>
        </tr>
      </Table>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }} {{ $t('tenders.countLabel') }}</span>
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
