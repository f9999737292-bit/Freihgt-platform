<script setup lang="ts">
import { CustomerPageHeader } from '@freight-platform/ui'

const { t } = useI18n()
</script>

<template>
  <section data-testid="home-shell">
    <CustomerPageHeader :title="t('shipper.homeTitle')" :subtitle="t('shipper.homeBody')" />
  </section>
</template>
