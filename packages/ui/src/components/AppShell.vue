<script setup lang="ts">
defineProps<{
  title: string
}>()
</script>

<template>
  <div class="app-shell">
    <header class="app-shell__header">
      <h1>{{ title }}</h1>
      <slot name="actions" />
    </header>
    <main class="app-shell__main">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
  font-family: system-ui, sans-serif;
  color: #1a1a1a;
  background: #f7f8fa;
}

.app-shell__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.5rem;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}

.app-shell__header h1 {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
}

.app-shell__main {
  padding: 1.5rem;
}
</style>
