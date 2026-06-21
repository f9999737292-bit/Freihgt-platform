<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    status: string
    tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
  }>(),
  { tone: 'neutral' },
)

const toneClass = computed(() => {
  const map: Record<string, string> = {
    ACTIVE: 'success',
    DRAFT: 'neutral',
    PUBLISHED: 'info',
    APPROVED: 'success',
    CLOSED: 'neutral',
    CANCELLED: 'danger',
    CANCELED: 'danger',
  }
  return `ui-badge--${map[props.status] || props.tone}`
})
</script>

<template>
  <span class="ui-badge" :class="toneClass">{{ status }}</span>
</template>

<style scoped>
.ui-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.ui-badge--neutral {
  background: #eef2f6;
  color: #475569;
}

.ui-badge--success {
  background: #dcfce7;
  color: #166534;
}

.ui-badge--warning {
  background: #fef3c7;
  color: #92400e;
}

.ui-badge--danger {
  background: #fee2e2;
  color: #991b1b;
}

.ui-badge--info {
  background: #e0f2fe;
  color: #075985;
}
</style>
