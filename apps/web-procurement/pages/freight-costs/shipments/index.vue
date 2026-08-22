<script setup lang="ts">
import { resolveFreightCostListViewState } from '~/utils/freightCostWorkspace'

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
    :title="t('freightCosts.nav.shipments')"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <div v-if="viewState === 'loading'">{{ t('common.loading') }}</div>
    <EmptyState
      v-else
      :title="t('freightCosts.empty.shipments')"
      :description="liveUnavailable ? t('freightCosts.unavailable.liveData') : undefined"
    />
  </FreightCostShell>
</template>
