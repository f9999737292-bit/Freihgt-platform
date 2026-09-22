<script setup lang="ts">
import BuyerXlsxCreatePanel from '~/components/rfx/BuyerXlsxCreatePanel.vue'
import PageHeader from '~/components/ui/PageHeader.vue'

definePageMeta({ middleware: 'auth', layout: 'default' })
</script>

<template>
  <div class="page-stack" data-testid="buyer-xlsx-create-page">
    <PageHeader
      :title="$t('tenders.buyerXlsxCreate.title')"
      :subtitle="$t('tenders.buyerXlsxCreate.hint')"
    />
    <BuyerXlsxCreatePanel />
  </div>
</template>
