<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const {
  loading,
  kpiCards,
  operationsRows,
  transportFunnel,
  transportFunnelEmpty,
  tenderFunnel,
  shipmentStatusBoard,
  documentsSummary,
  billingSummary,
  riskAlerts,
  recentActivity,
  loadData,
} = useControlTower()

const { backendOnline, backendStatus, checkBackendStatus } = useBackendStatus()
const checkingBackend = ref(false)

const systemLinks = [
  { key: 'health', labelKey: 'controlTower.systemLinks.health', to: '/health' },
  { key: 'swagger', labelKey: 'controlTower.systemLinks.swagger', href: 'http://localhost:8080/docs' },
  { key: 'prometheus', labelKey: 'controlTower.systemLinks.prometheus', href: 'http://localhost:9090' },
  { key: 'grafana', labelKey: 'controlTower.systemLinks.grafana', href: 'http://localhost:3001' },
]

onMounted(async () => {
  await checkBackendStatus()
  await loadData()
})

async function onRefreshAll() {
  checkingBackend.value = true
  try {
    await checkBackendStatus()
    await loadData()
  } finally {
    checkingBackend.value = false
  }
}
</script>

<template>
  <div class="page-stack control-tower">
    <header class="control-tower__hero">
      <div>
        <h1 class="control-tower__title">{{ $t('controlTower.title') }}</h1>
        <p class="control-tower__subtitle">{{ $t('controlTower.subtitle') }}</p>
      </div>
      <UiButton size="sm" variant="secondary" :disabled="loading" @click="loadData">
        {{ loading ? $t('common.loading') : $t('controlTower.refresh') }}
      </UiButton>
    </header>

    <div
      v-if="!backendOnline && backendStatus !== 'checking'"
      class="control-tower__backend-offline"
    >
      <p>{{ $t('controlTower.backendOffline') }}</p>
      <UiButton size="sm" variant="secondary" :loading="checkingBackend" @click="onRefreshAll">
        {{ $t('backendStatus.refresh') }}
      </UiButton>
    </div>

    <section class="control-tower__section">
      <h2 class="control-tower__section-title">{{ $t('controlTower.sections.kpi') }}</h2>
      <div class="control-tower__kpi-grid">
        <ControlTowerKpiCard
          v-for="card in kpiCards"
          :key="card.key"
          :title="$t(card.titleKey)"
          :value="card.value"
          :description="$t(card.descriptionKey)"
          :badge-label="card.badgeLabel"
          :badge-tone="card.badgeTone"
          :link="card.link"
          :unavailable="card.unavailable"
        />
      </div>
    </section>

    <section class="control-tower__section">
      <h2 class="control-tower__section-title">{{ $t('controlTower.sections.operationsStatus') }}</h2>
      <UiCard>
        <ControlTowerOperationsStatusTable :rows="operationsRows" :loading="loading" />
      </UiCard>
    </section>

    <div class="control-tower__two-col">
      <section class="control-tower__section">
        <h2 class="control-tower__section-title">{{ $t('controlTower.sections.transportFunnel') }}</h2>
        <UiCard>
          <ControlTowerFunnelBoard
            :steps="transportFunnel"
            :empty="transportFunnelEmpty"
            :empty-message="$t('controlTower.transportFunnel.empty')"
          />
        </UiCard>
      </section>

      <section class="control-tower__section">
        <h2 class="control-tower__section-title">{{ $t('controlTower.sections.tenderFunnel') }}</h2>
        <UiCard>
          <ControlTowerFunnelBoard :steps="tenderFunnel" />
        </UiCard>
      </section>
    </div>

    <section class="control-tower__section">
      <h2 class="control-tower__section-title">{{ $t('controlTower.sections.shipmentStatus') }}</h2>
      <UiCard>
        <ControlTowerShipmentStatusBoard :rows="shipmentStatusBoard" :loading="loading" />
      </UiCard>
    </section>

    <section class="control-tower__section">
      <h2 class="control-tower__section-title">{{ $t('controlTower.sections.documentsBilling') }}</h2>
      <UiCard>
        <ControlTowerDocumentsBillingStatus :documents="documentsSummary" :billing="billingSummary" />
      </UiCard>
    </section>

    <div class="control-tower__two-col">
      <section class="control-tower__section">
        <h2 class="control-tower__section-title">{{ $t('controlTower.sections.alertsRisks') }}</h2>
        <UiCard>
          <ControlTowerRiskAlerts :alerts="riskAlerts" />
        </UiCard>
      </section>

      <section class="control-tower__section">
        <h2 class="control-tower__section-title">{{ $t('controlTower.sections.quickActions') }}</h2>
        <UiCard>
          <ControlTowerQuickActions />
        </UiCard>
      </section>
    </div>

    <section class="control-tower__section">
      <h2 class="control-tower__section-title">{{ $t('controlTower.sections.recentActivity') }}</h2>
      <UiCard>
        <ControlTowerRecentActivity :items="recentActivity" :loading="loading" />
      </UiCard>
    </section>

    <section class="control-tower__section">
      <h2 class="control-tower__section-title">{{ $t('controlTower.sections.systemLinks') }}</h2>
      <UiCard>
        <div class="control-tower__system-links">
          <template v-for="link in systemLinks" :key="link.key">
            <NuxtLink v-if="link.to" :to="link.to" class="control-tower__system-link">
              {{ $t(link.labelKey) }}
            </NuxtLink>
            <a
              v-else-if="link.href"
              :href="link.href"
              class="control-tower__system-link"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ $t(link.labelKey) }}
            </a>
          </template>
        </div>
      </UiCard>
    </section>
  </div>
</template>

<style scoped>
.control-tower__hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.control-tower__title {
  margin: 0;
  font-size: 1.75rem;
}

.control-tower__subtitle {
  margin: 0.35rem 0 0;
  color: var(--color-text-muted);
  max-width: 52rem;
}

.control-tower__section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.control-tower__section-title {
  margin: 0;
  font-size: 1.0625rem;
  font-weight: 700;
}

.control-tower__kpi-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 1rem;
}

.control-tower__two-col {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 1rem;
}

.control-tower__system-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.control-tower__system-link {
  padding: 0.5rem 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-size: 0.875rem;
  text-decoration: none;
  color: var(--color-primary);
}

.control-tower__backend-offline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  background: #fffbeb;
  border: 1px solid #fde68a;
  color: #92400e;
  font-size: 0.875rem;
}

.control-tower__backend-offline p {
  margin: 0;
  flex: 1;
}

.control-tower__system-link:hover {
  background: #f8fafc;
  text-decoration: none;
}
</style>
