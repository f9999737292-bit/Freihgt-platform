<script setup lang="ts">
import { SHIPMENT_STATUSES } from '~/types/shipment'

const props = defineProps<{ status: string }>()

const timelineStatuses = SHIPMENT_STATUSES.filter((s) => s !== 'CANCELLED')

const currentIndex = computed(() => {
  if (props.status === 'CANCELLED') return -1
  return timelineStatuses.indexOf(props.status as (typeof timelineStatuses)[number])
})

function stepClass(index: number) {
  if (props.status === 'CANCELLED') return 'timeline-step--cancelled'
  if (index < currentIndex.value) return 'timeline-step--done'
  if (index === currentIndex.value) return 'timeline-step--current'
  return 'timeline-step--pending'
}
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('shipments.statusTimeline') }}</h3>
    </template>
    <div v-if="status === 'CANCELLED'" class="cancelled-note">{{ $t('shipments.cancelledNote') }}</div>
    <ol class="timeline">
      <li
        v-for="(step, index) in timelineStatuses"
        :key="step"
        class="timeline-step"
        :class="stepClass(index)"
      >
        <span class="timeline-step__dot" />
        <span class="timeline-step__label">{{ step }}</span>
      </li>
    </ol>
  </UiCard>
</template>

<style scoped>
.timeline {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.timeline-step {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.875rem;
}

.timeline-step__dot {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 999px;
  background: var(--color-border);
  flex-shrink: 0;
}

.timeline-step--done .timeline-step__dot {
  background: #16a34a;
}

.timeline-step--current .timeline-step__dot {
  background: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2);
}

.timeline-step--current .timeline-step__label {
  font-weight: 600;
  color: var(--color-text);
}

.timeline-step--pending .timeline-step__label {
  color: var(--color-text-muted);
}

.timeline-step--cancelled .timeline-step__dot {
  background: #ef4444;
}

.cancelled-note {
  margin-bottom: 1rem;
  padding: 0.75rem;
  border-radius: var(--radius-sm);
  background: #fee2e2;
  color: #991b1b;
  font-size: 0.875rem;
}

.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}
</style>
