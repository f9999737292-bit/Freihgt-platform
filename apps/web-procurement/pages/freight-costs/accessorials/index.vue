<script setup lang="ts">
definePageMeta({ middleware: ['auth', 'freight-cost-workspace'], layout: 'default' })

const { t } = useI18n()
const {
  actor,
  loading,
  liveUnavailable,
} = useFreightCostPageContext()
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
      v-else
      :title="t('freightCosts.empty.accessorials')"
      :description="t('freightCosts.unavailable.liveData')"
    />
  </FreightCostShell>
</template>
