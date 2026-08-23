<script setup lang="ts">
import { resolveFreightCostOverviewViewState } from '~/utils/freightCostWorkspace'

definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const { t } = useI18n()
const {
  actor,
  loading,
  forbidden,
  apiUnavailable,
  liveUnavailable,
  missingCompany,
} = useFreightCostPageContext()

const viewState = computed(() => resolveFreightCostOverviewViewState({
  loading: loading.value,
  missingCompany: missingCompany.value,
  forbidden: forbidden.value,
  liveUnavailable: liveUnavailable.value,
  apiUnavailable: apiUnavailable.value,
  mixedCurrency: false,
  hasAggregate: false,
}))
</script>

<template>
  <FreightCostShell
    :title="t('freightCosts.nav.overview')"
    :subtitle="t('freightCosts.overview.subtitle')"
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
    <FreightCostOverviewKpis
      v-else
      :actor="actor"
      :aggregate="null"
      :mixed-currency="false"
      :live-unavailable="liveUnavailable"
    />
  </FreightCostShell>
</template>
