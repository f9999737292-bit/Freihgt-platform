<script setup lang="ts">
import type { ControlTowerQuickAction } from '~/types/controlTower'

const actions: ControlTowerQuickAction[] = [
  { key: 'company', labelKey: 'controlTower.quickActions.createCompany', to: '/companies' },
  { key: 'user', labelKey: 'controlTower.quickActions.createUser', to: '/users' },
  { key: 'order', labelKey: 'controlTower.quickActions.createTransportOrder', to: '/transport-orders' },
  { key: 'rfx', labelKey: 'controlTower.quickActions.createRfx', to: '/rfx' },
  { key: 'shipments', labelKey: 'controlTower.quickActions.viewShipments', to: '/shipments' },
  { key: 'billing', labelKey: 'controlTower.quickActions.createBillingRegister', to: '/billing-registers' },
  { key: 'health', labelKey: 'controlTower.quickActions.openHealth', to: '/health' },
  { key: 'swagger', labelKey: 'controlTower.quickActions.openSwagger', href: 'http://localhost:8080/docs', external: true },
  { key: 'prometheus', labelKey: 'controlTower.quickActions.openPrometheus', href: 'http://localhost:9090', external: true },
  { key: 'grafana', labelKey: 'controlTower.quickActions.openGrafana', href: 'http://localhost:3001', external: true },
]
</script>

<template>
  <div class="quick-actions">
    <template v-for="action in actions" :key="action.key">
      <NuxtLink v-if="action.to" :to="action.to" class="quick-actions__btn">
        {{ $t(action.labelKey) }}
      </NuxtLink>
      <a
        v-else-if="action.href"
        :href="action.href"
        class="quick-actions__btn quick-actions__btn--external"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ $t(action.labelKey) }}
      </a>
    </template>
  </div>
</template>

<style scoped>
.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.quick-actions__btn {
  display: inline-flex;
  align-items: center;
  padding: 0.5rem 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
  font-size: 0.8125rem;
  font-weight: 600;
  text-decoration: none;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.quick-actions__btn:hover {
  border-color: var(--color-primary);
  background: #f8fafc;
  text-decoration: none;
}

.quick-actions__btn--external::after {
  content: '↗';
  margin-left: 0.35rem;
  font-size: 0.75rem;
  opacity: 0.7;
}
</style>
