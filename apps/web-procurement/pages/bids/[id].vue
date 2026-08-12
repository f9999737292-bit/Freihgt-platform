<script setup lang="ts">
import type { Bid } from '~/types/bid'
import { formatDateTime, formatMoney, isValidBidId } from '~/utils/format'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const { getBid, isUnauthorizedError, isNotFoundError, isServerError, isNetworkError } = useBidsApi()
const { logout } = useAuth()
const { t } = useI18n()

type ViewState =
  | 'loading'
  | 'invalid-id'
  | 'unauthorized'
  | 'not-found'
  | 'server-error'
  | 'network-error'
  | 'ready'
  | 'error'

const bid = ref<Bid | null>(null)
const viewState = ref<ViewState>('loading')
const errorMessage = ref('')

const bidId = computed(() => String(route.params.id ?? '').trim())

async function loadBid() {
  if (!isValidBidId(bidId.value)) {
    bid.value = null
    viewState.value = 'invalid-id'
    errorMessage.value = ''
    return
  }

  viewState.value = 'loading'
  errorMessage.value = ''
  bid.value = null

  try {
    bid.value = await getBid(bidId.value)
    viewState.value = 'ready'
  } catch (error) {
    bid.value = null
    if (isUnauthorizedError(error)) {
      viewState.value = 'unauthorized'
      await logout()
      return
    }
    if (isNotFoundError(error)) {
      viewState.value = 'not-found'
      return
    }
    if (isServerError(error)) {
      viewState.value = 'server-error'
      return
    }
    if (isNetworkError(error)) {
      viewState.value = 'network-error'
      return
    }
    viewState.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  }
}

watch(bidId, loadBid, { immediate: true })
</script>

<template>
  <div class="bid-page">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/">{{ $t('common.home') }}</NuxtLink>
      <template v-if="bid?.freight_request_id">
        <span class="breadcrumbs__sep">/</span>
        <NuxtLink :to="`/freight-requests/${bid.freight_request_id}`">
          {{ $t('freightRequests.detail.backToFreightRequest') }}
        </NuxtLink>
      </template>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('bid.title') }}</span>
    </nav>

    <header class="bid-page__header">
      <div>
        <h2>{{ bid?.bid_number || $t('bid.title') }}</h2>
        <p v-if="viewState === 'ready'" class="bid-page__subtitle">{{ $t('bid.readOnlyHint') }}</p>
      </div>
      <span v-if="bid" class="status-badge" :class="`status-badge--${bid.status.toLowerCase()}`">
        {{ bid.status }}
      </span>
    </header>

    <div v-if="viewState === 'loading'" class="state-card">{{ $t('common.loading') }}</div>
    <div v-else-if="viewState === 'invalid-id'" class="state-card state-card--error">
      {{ $t('bid.invalidId') }}
    </div>
    <div v-else-if="viewState === 'unauthorized'" class="state-card state-card--error">
      {{ $t('bid.unauthorized') }}
    </div>
    <div v-else-if="viewState === 'not-found'" class="state-card state-card--error">
      {{ $t('bid.notFound') }}
    </div>
    <div v-else-if="viewState === 'server-error'" class="state-card state-card--error">
      {{ $t('bid.serverError') }}
    </div>
    <div v-else-if="viewState === 'network-error'" class="state-card state-card--error">
      {{ $t('bid.networkError') }}
    </div>
    <div v-else-if="viewState === 'error'" class="state-card state-card--error">
      {{ errorMessage || $t('common.error') }}
    </div>

    <template v-else-if="bid">
      <section class="panel">
        <h3>{{ $t('bid.summary') }}</h3>
        <dl class="details-grid">
          <div>
            <dt>{{ $t('bid.bidNumber') }}</dt>
            <dd>{{ bid.bid_number }}</dd>
          </div>
          <div>
            <dt>{{ $t('common.status') }}</dt>
            <dd>{{ bid.status }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.carrierCompanyId') }}</dt>
            <dd>{{ bid.carrier_company_id }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.totalAmount') }}</dt>
            <dd>{{ formatMoney(bid.total_amount, bid.currency_code) }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.currency') }}</dt>
            <dd>{{ bid.currency_code || '—' }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.vatRate') }}</dt>
            <dd>{{ bid.vat_rate != null ? `${bid.vat_rate}%` : '—' }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.vatAmount') }}</dt>
            <dd>{{ formatMoney(bid.vat_amount, bid.currency_code) }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.totalAmountWithVat') }}</dt>
            <dd>{{ formatMoney(bid.total_amount_with_vat, bid.currency_code) }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.validUntil') }}</dt>
            <dd>{{ formatDateTime(bid.valid_until) }}</dd>
          </div>
          <div>
            <dt>{{ $t('bid.submittedAt') }}</dt>
            <dd>{{ formatDateTime(bid.submitted_at) }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="bid.items?.length" class="panel">
        <h3>{{ $t('bid.lineItems') }}</h3>
        <div class="table-wrap">
          <table class="items-table">
            <thead>
              <tr>
                <th>{{ $t('bid.description') }}</th>
                <th>{{ $t('bid.baseAmount') }}</th>
                <th>{{ $t('bid.fuelSurcharge') }}</th>
                <th>{{ $t('bid.tollAmount') }}</th>
                <th>{{ $t('bid.extraCharges') }}</th>
                <th>{{ $t('bid.amountWithoutVat') }}</th>
                <th>{{ $t('bid.vatAmount') }}</th>
                <th>{{ $t('bid.amountWithVat') }}</th>
                <th>{{ $t('bid.comment') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, index) in bid.items" :key="item.id || index">
                <td>{{ item.description || '—' }}</td>
                <td>{{ formatMoney(item.base_amount, bid.currency_code) }}</td>
                <td>{{ formatMoney(item.fuel_surcharge, bid.currency_code) }}</td>
                <td>{{ formatMoney(item.toll_amount, bid.currency_code) }}</td>
                <td>{{ formatMoney(item.extra_charges, bid.currency_code) }}</td>
                <td>{{ formatMoney(item.amount_without_vat, bid.currency_code) }}</td>
                <td>{{ formatMoney(item.vat_amount, bid.currency_code) }}</td>
                <td>{{ formatMoney(item.amount_with_vat, bid.currency_code) }}</td>
                <td>{{ item.comment || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.bid-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: #6b7280;
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.bid-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.bid-page__header h2 {
  margin: 0;
}

.bid-page__subtitle {
  margin: 0.375rem 0 0;
  color: #6b7280;
  font-size: 0.875rem;
}

.status-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.status-badge--draft {
  background: #eef2f6;
  color: #475569;
}

.status-badge--submitted {
  background: #dbeafe;
  color: #1e40af;
}

.status-badge--accepted {
  background: #dcfce7;
  color: #166534;
}

.status-badge--rejected {
  background: #fee2e2;
  color: #991b1b;
}

.state-card {
  padding: 1rem;
  border-radius: 0.5rem;
  background: #fff;
  border: 1px solid #e5e7eb;
}

.state-card--error {
  border-color: #fecaca;
  background: #fef2f2;
  color: #991b1b;
}

.panel {
  padding: 1rem 1.25rem;
  border-radius: 0.5rem;
  background: #fff;
  border: 1px solid #e5e7eb;
}

.panel h3 {
  margin: 0 0 1rem;
  font-size: 1rem;
}

.details-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem 1.5rem;
  margin: 0;
}

.details-grid div {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.details-grid dt {
  margin: 0;
  font-size: 0.8125rem;
  color: #6b7280;
}

.details-grid dd {
  margin: 0;
}

.table-wrap {
  overflow-x: auto;
}

.items-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.items-table th,
.items-table td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid #e5e7eb;
  text-align: left;
  vertical-align: top;
}

.items-table th {
  color: #6b7280;
  font-weight: 600;
}

@media (max-width: 768px) {
  .details-grid {
    grid-template-columns: 1fr;
  }
}
</style>
