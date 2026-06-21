<script setup lang="ts">
import type { ControlTowerBillingSummary, ControlTowerDocumentsSummary } from '~/types/controlTower'

defineProps<{
  documents: ControlTowerDocumentsSummary
  billing: ControlTowerBillingSummary
}>()
</script>

<template>
  <div class="docs-billing-grid">
    <div class="docs-billing-panel">
      <h4 class="docs-billing-panel__title">{{ $t('controlTower.documentsBilling.documents') }}</h4>
      <dl class="docs-billing-panel__list">
        <div><dt>{{ $t('controlTower.documentsBilling.total') }}</dt><dd>{{ documents.unavailable ? '—' : documents.total }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.readyForSigning') }}</dt><dd>{{ documents.unavailable ? '—' : documents.readyForSigning }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.signed') }}</dt><dd>{{ documents.unavailable ? '—' : documents.signed }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.archived') }}</dt><dd>{{ documents.unavailable ? '—' : documents.archived }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.cancelled') }}</dt><dd>{{ documents.unavailable ? '—' : documents.cancelled }}</dd></div>
      </dl>
      <p v-if="documents.unavailable" class="docs-billing-panel__warn">{{ $t('controlTower.apiUnavailable') }}</p>
    </div>

    <div class="docs-billing-panel">
      <h4 class="docs-billing-panel__title">{{ $t('controlTower.documentsBilling.billing') }}</h4>
      <dl class="docs-billing-panel__list">
        <div><dt>{{ $t('controlTower.documentsBilling.total') }}</dt><dd>{{ billing.unavailable ? '—' : billing.total }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.draft') }}</dt><dd>{{ billing.unavailable ? '—' : billing.draft }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.approved') }}</dt><dd>{{ billing.unavailable ? '—' : billing.approved }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.closingDocsCreated') }}</dt><dd>{{ billing.unavailable ? '—' : billing.closingDocsCreated }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.sentToEdo') }}</dt><dd>{{ billing.unavailable ? '—' : billing.sentToEdo }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.signed') }}</dt><dd>{{ billing.unavailable ? '—' : billing.signed }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.paid') }}</dt><dd>{{ billing.unavailable ? '—' : billing.paid }}</dd></div>
        <div><dt>{{ $t('controlTower.documentsBilling.closed') }}</dt><dd>{{ billing.unavailable ? '—' : billing.closed }}</dd></div>
      </dl>
      <p v-if="billing.unavailable" class="docs-billing-panel__warn">{{ $t('controlTower.apiUnavailable') }}</p>
    </div>
  </div>
</template>

<style scoped>
.docs-billing-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1rem;
}

.docs-billing-panel {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: #f8fafc;
}

.docs-billing-panel__title {
  margin: 0 0 0.75rem;
  font-size: 0.9375rem;
}

.docs-billing-panel__list {
  margin: 0;
  display: grid;
  gap: 0.5rem;
}

.docs-billing-panel__list > div {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  font-size: 0.875rem;
}

.docs-billing-panel__list dt {
  color: var(--color-text-muted);
}

.docs-billing-panel__list dd {
  margin: 0;
  font-weight: 600;
}

.docs-billing-panel__warn {
  margin: 0.75rem 0 0;
  color: #b45309;
  font-size: 0.75rem;
}
</style>
