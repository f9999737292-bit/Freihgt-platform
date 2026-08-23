<script setup lang="ts">
import type { FreightCostAnalyticsLanesResponse } from '~/types/freightCost'
import { resolveFreightCostIntelligenceListViewState } from '~/utils/freightCostIntelligence'

definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const { t } = useI18n()
const { currentCompanyId } = useTenantContext()
const { getFreightCostAnalyticsLanes } = useFreightCostAnalyticsApi()
const {
  actor,
  loading,
  forbidden,
  apiUnavailable,
  liveUnavailable,
  missingCompany,
  runLoad,
} = useFreightCostPageContext()

const response = ref<FreightCostAnalyticsLanesResponse | null>(null)

onMounted(async () => {
  if (!currentCompanyId.value) return
  response.value = await runLoad(() => getFreightCostAnalyticsLanes({
    company_id: currentCompanyId.value!,
  }))
})

const viewState = computed(() => resolveFreightCostIntelligenceListViewState({
  loading: loading.value,
  missingCompany: missingCompany.value,
  forbidden: forbidden.value,
  liveUnavailable: liveUnavailable.value,
  apiUnavailable: apiUnavailable.value,
  dataQuality: response.value?.data_quality,
  mixedCurrency: response.value?.mixed_currency ?? false,
  itemCount: response.value?.items.length ?? 0,
}))
</script>

<template>
  <FreightCostShell
    :title="t('freightCosts.nav.lanes')"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <div v-if="viewState === 'loading'">{{ t('common.loading') }}</div>
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
      :title="t('freightCosts.empty.lanes')"
    />
    <template v-else-if="response">
      <FreightCostIntelligenceDataQualityBanner
        :data-quality="response.data_quality"
        :mixed-currency="response.mixed_currency"
        :freshness="response.freshness"
      />
      <FreightCostLaneIntelligenceTable
        :items="response.items"
        :mixed-currency="response.mixed_currency"
        :live-unavailable="liveUnavailable"
      />
    </template>
  </FreightCostShell>
</template>
