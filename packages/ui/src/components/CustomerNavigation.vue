<script setup lang="ts">
defineProps<{
  items: Array<{ href: string; label: string; active?: boolean }>
}>()
</script>

<template>
  <nav class="ui-nav" aria-label="Customer">
    <a
      v-for="item in items"
      :key="item.href"
      :href="item.href"
      :aria-current="item.active ? 'page' : undefined"
    >
      {{ item.label }}
    </a>
  </nav>
</template>

<style scoped>
.ui-nav {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.ui-nav a {
  color: #1a1a1a;
  text-decoration: none;
  padding: 0.45rem 0.65rem;
  border-radius: 6px;
}
.ui-nav a[aria-current='page'] {
  background: #e8eef1;
  font-weight: 600;
}
</style>
