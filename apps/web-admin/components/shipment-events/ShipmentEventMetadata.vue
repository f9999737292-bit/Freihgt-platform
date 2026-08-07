<script setup lang="ts">
defineProps<{
  metadata: Record<string, unknown>
}>()

const allowedKeys = ['documentId', 'documentType', 'documentStatus', 'billingRegisterId', 'billingRegisterNumber', 'billingStatus', 'plannedAt', 'actualAt', 'delayMinutes', 'slaReason', 'slaStatus'] as const
</script>

<template>
  <dl class="metadata">
    <template v-for="key in allowedKeys" :key="key">
      <template v-if="metadata[key] !== undefined && metadata[key] !== null && metadata[key] !== ''">
        <dt>{{ $t(`shipmentEvents.metadata.${key}`) }}</dt>
        <dd>{{ metadata[key] }}</dd>
      </template>
    </template>
  </dl>
</template>

<style scoped>
.metadata {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.25rem 0.75rem;
  margin: 0.5rem 0 0;
  font-size: 0.8125rem;
}

dt {
  color: var(--color-text-muted);
}

dd {
  margin: 0;
}
</style>
