<script setup lang="ts">
defineProps<{
  kind: 'category' | 'source' | 'severity'
  value: string
}>()
</script>

<template>
  <span class="event-badge" :data-kind="kind" :data-value="value">
    {{ $t(`shipmentEvents.${kind}s.${value}`, value) }}
  </span>
</template>

<style scoped>
.event-badge {
  display: inline-flex;
  align-items: center;
  font-size: 0.75rem;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  background: #eef2ff;
  color: #3730a3;
}

.event-badge[data-kind='severity'][data-value='WARNING'] {
  background: #fef3c7;
  color: #92400e;
}

.event-badge[data-kind='severity'][data-value='CRITICAL'] {
  background: #fee2e2;
  color: #991b1b;
}
</style>
