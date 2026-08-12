<script setup lang="ts">
import type { FreightRequest } from '~/types/rfx'
import { isValidBidId } from '~/utils/format'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const {
  getFreightRequest,
  isUnauthorizedError,
  isNotFoundError,
  isServerError,
  isNetworkError,
} = useFreightRequestDetailApi()
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

const request = ref<FreightRequest | null>(null)
const viewState = ref<ViewState>('loading')
const errorMessage = ref('')

const requestId = computed(() => String(route.params.id ?? '').trim())
const bidsTableKey = ref(0)

async function loadRequest() {
  if (!isValidBidId(requestId.value)) {
    request.value = null
    viewState.value = 'invalid-id'
    errorMessage.value = ''
    return
  }

  viewState.value = 'loading'
  errorMessage.value = ''
  request.value = null

  try {
    request.value = await getFreightRequest(requestId.value)
    viewState.value = 'ready'
  } catch (error) {
    request.value = null
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

async function handleUpdated() {
  await loadRequest()
  bidsTableKey.value += 1
}

watch(requestId, loadRequest, { immediate: true })
</script>

<template>
  <div class="freight-request-page">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/">{{ $t('common.home') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('freightRequests.detail.title') }}</span>
    </nav>

    <header class="freight-request-page__header">
      <div>
        <h2>{{ request?.freight_request_number || $t('freightRequests.detail.title') }}</h2>
      </div>
      <span
        v-if="request"
        class="status-badge"
        :class="`status-badge--${request.status.toLowerCase()}`"
      >
        {{ request.status }}
      </span>
    </header>

    <div v-if="viewState === 'loading'" class="state-card">{{ $t('common.loading') }}</div>
    <div v-else-if="viewState === 'invalid-id'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.invalidId') }}
    </div>
    <div v-else-if="viewState === 'unauthorized'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.unauthorized') }}
    </div>
    <div v-else-if="viewState === 'not-found'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.notFound') }}
    </div>
    <div v-else-if="viewState === 'server-error'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.serverError') }}
    </div>
    <div v-else-if="viewState === 'network-error'" class="state-card state-card--error">
      {{ $t('freightRequests.detail.networkError') }}
    </div>
    <div v-else-if="viewState === 'error'" class="state-card state-card--error">
      {{ errorMessage || $t('common.error') }}
    </div>

    <template v-else-if="request">
      <FreightRequestsDetailFreightRequestSummary :request="request" />
      <FreightRequestsDetailFreightRequestBidsTable
        :key="bidsTableKey"
        :freight-request-id="request.id"
        @updated="handleUpdated"
      />
    </template>
  </div>
</template>

<style scoped>
.freight-request-page {
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

.freight-request-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.freight-request-page__header h2 {
  margin: 0;
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

.status-badge--published,
.status-badge--responses_open {
  background: #dbeafe;
  color: #1e40af;
}

.status-badge--awarded {
  background: #dcfce7;
  color: #166534;
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
</style>
