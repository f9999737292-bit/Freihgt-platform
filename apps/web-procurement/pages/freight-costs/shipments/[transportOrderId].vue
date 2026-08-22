<script setup lang="ts">
import type { FreightCostDetailVM } from '~/types/freightCost'
import { resolveFreightCostDetailViewState } from '~/utils/freightCostWorkspace'

definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const route = useRoute()
const { t } = useI18n()
const { getFreightCostOrderDetail } = useFreightCostsApi()
const {
  actor,
  loading,
  forbidden,
  apiUnavailable,
  liveUnavailable,
  missingCompany,
  currentCompanyId,
  runLoad,
} = useFreightCostPageContext()

const detail = ref<FreightCostDetailVM | null>(null)
const transportOrderId = computed(() => String(route.params.transportOrderId ?? ''))

async function loadDetail() {
  detail.value = await runLoad(() =>
    getFreightCostOrderDetail(transportOrderId.value, currentCompanyId.value!),
  )
}

onMounted(() => {
  void loadDetail()
})

const viewState = computed(() => resolveFreightCostDetailViewState({
  loading: loading.value,
  notFound: false,
  forbidden: forbidden.value,
  liveUnavailable: liveUnavailable.value,
  apiUnavailable: apiUnavailable.value,
  hasDetail: detail.value != null,
}))
</script>

<template>
  <FreightCostShell
    :title="t('freightCosts.detail.title')"
    :subtitle="transportOrderId"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <div v-if="viewState === 'loading'">{{ t('common.loading') }}</div>
    <EmptyState
      v-else-if="viewState === 'forbidden'"
      :title="t('freightCosts.errors.forbidden')"
    />
    <EmptyState
      v-else-if="liveUnavailable"
      :title="t('freightCosts.empty.detail')"
      :description="t('freightCosts.unavailable.liveData')"
    />
    <FreightCostDetailPanel
      v-else
      :actor="actor"
      :detail="detail"
      :live-unavailable="liveUnavailable"
    />
  </FreightCostShell>
</template>
