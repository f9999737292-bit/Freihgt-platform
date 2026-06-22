<script setup lang="ts">
const uiStore = useUiStore()

const navItems = [
  { to: '/dashboard', icon: '▣', labelKey: 'nav.dashboard' },
  { to: '/control-tower', icon: '◈', labelKey: 'nav.controlTower' },
  { to: '/companies', icon: '⌂', labelKey: 'nav.companies' },
  { to: '/users', icon: '👤', labelKey: 'nav.users' },
  { to: '/transport-orders', icon: '⇄', labelKey: 'nav.transportOrders' },
  { to: '/freight-requests', icon: '⚡', labelKey: 'nav.freightRequests' },
  { to: '/rfx', icon: '⚖', labelKey: 'nav.rfx' },
  { to: '/shipments', icon: '⛟', labelKey: 'nav.shipments' },
  { to: '/documents', icon: '📄', labelKey: 'nav.documents' },
  { to: '/billing-registers', icon: '₽', labelKey: 'nav.billingRegisters' },
  { to: '/low-code', icon: '⬡', labelKey: 'nav.lowCode' },
  { to: '/health', icon: '♥', labelKey: 'nav.health' },
  { to: '/settings', icon: '⚙', labelKey: 'nav.settings' },
]
</script>

<template>
  <aside class="sidebar" :class="{ 'sidebar--collapsed': uiStore.sidebarCollapsed }">
    <div class="sidebar__brand">
      <span class="sidebar__logo">7R</span>
      <div v-if="!uiStore.sidebarCollapsed" class="sidebar__brand-text">
        <strong>7Rights</strong>
        <span>Freight Platform</span>
      </div>
    </div>
    <nav class="sidebar__nav">
      <NuxtLink
        v-for="item in navItems"
        :key="item.to"
        :to="item.to"
        class="sidebar__link"
        active-class="sidebar__link--active"
      >
        <span class="sidebar__icon">{{ item.icon }}</span>
        <span v-if="!uiStore.sidebarCollapsed">{{ $t(item.labelKey) }}</span>
      </NuxtLink>
    </nav>
  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar-width);
  min-height: 100vh;
  background: var(--color-sidebar);
  color: var(--color-sidebar-text);
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease;
}

.sidebar--collapsed {
  width: var(--sidebar-collapsed-width);
}

.sidebar__brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1.25rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.sidebar__logo {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border-radius: var(--radius-md);
  background: var(--color-primary);
  color: #fff;
  font-weight: 700;
  flex-shrink: 0;
}

.sidebar__brand-text {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
}

.sidebar__brand-text strong {
  color: #fff;
}

.sidebar__brand-text span {
  font-size: 0.75rem;
  opacity: 0.75;
}

.sidebar__nav {
  display: flex;
  flex-direction: column;
  padding: 0.75rem;
  gap: 0.25rem;
}

.sidebar__link {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 0.75rem;
  border-radius: var(--radius-md);
  color: var(--color-sidebar-text);
  text-decoration: none;
  transition: background 0.15s;
}

.sidebar__link:hover {
  background: var(--color-sidebar-hover);
  text-decoration: none;
}

.sidebar__link--active {
  background: var(--color-sidebar-active);
  color: var(--color-sidebar-text-active);
}

.sidebar__icon {
  width: 1.25rem;
  text-align: center;
  flex-shrink: 0;
}
</style>
