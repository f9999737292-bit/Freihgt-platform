<script setup lang="ts">
import type { FreightCostActor } from '~/types/freightCost'
import { buildFreightCostNavItems } from '~/utils/freightCostWorkspace'

const props = defineProps<{
  actor: FreightCostActor
}>()

const { t } = useI18n()
const route = useRoute()

const items = computed(() => buildFreightCostNavItems(props.actor))

function isActive(to: string) {
  if (to === '/freight-costs') return route.path === to
  return route.path === to || route.path.startsWith(`${to}/`)
}
</script>

<template>
  <nav class="freight-cost-subnav" aria-label="Freight cost navigation">
    <NuxtLink
      v-for="item in items"
      :key="item.key"
      :to="item.to"
      class="freight-cost-subnav__link"
      :class="{ 'freight-cost-subnav__link--active': isActive(item.to) }"
    >
      {{ t(item.labelKey) }}
    </NuxtLink>
  </nav>
</template>

<style scoped>
.freight-cost-subnav {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-bottom: 1rem;
}

.freight-cost-subnav__link {
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-md);
  color: var(--color-text-muted);
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
}

.freight-cost-subnav__link--active {
  background: #dbeafe;
  color: #1d4ed8;
}
</style>
