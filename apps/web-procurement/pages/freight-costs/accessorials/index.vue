<script setup lang="ts">
definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const { t } = useI18n()
const { currentCompanyId } = useTenantContext()
const { getFreightCostAccessorialSummary } = useFreightCostsApi()
const {
  actor,
  loading,
  liveUnavailable,
  runLoad,
} = useFreightCostPageContext()

const capabilityUnavailable = ref(false)

onMounted(async () => {
  if (!currentCompanyId.value) return
  const result = await runLoad(() => getFreightCostAccessorialSummary({ company_id: currentCompanyId.value! }))
  capabilityUnavailable.value = isFreightCostDataCapabilityUnavailable(result?.data_capability)
})
</script>

<template>
  <FreightCostShell
    :title="t('freightCosts.nav.accessorials')"
    :actor="actor"
  >
    <FreightCostLiveUnavailableBanner
      v-if="liveUnavailable"
      :message="t('freightCosts.unavailable.liveData')"
    />
    <div v-if="loading">{{ t('common.loading') }}</div>
    <EmptyState
      v-else-if="capabilityUnavailable"
      :title="t('freightCosts.unavailable.analytics')"
      :description="t('freightCosts.unavailable.liveData')"
    />
    <EmptyState
      v-else
      :title="t('freightCosts.empty.accessorials')"
      :description="t('freightCosts.unavailable.liveData')"
    />
  </FreightCostShell>
</template>
