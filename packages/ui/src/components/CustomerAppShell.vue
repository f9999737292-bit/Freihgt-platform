<script setup lang="ts">
defineProps<{
  title: string
}>()
</script>

<template>
  <div class="customer-shell">
    <aside class="customer-shell__nav">
      <p class="customer-shell__title">{{ title }}</p>
      <slot name="nav" />
    </aside>
    <div class="customer-shell__body">
      <header class="customer-shell__bar">
        <slot name="context" />
        <div class="customer-shell__actions">
          <slot name="actions" />
        </div>
      </header>
      <main class="customer-shell__main">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.customer-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 14rem 1fr;
  font-family: system-ui, sans-serif;
  color: #1a1a1a;
  background: #f7f8fa;
}
.customer-shell__nav {
  background: #fff;
  border-right: 1px solid #e5e7eb;
  padding: 1rem;
}
.customer-shell__title {
  margin: 0 0 1rem;
  font-weight: 650;
}
.customer-shell__bar {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: center;
  padding: 0.75rem 1.25rem;
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
}
.customer-shell__actions {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}
.customer-shell__main {
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
@media (max-width: 720px) {
  .customer-shell {
    grid-template-columns: 1fr;
  }
}
</style>
