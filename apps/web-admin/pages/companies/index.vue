<script setup lang="ts">
import { COMPANY_STATUSES, COMPANY_TYPES, formatCompanyDate, type Company } from '~/types/company'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listCompanies, isApiUnavailableError } = useCompanies()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const items = ref<Company[]>([])
const total = ref(0)
const loading = ref(true)
const apiUnavailable = ref(false)
const showCreateModal = ref(false)

const filters = reactive({
  search: '',
  company_type: '',
  status: '',
  country_code: '',
})

const pagination = reactive({
  limit: 20,
  offset: 0,
})

const companyTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...COMPANY_TYPES.map((value) => ({ label: value, value })),
])

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...COMPANY_STATUSES.map((value) => ({ label: value, value })),
])

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

let searchTimer: ReturnType<typeof setTimeout> | undefined

async function loadCompanies() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    total.value = 0
    return
  }

  loading.value = true
  apiUnavailable.value = false
  try {
    const data = await listCompanies({
      search: filters.search,
      company_type: filters.company_type,
      status: filters.status,
      country_code: filters.country_code,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items ?? []
    total.value = data.total ?? items.value.length
  } catch (error) {
    items.value = []
    total.value = 0
    if (error instanceof TenantRequiredError) {
      return
    }
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  loadCompanies()
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(onFiltersChange, 350)
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadCompanies()
}

function goNext() {
  pagination.offset += pagination.limit
  loadCompanies()
}

function onCreated(companyId: string) {
  loadCompanies()
  router.push(`/companies/${companyId}`)
}

onMounted(loadCompanies)
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('companies.title')">
      <template #actions>
        <UiButton @click="showCreateModal = true">{{ $t('companies.create') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard v-if="!hasTenant" class="tenant-warning">
      <p>{{ $t('tenant.notSelected') }}</p>
    </UiCard>

    <UiCard>
      <div class="filters-row">
        <UiInput
          v-model="filters.search"
          :label="$t('common.search')"
          :placeholder="$t('common.search')"
          @update:model-value="onSearchInput"
        />
        <UiSelect
          v-model="filters.company_type"
          :label="$t('companies.companyType')"
          :options="companyTypeOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <UiInput
          v-model="filters.country_code"
          :label="$t('companies.country')"
          maxlength="2"
          placeholder="RU"
          @update:model-value="onSearchInput"
        />
      </div>
    </UiCard>

    <UiEmptyState
      v-if="apiUnavailable && !loading"
      :title="$t('companies.apiUnavailable')"
    />

    <UiEmptyState
      v-else-if="!loading && !hasItems"
      :title="$t('companies.noCompanies')"
    />

    <UiCard v-else>
      <UiTable
        :columns="[
          $t('companies.legalName'),
          $t('companies.shortName'),
          $t('companies.companyType'),
          $t('companies.taxId'),
          $t('companies.country'),
          $t('companies.preferredLanguage'),
          $t('common.status'),
          $t('companies.createdAt'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/companies/${item.id}`" class="company-link">{{ item.legal_name }}</NuxtLink>
          </td>
          <td>{{ item.short_name || '—' }}</td>
          <td><CompaniesCompanyTypeBadge :type="item.company_type" /></td>
          <td>{{ item.tax_id || '—' }}</td>
          <td>{{ item.country_code }}</td>
          <td>{{ item.preferred_locale }}</td>
          <td><CompaniesCompanyStatusBadge :status="item.status" /></td>
          <td>{{ formatCompanyDate(item.created_at) }}</td>
          <td>
            <NuxtLink :to="`/companies/${item.id}`">{{ $t('common.details') }}</NuxtLink>
          </td>
        </tr>
      </UiTable>

      <div class="pagination">
        <span class="text-sm text-muted">
          {{ total }} {{ $t('companies.title').toLowerCase() }}
        </span>
        <div class="pagination__actions">
          <UiButton size="sm" variant="secondary" :disabled="!canGoPrev" @click="goPrev">
            ←
          </UiButton>
          <UiButton size="sm" variant="secondary" :disabled="!canGoNext" @click="goNext">
            →
          </UiButton>
        </div>
      </div>
    </UiCard>

    <CompaniesCompanyCreateModal
      :open="showCreateModal"
      @close="showCreateModal = false"
      @created="onCreated"
    />
  </div>
</template>

<style scoped>
.tenant-warning p {
  margin: 0;
  color: #92400e;
}

.company-link {
  font-weight: 500;
  text-decoration: none;
}

.company-link:hover {
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
