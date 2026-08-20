<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { PaymentActor, PaymentRecord } from '~/types/payment'
import { PAYMENT_STATUSES } from '~/types/payment'
import { formatPaymentMoney, paymentStatusLabelKey, resolvePaymentActor } from '~/utils/payment'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listPayments } = usePaymentsApi()
const { getUserCompanies } = useCompanies()
const { currentCompanyId } = useTenantContext()
const { canReadPayments } = usePermissions()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(true)
const apiUnavailable = ref(false)
const items = ref<PaymentRecord[]>([])
const total = ref(0)
const memberships = ref<UserCompanyMembership[]>([])
const actor = ref<PaymentActor | null>(null)
const pagination = reactive({ limit: 20, offset: 0 })
const filters = reactive({
  q: '',
  status: '',
  currency_code: '',
  from_date: '',
  to_date: '',
})

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)
const missingCompany = computed(() => !currentCompanyId.value)
const missingActor = computed(() => !actor.value)
const accessDenied = computed(() => !canReadPayments())

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...PAYMENT_STATUSES.map((status) => ({ label: t(paymentStatusLabelKey(status)), value: status })),
])

let searchTimer: ReturnType<typeof setTimeout> | undefined

async function loadMemberships() {
  if (!authStore.user?.id) {
    memberships.value = []
    actor.value = null
    return
  }
  memberships.value = await getUserCompanies(authStore.user.id)
  actor.value = resolvePaymentActor(currentCompanyId.value, memberships.value)
}

async function loadPayments() {
  loading.value = true
  apiUnavailable.value = false
  try {
    await loadMemberships()
    if (!currentCompanyId.value || !actor.value || accessDenied.value) {
      items.value = []
      total.value = 0
      return
    }
    const data = await listPayments({
      q: filters.q || undefined,
      status: filters.status || undefined,
      currency_code: filters.currency_code || undefined,
      from_date: filters.from_date || undefined,
      to_date: filters.to_date || undefined,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items
    total.value = data.total
  } catch (error) {
    items.value = []
    total.value = 0
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('payments.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  loadPayments()
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(onFiltersChange, 350)
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadPayments()
}

function goNext() {
  pagination.offset += pagination.limit
  loadPayments()
}

watch(currentCompanyId, () => {
  pagination.offset = 0
  loadPayments()
}, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('payments.title')" />

    <Card v-if="!accessDenied && !missingCompany && !missingActor">
      <div class="filters-row">
        <Input
          v-model="filters.q"
          :label="t('payments.search')"
          :placeholder="t('payments.searchPlaceholder')"
          @input="onSearchInput"
        />
        <Select
          v-model="filters.status"
          :label="t('payments.statusFilter')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <Input
          v-model="filters.currency_code"
          :label="t('payments.currencyFilter')"
          @change="onFiltersChange"
        />
        <Input
          v-model="filters.from_date"
          type="date"
          :label="t('payments.fromDate')"
          @change="onFiltersChange"
        />
        <Input
          v-model="filters.to_date"
          type="date"
          :label="t('payments.toDate')"
          @change="onFiltersChange"
        />
      </div>
    </Card>

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="accessDenied" :title="t('payments.missingActor')" />
    <EmptyState v-else-if="missingCompany" :title="t('payments.missingCompany')" />
    <EmptyState v-else-if="missingActor" :title="t('payments.missingActor')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('payments.loadFailed')" />
    <EmptyState v-else-if="!hasItems" :title="t('payments.empty')" />

    <Card v-else>
      <div class="table-scroll">
        <Table
          :columns="[
            t('payments.paymentNumber'),
            t('common.status'),
            t('payments.amount'),
            t('payments.allocated'),
            t('payments.unallocated'),
            t('payments.paymentDate'),
            t('common.actions'),
          ]"
          :loading="loading"
        >
          <tr v-for="item in items" :key="item.id">
            <td>
              <NuxtLink :to="`/payments/${item.id}`" class="link">{{ item.payment_number }}</NuxtLink>
              <div v-if="item.external_id" class="muted">{{ item.external_id }}</div>
            </td>
            <td><Badge :status="t(paymentStatusLabelKey(item.status))" /></td>
            <td>{{ formatPaymentMoney(item.amount, item.currency_code) }}</td>
            <td>{{ formatPaymentMoney(item.allocated_amount, item.currency_code) }}</td>
            <td>{{ formatPaymentMoney(item.unallocated_amount, item.currency_code) }}</td>
            <td>{{ item.payment_date }}</td>
            <td>
              <NuxtLink :to="`/payments/${item.id}`">{{ t('payments.actions.view') }}</NuxtLink>
            </td>
          </tr>
        </Table>
      </div>
      <div class="pagination-row">
        <Button variant="secondary" :disabled="!canGoPrev" @click="goPrev">{{ t('common.back') }}</Button>
        <span>{{ pagination.offset + 1 }}–{{ Math.min(pagination.offset + pagination.limit, total) }} / {{ total }}</span>
        <Button variant="secondary" :disabled="!canGoNext" @click="goNext">{{ t('common.next') }}</Button>
      </div>
    </Card>
  </div>
</template>

<style scoped>
.filters-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1rem;
}
.pagination-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 1rem;
  margin-top: 1rem;
}
.link {
  color: var(--color-primary);
  text-decoration: none;
  font-weight: 500;
}
.muted {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}
</style>
