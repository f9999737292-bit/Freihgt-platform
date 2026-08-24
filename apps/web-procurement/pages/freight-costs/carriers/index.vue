<script setup lang="ts">
import type { FreightCostAnalyticsCarriersResponse } from '~/types/freightCost'
import { resolveFreightCostIntelligenceListViewState } from '~/utils/freightCostIntelligence'

definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const { t } = useI18n()
const { currentCompanyId } = useTenantContext()
const { getFreightCostAnalyticsCarriers } = useFreightCostAnalyticsApi()
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
const route = useRoute()
const response = ref<FreightCostAnalyticsCarriersResponse | null>(null)

async function loadCarriers() {
  if (!currentCompanyId.value) return
  const query = routeQuery.value
  const result = await runLoad(() => getFreightCostAnalyticsCarriers({
    company_id: currentCompanyId.value!,
    currency: query.currency,
    limit: query.limit,
    offset: query.offset,
  }))
  if (result) {
    response.value = result
  }
}

onMounted(() => {
  void loadCarriers()
})

watch(
  () => [currentCompanyId.value, route.query.currency, route.query.limit, route.query.offset] as const,
  () => {
    void loadCarriers()
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
    :title="t('freightCosts.nav.carriers')"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <div v-if="viewState === 'loading'">{{ t('common.loading') }}</div>
    <template v-else-if="response?.items?.length">
      <FreightCostIntelligenceDataQualityBanner
        :data-quality="response.data_quality"
        :mixed-currency="response.mixed_currency"
        :freshness="response.freshness"
      />
      <FreightCostCarrierIntelligenceTable
        :items="response.items"
        :mixed-currency="response.mixed_currency"
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
      :title="t('freightCosts.empty.carriers')"
    />
  </FreightCostShell>
</template>
