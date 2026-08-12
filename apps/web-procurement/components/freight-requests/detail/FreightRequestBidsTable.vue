<script setup lang="ts">
import type { Bid } from '~/types/bid'
import { formatMoney } from '~/utils/format'

const props = defineProps<{
  freightRequestId: string
}>()

const emit = defineEmits<{
  updated: []
}>()

const {
  listFreightRequestBids,
  isUnauthorizedError,
  isNotFoundError,
  isServerError,
  isNetworkError,
} = useFreightRequestDetailApi()
const {
  acceptBid,
  isUnauthorizedError: isAcceptUnauthorizedError,
  isNotFoundError: isAcceptNotFoundError,
  isServerError: isAcceptServerError,
  isNetworkError: isAcceptNetworkError,
  isConflictError,
} = useBidsApi()
const { logout } = useAuth()
const { t } = useI18n()

type BidsViewState = 'loading' | 'ready' | 'unauthorized' | 'not-found' | 'server-error' | 'network-error' | 'error'

const bids = ref<Bid[]>([])
const viewState = ref<BidsViewState>('loading')
const errorMessage = ref('')
const actionLoadingId = ref<string | null>(null)
const actionMessage = ref('')
const actionMessageKind = ref<'success' | 'error'>('success')

async function loadBids() {
  viewState.value = 'loading'
  errorMessage.value = ''
  actionMessage.value = ''

  try {
    bids.value = await listFreightRequestBids(props.freightRequestId)
    viewState.value = 'ready'
  } catch (error) {
    bids.value = []
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

function acceptErrorMessage(error: unknown): string {
  if (isAcceptUnauthorizedError(error)) return t('freightRequests.detail.acceptUnauthorized')
  if (isAcceptNotFoundError(error)) return t('freightRequests.detail.acceptNotFound')
  if (isConflictError(error)) return t('freightRequests.detail.acceptConflict')
  if (isAcceptServerError(error)) return t('freightRequests.detail.acceptServerError')
  if (isAcceptNetworkError(error)) return t('freightRequests.detail.acceptNetworkError')
  return error instanceof Error ? error.message : t('common.error')
}

async function handleAccept(bid: Bid) {
  if (!window.confirm(t('freightRequests.detail.acceptConfirm'))) return

  actionLoadingId.value = bid.id
  actionMessage.value = ''
  actionMessageKind.value = 'error'

  try {
    await acceptBid(bid.id)
    actionMessageKind.value = 'success'
    actionMessage.value = t('freightRequests.detail.acceptSuccess')
    await loadBids()
    emit('updated')
  } catch (error) {
    if (isAcceptUnauthorizedError(error)) {
      await logout()
      return
    }
    actionMessageKind.value = 'error'
    actionMessage.value = acceptErrorMessage(error)
  } finally {
    actionLoadingId.value = null
  }
}

watch(() => props.freightRequestId, loadBids, { immediate: true })
</script>

<template>
  <section class="panel">
    <h3>{{ $t('freightRequests.detail.bidsTitle') }}</h3>

    <div v-if="actionMessage" class="action-message" :class="`action-message--${actionMessageKind}`">
      {{ actionMessage }}
    </div>

    <div v-if="viewState === 'loading'" class="state-card">{{ $t('common.loading') }}</div>
    <div v-else-if="viewState === 'unauthorized'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.unauthorized') }}
    </div>
    <div v-else-if="viewState === 'not-found'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.notFound') }}
    </div>
    <div v-else-if="viewState === 'server-error'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.bidsLoadError') }}
    </div>
    <div v-else-if="viewState === 'network-error'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.networkError') }}
    </div>
    <div v-else-if="viewState === 'error'" class="state-card state-card--error">
      {{ errorMessage || $t('freightRequests.detail.bidsLoadError') }}
    </div>

    <UiEmptyState v-else-if="!bids.length" :title="$t('freightRequests.detail.bidsEmpty')" />

    <div v-else class="table-wrap">
      <table class="bids-table">
        <thead>
          <tr>
            <th>{{ $t('freightRequests.detail.bidNumber') }}</th>
            <th>{{ $t('freightRequests.detail.carrierCompanyId') }}</th>
            <th>{{ $t('common.status') }}</th>
            <th>{{ $t('freightRequests.detail.totalAmount') }}</th>
            <th>{{ $t('freightRequests.detail.currency') }}</th>
            <th>{{ $t('freightRequests.detail.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="bid in bids" :key="bid.id">
            <td>
              <NuxtLink :to="`/bids/${bid.id}`" class="bid-link">
                {{ bid.bid_number }}
              </NuxtLink>
            </td>
            <td>{{ bid.carrier_company_id }}</td>
            <td>
              <span class="status-badge" :class="`status-badge--${bid.status.toLowerCase()}`">
                {{ bid.status }}
              </span>
            </td>
            <td>{{ formatMoney(bid.total_amount, bid.currency_code) }}</td>
            <td>{{ bid.currency_code || '—' }}</td>
            <td class="actions-cell">
              <UiButton :to="`/bids/${bid.id}`" variant="secondary">
                {{ $t('freightRequests.detail.viewBid') }}
              </UiButton>
              <UiButton
                v-if="bid.status === 'SUBMITTED'"
                :disabled="actionLoadingId === bid.id"
                @click="handleAccept(bid)"
              >
                {{ $t('freightRequests.detail.acceptBid') }}
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
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

.action-message {
  margin-bottom: 1rem;
  padding: 0.75rem 1rem;
  border-radius: 0.375rem;
  font-size: 0.875rem;
}

.action-message--success {
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  color: #065f46;
}

.action-message--error {
  background: #fef2f2;
  border: 1px solid #fecaca;
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

.table-wrap {
  overflow-x: auto;
}

.bids-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.bids-table th,
.bids-table td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid #e5e7eb;
  text-align: left;
  vertical-align: middle;
}

.bids-table th {
  color: #6b7280;
  font-weight: 600;
}

.bid-link {
  color: #2563eb;
  text-decoration: none;
}

.bid-link:hover {
  text-decoration: underline;
}

.actions-cell {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
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
</style>
