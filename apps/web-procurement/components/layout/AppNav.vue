<script setup lang="ts">
const route = useRoute()

const navItems = [
  { to: '/', labelKey: 'nav.home' },
  { to: '/freight-requests', labelKey: 'nav.freightRequests' },
]

function isActive(path: string) {
  if (path === '/') {
    return route.path === '/'
  }
  return route.path === path || route.path.startsWith(`${path}/`)
}
</script>

<template>
  <nav class="app-nav" aria-label="Main navigation">
    <NuxtLink
      v-for="item in navItems"
      :key="item.to"
      :to="item.to"
      class="app-nav__link"
      :class="{ 'app-nav__link--active': isActive(item.to) }"
    >
      {{ $t(item.labelKey) }}
    </NuxtLink>
  </nav>
</template>

<style scoped>
.app-nav {
  display: flex;
  gap: 0.25rem;
  padding: 0 1.5rem;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}

.app-nav__link {
  display: inline-flex;
  align-items: center;
  padding: 0.75rem 0.875rem;
  border-bottom: 2px solid transparent;
  color: #4b5563;
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
}

.app-nav__link:hover {
  color: #1a1a1a;
}

.app-nav__link--active {
  color: #2563eb;
  border-bottom-color: #2563eb;
}
</style>
