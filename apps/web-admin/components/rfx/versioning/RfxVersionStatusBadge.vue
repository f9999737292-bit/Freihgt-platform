<script setup lang="ts">
import type { RfxVersionStatus } from '~/types/rfx-version-lifecycle'

defineProps<{ status: RfxVersionStatus }>()
</script>

<template>
  <span class="rfx-version-badge" :class="`rfx-version-badge--${status.toLowerCase()}`">
    {{ $t(`rfx.versions.status.${status}`) }}
  </span>
</template>

<style scoped>
.rfx-version-badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  font-weight: 600;
}

.rfx-version-badge--draft { background: #e0f2fe; color: #0369a1; }
.rfx-version-badge--published { background: #dcfce7; color: #15803d; }
.rfx-version-badge--superseded { background: #f3f4f6; color: #4b5563; }
.rfx-version-badge--archived { background: #fee2e2; color: #b91c1c; }
</style>
