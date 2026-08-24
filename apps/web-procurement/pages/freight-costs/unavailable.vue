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
    <div class="ui-page-header">
      <h1>{{ featureTitle }}</h1>
    </div>
    <div class="ui-empty">
      <h3>{{ featureTitle }}</h3>
      <p>{{ featureBody }}</p>
    </div>
  </div>
</template>

<style scoped>
.ui-page-header h1 {
  font-size: 1.5rem;
}

.ui-empty {
  padding: 3rem 1.5rem;
  text-align: center;
  color: var(--color-text-muted);
}

.ui-empty h3 {
  margin: 0 0 0.75rem;
  color: var(--color-text);
}
</style>
