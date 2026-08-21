<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('contracts.featureUnavailableTitle')" />
    <EmptyState
      :title="t('contracts.featureUnavailableTitle')"
      :description="t('contracts.featureUnavailableBody')"
    />
  </div>
</template>
