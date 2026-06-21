<script setup lang="ts">
const route = useRoute()

const crumbs = computed(() => {
  const segments = route.path.split('/').filter(Boolean)
  return segments.map((segment, index) => ({
    label: segment,
    to: '/' + segments.slice(0, index + 1).join('/'),
    isLast: index === segments.length - 1,
  }))
})
</script>

<template>
  <nav v-if="crumbs.length" class="breadcrumbs" aria-label="Breadcrumb">
    <NuxtLink to="/dashboard">Dashboard</NuxtLink>
    <template v-for="crumb in crumbs" :key="crumb.to">
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink v-if="!crumb.isLast" :to="crumb.to">{{ crumb.label }}</NuxtLink>
      <span v-else class="breadcrumbs__current">{{ crumb.label }}</span>
    </template>
  </nav>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  margin-bottom: 1rem;
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.breadcrumbs__current {
  color: var(--color-text);
  font-weight: 500;
}
</style>
