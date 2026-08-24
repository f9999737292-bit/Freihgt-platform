<script setup lang="ts">
import type { FreightCostAnalyticsOpportunitiesResponse } from '~/types/freightCost'
import { resolveFreightCostIntelligenceListViewState } from '~/utils/freightCostIntelligence'

definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const { t } = useI18n()
const { currentCompanyId } = useTenantContext()
const { getFreightCostAnalyticsOpportunities } = useFreightCostAnalyticsApi()
const {
  actor,
  loading,
  forbidden,
  apiUnavailable,
  liveUnavailable,
  missingCompany,
  runLoad,
} = useFreightCostPageContext()

const routeQuery = useFreightCostIntelligenceRouteQuery()
const response = ref<FreightCostAnalyticsOpportunitiesResponse | null>(null)

async function loadOpportunities() {
  if (!currentCompanyId.value) return
  const query = routeQuery.value
  const result = await runLoad(() => getFreightCostAnalyticsOpportunities({
    company_id: currentCompanyId.value!,
    currency: query.currency,
    limit: query.limit,
    offset: query.offset,
  }))
  if (result) {
    response.value = result
  }
}

const route = useRoute()

onMounted(() => {
  void loadOpportunities()
})

watch(
  () => [currentCompanyId.value, route.query.currency, route.query.limit, route.query.offset] as const,
  () => {
    void loadOpportunities()
  },
)

const viewState = computed(() => resolveFreightCostIntelligenceListViewState({
  loading: loading.value,
  missingCompany: missingCompany.value,
  forbidden: forbidden.value,
  liveUnavailable: liveUnavailable.value,
  apiUnavailable: apiUnavailable.value,
  dataQuality: response.value?.data_quality,
  mixedCurrency: response.value?.mixed_currency ?? false,
  itemCount: response.value?.items?.length ?? 0,
}))
</script>

<template>
  <FreightCostShell
    :title="t('freightCosts.nav.opportunities')"
    :subtitle="t('freightCosts.intelligence.opportunities.subtitle')"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <p class="opportunities-hint">{{ t('freightCosts.intelligence.hints.backendSavingsOnly') }}</p>
    <div v-if="loading && !response">{{ t('common.loading') }}</div>
    <template v-else-if="response?.items?.length">
      <FreightCostIntelligenceDataQualityBanner
        :data-quality="response.data_quality"
        :mixed-currency="response.mixed_currency"
        :freshness="response.freshness"
      />
      <FreightCostOpportunitiesTable
        :items="response.items"
        :live-unavailable="liveUnavailable"
      />
    </template>
    <EmptyState
      v-else-if="viewState === 'missing_company'"
      :title="t('freightCosts.errors.missingCompany')"
    />
    <EmptyState
      v-else-if="viewState === 'forbidden'"
      :title="t('freightCosts.errors.forbidden')"
    />
    <EmptyState
      v-else-if="viewState === 'backend_unavailable'"
      :title="t('freightCosts.errors.apiFailure')"
    />
    <EmptyState
      v-else-if="viewState === 'not_available'"
      :title="t('freightCosts.unavailable.analytics')"
      :description="t('freightCosts.intelligence.banners.notAvailable')"
    />
    <EmptyState
      v-else-if="viewState === 'mixed_currency'"
      :title="t('freightCosts.mixedCurrencyUnavailable')"
      :description="t('freightCosts.intelligence.banners.mixedCurrency')"
    />
    <EmptyState
      v-else-if="viewState === 'empty'"
      :title="t('freightCosts.empty.opportunities')"
    />
  </FreightCostShell>
</template>

<style scoped>
.opportunities-hint {
  margin: 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}
</style>
