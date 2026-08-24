<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()

const featureFlagEnvVar = 'NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED'

const featureTitle = computed(() => {
  const label = t('freightCosts.unavailable.featureTitle')
  return label.startsWith('freightCosts.') ? 'Freight cost workspace unavailable' : label
})

const featureBody = computed(() => {
  const label = t('freightCosts.unavailable.featureBody')
  return label.startsWith('freightCosts.')
    ? `This workspace is disabled. Enable ${featureFlagEnvVar} to access routes.`
    : label
})
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="featureTitle" />
    <EmptyState
      :title="featureTitle"
      :description="featureBody"
    />
    <p class="feature-flag-env-hint">{{ featureFlagEnvVar }}</p>
  </div>
</template>

<style scoped>
.feature-flag-env-hint {
  margin: 0;
  text-align: center;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}
</style>
