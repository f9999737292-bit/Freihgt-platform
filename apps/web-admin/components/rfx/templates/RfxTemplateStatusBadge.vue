<script setup lang="ts">
import type { RfxTemplateAggregateStatus, RfxTemplateVersionStatus } from '~/types/rfx-template'

const props = defineProps<{
  aggregateStatus?: RfxTemplateAggregateStatus
  versionStatus?: RfxTemplateVersionStatus
}>()

const labelKey = computed(() => {
  if (props.versionStatus) return `rfx.templates.versionStatus.${props.versionStatus}`
  if (props.aggregateStatus) return `rfx.templates.aggregateStatus.${props.aggregateStatus}`
  return 'common.unknown'
})

const statusClass = computed(() => {
  const status = props.versionStatus ?? props.aggregateStatus ?? 'UNKNOWN'
  return `rfx-status-badge--${String(status).toLowerCase()}`
})
</script>

<template>
  <span class="rfx-status-badge" :class="statusClass">{{ $t(labelKey) }}</span>
</template>

<style scoped>
.rfx-status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.rfx-status-badge--draft { background: #e0f2fe; color: #0369a1; }
.rfx-status-badge--published { background: #dcfce7; color: #15803d; }
.rfx-status-badge--superseded { background: #f3f4f6; color: #4b5563; }
.rfx-status-badge--active { background: #dcfce7; color: #15803d; }
.rfx-status-badge--archived { background: #fee2e2; color: #b91c1c; }
</style>
