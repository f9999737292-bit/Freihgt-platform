<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()
const uiStore = useUiStore()
const config = useRuntimeConfig()
const tenantStore = useTenantStore()
const { apiGet, isApiUnavailableError } = useApi()
const { backendOnline, backendStatus, checkBackendStatus } = useBackendStatus()

const checkingBackend = ref(false)

const stats = ref({
  companies: 0,
  users: 0,
  transportOrders: 0,
  rfx: 0,
  shipments: 0,
  documents: 0,
  billingRegisters: 0,
})

const unavailableKeys = ref<Set<string>>(new Set())
const loading = ref(true)

const apiUnavailable = computed(
  () => unavailableKeys.value.size > 0 && unavailableKeys.value.size === 7,
)
const apiPartialUnavailable = computed(
  () => unavailableKeys.value.size > 0 && unavailableKeys.value.size < 7,
)

const cards = computed(() => [
  { key: 'companies', label: t('nav.companies'), value: stats.value.companies, to: '/companies' },
  { key: 'users', label: t('nav.users'), value: stats.value.users, to: '/users' },
  { key: 'transportOrders', label: t('nav.transportOrders'), value: stats.value.transportOrders, to: '/transport-orders' },
  { key: 'rfx', label: t('nav.rfx'), value: stats.value.rfx, to: '/rfx' },
  { key: 'shipments', label: t('nav.shipments'), value: stats.value.shipments, to: '/shipments' },
  { key: 'documents', label: t('nav.documents'), value: stats.value.documents, to: '/documents' },
  { key: 'billingRegisters', label: t('nav.billingRegisters'), value: stats.value.billingRegisters, to: '/billing-registers' },
])

function cardValue(key: string, value: number) {
  return unavailableKeys.value.has(key) ? '—' : value
}

async function loadCounts() {
  if (!tenantStore.tenantId) {
    loading.value = false
    return
  }

  loading.value = true
  unavailableKeys.value = new Set()

  const query = { tenant_id: tenantStore.tenantId, limit: 1, offset: 0 }
  const endpoints = [
    ['companies', '/api/v1/companies'],
    ['users', '/api/v1/users'],
    ['transportOrders', '/api/v1/transport-orders'],
    ['rfx', '/api/v1/rfx-events'],
    ['shipments', '/api/v1/shipments'],
    ['documents', '/api/v1/documents'],
    ['billingRegisters', '/api/v1/billing-registers'],
  ] as const

  const results = await Promise.allSettled(
    endpoints.map(async ([key, path]) => {
      const data = await apiGet<{ total: number }>(path, { query })
      return { key, total: data.total ?? 0 }
    }),
  )

  for (let i = 0; i < results.length; i++) {
    const result = results[i]
    const key = endpoints[i][0]
    if (result.status === 'fulfilled') {
      stats.value[result.value.key] = result.value.total
      continue
    }
    stats.value[key] = 0
    if (isApiUnavailableError(result.reason)) {
      unavailableKeys.value = new Set([...unavailableKeys.value, key])
    }
  }

  loading.value = false
}

async function onRefreshBackendStatus() {
  checkingBackend.value = true
  try {
    await checkBackendStatus()
    await loadCounts()
  } finally {
    checkingBackend.value = false
  }
}

onMounted(async () => {
  await checkBackendStatus()
  await loadCounts()
})
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('dashboard.title')" />

    <div
      v-if="!backendOnline && backendStatus !== 'checking'"
      class="dashboard-backend-offline"
    >
      <p>{{ $t('dashboard.backendOffline') }}</p>
      <UiButton size="sm" variant="secondary" :loading="checkingBackend" @click="onRefreshBackendStatus">
        {{ $t('backendStatus.refresh') }}
      </UiButton>
    </div>

    <CommonApiUnavailableState v-if="apiUnavailable" @retry="loadCounts" />

    <p v-else-if="apiPartialUnavailable" class="dashboard-warning">
      {{ $t('common.apiUnavailable') }} — {{ $t('common.apiUnavailableHint') }}
      <UiButton size="sm" variant="secondary" @click="loadCounts">{{ $t('common.refresh') }}</UiButton>
    </p>

    <div class="dashboard-grid">
      <NuxtLink v-for="card in cards" :key="card.to" :to="card.to" class="dashboard-card">
        <span class="dashboard-card__label">{{ card.label }}</span>
        <strong class="dashboard-card__value">{{ cardValue(card.key, card.value) }}</strong>
      </NuxtLink>
    </div>

    <div class="dashboard-meta">
      <UiCard>
        <template #header>{{ $t('dashboard.gatewayStatus') }}</template>
        <UiBadge :status="uiStore.apiGatewayStatus === 'online' ? 'ACTIVE' : 'CANCELLED'" />
      </UiCard>
      <UiCard>
        <template #header>{{ $t('dashboard.smokeTest') }}</template>
        <span>{{ uiStore.lastSmokeTestStatus }}</span>
      </UiCard>
      <UiCard>
        <template #header>{{ $t('dashboard.environment') }}</template>
        <span>{{ $t('dashboard.environmentLocal') }} · {{ config.public.apiBaseUrl }}</span>
      </UiCard>
    </div>
  </div>
</template>

<style scoped>
.dashboard-backend-offline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  background: #fffbeb;
  border: 1px solid #fde68a;
  color: #92400e;
  font-size: 0.875rem;
}

.dashboard-backend-offline p {
  margin: 0;
  flex: 1;
}

.dashboard-warning {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  margin: 0;
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  background: #fffbeb;
  color: #92400e;
  font-size: 0.875rem;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 1rem;
}

.dashboard-card {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1.25rem;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  text-decoration: none;
  color: inherit;
  box-shadow: var(--shadow-sm);
}

.dashboard-card:hover {
  border-color: var(--color-primary);
  text-decoration: none;
}

.dashboard-card__label {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.dashboard-card__value {
  font-size: 1.75rem;
}

.dashboard-meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
}
</style>
