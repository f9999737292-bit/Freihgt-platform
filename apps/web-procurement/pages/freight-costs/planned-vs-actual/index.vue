<script setup lang="ts">
import { createDefaultFreightCostFilters, resolveFreightCostListViewState } from '~/utils/freightCostWorkspace'

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

const filters = ref(createDefaultFreightCostFilters())
const items = ref([])

const viewState = computed(() => resolveFreightCostListViewState({
  loading: loading.value,
  missingCompany: missingCompany.value,
  forbidden: forbidden.value,
  liveUnavailable: liveUnavailable.value,
  apiUnavailable: apiUnavailable.value,
  itemCount: items.value.length,
}))
</script>

<template>
  <FreightCostShell
    :title="t('freightCosts.nav.plannedVsActual')"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <FreightCostFilters v-model="filters" />
    <div v-if="viewState === 'loading'">{{ t('common.loading') }}</div>
    <EmptyState
      v-else-if="viewState === 'empty' || liveUnavailable"
      :title="t('freightCosts.empty.plannedVsActual')"
      :description="liveUnavailable ? t('freightCosts.unavailable.liveData') : undefined"
    />
    <FreightCostOrdersTable
      v-else
      :actor="actor"
      :rows="items"
      :loading="loading"
      :live-unavailable="liveUnavailable"
    />
  </FreightCostShell>
</template>
