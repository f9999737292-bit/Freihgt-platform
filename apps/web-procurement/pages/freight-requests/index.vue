<script setup lang="ts">
import type { FreightRequest } from '~/types/rfx'

definePageMeta({ middleware: 'auth' })

const {
  listFreightRequests,
  isUnauthorizedError,
  isServerError,
  isNetworkError,
} = useFreightRequestsListApi()
const { logout } = useAuth()
const { t } = useI18n()

type ViewState = 'loading' | 'unauthorized' | 'server-error' | 'network-error' | 'error' | 'ready'

const items = ref<FreightRequest[]>([])
const total = ref(0)
const viewState = ref<ViewState>('loading')
const errorMessage = ref('')

const filters = reactive({
  request_type: '',
  status: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

async function load() {
  viewState.value = 'loading'
  errorMessage.value = ''

  try {
    const data = await listFreightRequests({
      request_type: filters.request_type || undefined,
      status: filters.status || undefined,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items
    total.value = data.total ?? data.items.length
    viewState.value = 'ready'
  } catch (error) {
    items.value = []
    total.value = 0

    if (isUnauthorizedError(error)) {
      viewState.value = 'unauthorized'
      await logout()
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

function onFiltersChange() {
  pagination.offset = 0
  load()
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  load()
}

function goNext() {
  pagination.offset += pagination.limit
  load()
}

onMounted(load)
</script>

<template>
  <div class="fr-list-page">
    <UiPageHeader :title="$t('freightRequests.list.title')" />

    <UiCard>
      <FreightRequestsListFilters
        v-model:request-type="filters.request_type"
        v-model:status="filters.status"
        @change="onFiltersChange"
      />
    </UiCard>

    <div v-if="viewState === 'loading'" class="state-card">{{ $t('common.loading') }}</div>

    <UiEmptyState
      v-else-if="viewState === 'unauthorized'"
      :title="$t('freightRequests.list.unauthorized')"
    />
    <UiEmptyState
      v-else-if="viewState === 'server-error'"
      :title="$t('freightRequests.list.serverError')"
    />
    <UiEmptyState
      v-else-if="viewState === 'network-error'"
      :title="$t('freightRequests.list.networkError')"
    />
    <UiEmptyState
      v-else-if="viewState === 'error'"
      :title="errorMessage || $t('freightRequests.list.loadFailed')"
    />
    <UiEmptyState
      v-else-if="viewState === 'ready' && !hasItems"
      :title="$t('freightRequests.list.noRequestsFound')"
    />

    <UiCard v-else-if="viewState === 'ready' && hasItems" class="fr-list-page__table-card">
      <FreightRequestsListTable :items="items" />
      <FreightRequestsListPagination
        :total="total"
        :can-go-prev="canGoPrev"
        :can-go-next="canGoNext"
        @prev="goPrev"
        @next="goNext"
      />
    </UiCard>
  </div>
</template>

<style scoped>
.fr-list-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.fr-list-page__table-card {
  padding: 0;
  overflow: hidden;
}

.state-card {
  padding: 1rem 1.25rem;
  border-radius: 0.5rem;
  background: #fff;
  border: 1px solid #e5e7eb;
  color: #6b7280;
}
</style>
