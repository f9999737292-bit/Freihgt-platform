<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()
const config = useRuntimeConfig()

interface HealthResponse {
  status: string
  service: string
  version?: string
  uptime_seconds?: number
  timestamp?: string
}

interface ReadyResponse {
  status: string
  services?: Record<string, string>
}

interface ServiceRow {
  name: string
  status: string
  lastChecked: string
  url: string
}

const loading = ref(false)
const gatewayUnavailable = ref(false)
const gatewayHealth = ref<HealthResponse | null>(null)
const gatewayReady = ref<ReadyResponse | null>(null)
const serviceRows = ref<ServiceRow[]>([])

const serviceUrls: Record<string, string> = {
  'identity-service': 'http://localhost:8081',
  'company-service': 'http://localhost:8082',
  'transport-order-service': 'http://localhost:8083',
  'rfx-service': 'http://localhost:8084',
  'shipment-service': 'http://localhost:8085',
  'document-service': 'http://localhost:8086',
  'billing-register-service': 'http://localhost:8087',
}

function statusTone(status: string): 'success' | 'danger' | 'neutral' {
  if (status === 'ok' || status === 'ready') return 'success'
  if (status === 'down' || status === 'not_ready') return 'danger'
  return 'neutral'
}

function formatStatus(status: string) {
  if (status === 'ok' || status === 'ready') return 'ok'
  if (status === 'down' || status === 'not_ready') return 'down'
  return 'unknown'
}

async function refresh() {
  loading.value = true
  gatewayUnavailable.value = false
  const base = config.public.apiBaseUrl.replace(/\/$/, '')
  const checkedAt = new Date().toLocaleString()

  try {
    const [healthRes, readyRes] = await Promise.all([
      fetch(`${base}/health`, { headers: { Accept: 'application/json' } }),
      fetch(`${base}/ready`, { headers: { Accept: 'application/json' } }),
    ])

    if (!healthRes.ok && !readyRes.ok) {
      gatewayUnavailable.value = true
      gatewayHealth.value = null
      gatewayReady.value = null
      serviceRows.value = []
      return
    }

    gatewayHealth.value = healthRes.ok ? ((await healthRes.json()) as HealthResponse) : null

    try {
      gatewayReady.value = (await readyRes.json()) as ReadyResponse
    } catch {
      gatewayReady.value = null
    }

    const services = gatewayReady.value?.services ?? {}
    serviceRows.value = Object.entries(services).map(([name, status]) => ({
      name,
      status: formatStatus(status),
      lastChecked: checkedAt,
      url: serviceUrls[name] ?? `${base}`,
    }))
  } catch {
    gatewayUnavailable.value = true
    gatewayHealth.value = null
    gatewayReady.value = null
    serviceRows.value = []
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('health.title')">
      <template #actions>
        <UiButton variant="secondary" :disabled="loading" @click="refresh">
          {{ $t('health.refresh') }}
        </UiButton>
        <a href="http://localhost:8080/docs" target="_blank" rel="noopener noreferrer">
          <UiButton variant="secondary">{{ $t('health.openSwagger') }}</UiButton>
        </a>
        <a href="http://localhost:9090" target="_blank" rel="noopener noreferrer">
          <UiButton variant="secondary">{{ $t('health.openPrometheus') }}</UiButton>
        </a>
        <a href="http://localhost:3001" target="_blank" rel="noopener noreferrer">
          <UiButton variant="secondary">{{ $t('health.openGrafana') }}</UiButton>
        </a>
      </template>
    </UiPageHeader>

    <UiCard v-if="gatewayUnavailable">
      <UiEmptyState :title="$t('health.gatewayUnavailable')" />
    </UiCard>

    <template v-else>
      <div class="health-grid">
        <UiCard>
          <template #header>API Gateway /health</template>
          <div v-if="gatewayHealth" class="health-meta">
            <UiBadge :status="formatStatus(gatewayHealth.status)" :tone="statusTone(gatewayHealth.status)" />
            <span>{{ gatewayHealth.service }} · v{{ gatewayHealth.version }} · {{ gatewayHealth.uptime_seconds }}s</span>
          </div>
          <span v-else class="muted">{{ $t('common.loading') }}</span>
        </UiCard>

        <UiCard>
          <template #header>API Gateway /ready</template>
          <div v-if="gatewayReady" class="health-meta">
            <UiBadge :status="formatStatus(gatewayReady.status)" :tone="statusTone(gatewayReady.status)" />
          </div>
          <span v-else class="muted">{{ $t('common.loading') }}</span>
        </UiCard>
      </div>

      <UiCard>
        <template #header>{{ $t('health.serviceStatus') }}</template>
        <UiTable
          :columns="['Service', $t('common.status'), $t('health.lastChecked'), 'URL', $t('common.actions')]"
          :loading="loading"
        >
          <tr v-for="row in serviceRows" :key="row.name">
            <td>{{ row.name }}</td>
            <td>
              <UiBadge :status="row.status" :tone="statusTone(row.status)" />
            </td>
            <td>{{ row.lastChecked }}</td>
            <td>
              <a :href="row.url" target="_blank" rel="noopener noreferrer">{{ row.url }}</a>
            </td>
            <td>
              <a :href="`${row.url}/health`" target="_blank" rel="noopener noreferrer">
                <UiButton size="sm" variant="secondary">/health</UiButton>
              </a>
            </td>
          </tr>
        </UiTable>
        <UiEmptyState v-if="!loading && !serviceRows.length" :title="$t('common.empty')" />
      </UiCard>

      <UiCard>
        <template #header>{{ $t('health.databaseMetrics') }}</template>
        <p class="health-db-desc">{{ $t('health.databaseMetricsHint') }}</p>
        <div class="health-db-actions">
          <a
            href="http://localhost:9090/graph?g0.expr=db_query_duration_seconds_count&g0.tab=0"
            target="_blank"
            rel="noopener noreferrer"
          >
            <UiButton variant="secondary">{{ $t('health.openPrometheusDbMetrics') }}</UiButton>
          </a>
          <a
            href="http://localhost:9090/graph?g0.expr=sum(rate(db_query_duration_seconds_count%7Bstatus%3D%22error%22%7D%5B5m%5D))%20by%20(service%2C%20operation)&g0.tab=0"
            target="_blank"
            rel="noopener noreferrer"
          >
            <UiButton variant="secondary">{{ $t('health.openPrometheusDbErrors') }}</UiButton>
          </a>
          <a
            href="http://localhost:3001/d/freight-platform-overview/freight-platform-overview"
            target="_blank"
            rel="noopener noreferrer"
          >
            <UiButton variant="secondary">{{ $t('health.openGrafanaDbDashboard') }}</UiButton>
          </a>
        </div>
      </UiCard>
    </template>
  </div>
</template>

<style scoped>
.health-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1rem;
}

.health-meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.muted {
  color: var(--color-text-muted, #64748b);
}

.health-db-desc {
  margin: 0 0 1rem;
  color: var(--color-text-muted, #64748b);
}

.health-db-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
</style>
