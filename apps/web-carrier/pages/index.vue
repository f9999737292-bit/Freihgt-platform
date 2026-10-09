<script setup lang="ts">
import { CustomerPageHeader } from '@freight-platform/ui'

const { t } = useI18n()
</script>

<template>
  <section data-testid="home-shell">
    <CustomerPageHeader :title="t('carrier.homeTitle')" :subtitle="t('carrier.homeBody')" />
  </section>
</template>
