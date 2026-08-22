<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('freightCosts.unavailable.featureTitle')" />
    <EmptyState
      :title="t('freightCosts.unavailable.featureTitle')"
      :description="t('freightCosts.unavailable.featureBody')"
    />
  </div>
</template>
