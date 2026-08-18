<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { CarrierInvitedTender, CarrierResponseFilter } from '~/types/carrierRfx'
import { CARRIER_RESPONSE_FILTERS, formatDeadlineRemaining, isDeadlineExpired } from '~/types/carrierRfx'
import { formatRfxDate } from '~/types/rfx'
import type { Company } from '~/types/company'
import {
  filterCarrierMemberships,
  membershipSelectOptions,
  selectDefaultCarrierCompany,
} from '~/utils/companyMembership'
import { shouldShowNotFound } from '~/utils/apiError'
import { ApiError, TenantRequiredError } from '~/utils/apiClient'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listInvitedTenders, isApiUnavailableError } = useCarrierRfxApi()
const { getUserCompanies } = useCompanies()
const { listCompanies } = useCompanies()
const { hasTenant } = useTenantContext()
const { setCompany } = useTenantContext()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<CarrierInvitedTender[]>([])
const total = ref(0)
const companies = ref<Company[]>([])
const memberships = ref<UserCompanyMembership[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const permissionDenied = ref(false)

const selectedCarrierCompanyId = ref('')
const filters = reactive({
  response_filter: '' as '' | CarrierResponseFilter,
  search: '',
})
const pagination = reactive({ limit: 20, offset: 0 })

const carrierMemberships = computed(() => filterCarrierMemberships(memberships.value))
const carrierOptions = computed(() => membershipSelectOptions(carrierMemberships.value))
const hasMembership = computed(() => carrierMemberships.value.length > 0)
const companyName = (id?: string) =>
  id ? companies.value.find((company) => company.id === id)?.legal_name || `${id.slice(0, 8)}…` : '—'

const responseFilterLabels: Record<string, string> = {
  OPEN_FOR_RESPONSE: 'openForResponse',
  RESPONDED: 'responded',
  NOT_RESPONDED: 'notResponded',
  CLOSED: 'closed',
}

const responseFilterOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...CARRIER_RESPONSE_FILTERS.map((value) => ({
    label: t(`carrierTenders.filters.${responseFilterLabels[value]}`),
    value,
  })),
])

const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

function responseStatusLabel(status: string) {
  const key = `carrierTenders.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

async function loadMemberships() {
  const userId = authStore.user?.id
  if (!userId) {
    memberships.value = []
    return
  }
  try {
    memberships.value = await getUserCompanies(userId)
    if (!selectedCarrierCompanyId.value) {
      selectedCarrierCompanyId.value = selectDefaultCarrierCompany(filterCarrierMemberships(memberships.value))
    }
  } catch {
    memberships.value = []
  }
}

async function loadCompanies() {
  try {
    const data = await listCompanies({ limit: 200, status: 'ACTIVE' })
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
  if (!hasMembership.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  permissionDenied.value = false
  try {
    if (selectedCarrierCompanyId.value) {
      setCompany(selectedCarrierCompanyId.value)
    }
    const data = await listInvitedTenders({
      carrier_company_id: selectedCarrierCompanyId.value || undefined,
      response_filter: filters.response_filter || undefined,
      search: filters.search,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items ?? []
    total.value = data.total ?? items.value.length
  } catch (error) {
    items.value = []
    total.value = 0
    if (error instanceof ApiError && error.status === 403) {
      permissionDenied.value = true
    } else if (error instanceof TenantRequiredError) {
      // handled by empty tenant state
    } else {
      loadFailed.value = true
      if (!isApiUnavailableError(error)) {
        pushToast('error', t('carrierTenders.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

function applyFilters() {
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

watch(selectedCarrierCompanyId, () => {
  pagination.offset = 0
  loadTenders()
})

onMounted(async () => {
  await Promise.all([loadMemberships(), loadCompanies()])
  await loadTenders()
})
</script>

<template>
  <div>
    <UiPageHeader :title="t('carrierTenders.title')" :subtitle="t('carrierTenders.subtitle')" />

    <Card v-if="!hasTenant" class="mb-4">
      <EmptyState :title="t('tenant.required')" />
    </Card>

    <Card v-else-if="!hasMembership" class="mb-4">
      <EmptyState :title="t('carrierTenders.noMembership')" />
    </Card>

    <template v-else>
      <Card class="mb-4">
        <div class="filters">
          <Select
            v-if="carrierOptions.length > 1"
            v-model="selectedCarrierCompanyId"
            :label="t('carrierTenders.companyLabel')"
            :options="carrierOptions"
          />
          <Select
            v-model="filters.response_filter"
            :label="t('carrierTenders.filters.responseFilter')"
            :options="responseFilterOptions"
            @update:model-value="applyFilters"
          />
          <Input
            v-model="filters.search"
            :label="t('carrierTenders.filters.search')"
            @keyup.enter="applyFilters"
          />
          <Button variant="secondary" @click="applyFilters">{{ t('common.save') }}</Button>
        </div>
      </Card>

      <Card>
        <div v-if="loading" role="status">{{ t('common.loading') }}</div>
        <EmptyState
          v-else-if="permissionDenied"
          :title="t('carrierTenders.permissionDenied')"
        />
        <EmptyState
          v-else-if="loadFailed"
          :title="t('carrierTenders.loadFailed')"
        />
        <EmptyState
          v-else-if="items.length === 0"
          :title="t('carrierTenders.empty')"
          :description="t('carrierTenders.emptyHint')"
        />
        <Table
          :columns="[
            t('carrierTenders.columns.reference'),
            t('carrierTenders.columns.title'),
            t('carrierTenders.columns.buyer'),
            t('carrierTenders.columns.status'),
            t('carrierTenders.columns.deadline'),
            t('carrierTenders.columns.lots'),
            t('carrierTenders.columns.ownResponse'),
            t('common.actions'),
          ]"
          :loading="loading"
        >
          <tr v-for="item in items" :key="item.id">
            <td>
              <NuxtLink :to="`/carrier/tenders/${item.id}`" class="link">{{ item.rfx_number }}</NuxtLink>
            </td>
            <td>{{ item.title }}</td>
            <td>{{ companyName(item.owner_company_id) }}</td>
            <td><Badge :status="item.status" /></td>
            <td>
              <span :class="{ 'text-danger': isDeadlineExpired(item.response_deadline) }">
                {{ formatRfxDate(item.response_deadline) }}
              </span>
              <span v-if="formatDeadlineRemaining(item.response_deadline)" class="muted">
                ({{ formatDeadlineRemaining(item.response_deadline) }})
              </span>
            </td>
            <td>{{ item.lot_count ?? 0 }}</td>
            <td>{{ responseStatusLabel(item.own_response_status) }}</td>
            <td>
              <NuxtLink :to="`/carrier/tenders/${item.id}`">{{ t('common.details') }}</NuxtLink>
            </td>
          </tr>
        </Table>

        <div v-if="items.length > 0" class="pagination">
          <Button variant="secondary" :disabled="!canGoPrev" @click="goPrev">{{ t('common.back') }}</Button>
          <span class="muted">{{ pagination.offset + 1 }}–{{ Math.min(pagination.offset + pagination.limit, total) }} / {{ total }}</span>
          <Button variant="secondary" :disabled="!canGoNext" @click="goNext">{{ t('common.next') }}</Button>
        </div>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.filters {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  align-items: end;
}

.pagination {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 1rem;
}

.link {
  color: var(--color-primary);
  font-weight: 600;
  text-decoration: none;
}

.muted {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.text-danger {
  color: var(--color-danger);
}
</style>
