<script setup lang="ts">
import type { FreightCostActor } from '~/types/freightCost'

defineProps<{
  title: string
  subtitle?: string
  actor: FreightCostActor
}>()
</script>

<template>
  <div class="freight-cost-shell">
    <PageHeader :title="title" :subtitle="subtitle" />
    <FreightCostSubnav :actor="actor" />
    <slot />
  </div>
</template>

<style scoped>
.freight-cost-shell {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
