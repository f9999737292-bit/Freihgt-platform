<script setup lang="ts">
const uiStore = useUiStore()
const { toasts } = useToast()
</script>

<template>
  <div class="app-shell">
    <LayoutAppSidebar />
    <div class="app-shell__main">
      <LayoutAppHeader />
      <main class="app-shell__content">
        <SystemBackendStatusBanner />
        <LayoutAppBreadcrumbs />
        <slot />
      </main>
    </div>
    <div class="toast-stack">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="toast"
        :class="`toast--${toast.type}`"
      >
        {{ toast.message }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.app-shell {
  display: flex;
  min-height: 100vh;
}

.app-shell__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.app-shell__content {
  flex: 1;
  padding: 1.5rem;
}

.toast-stack {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  z-index: 1100;
}

.toast {
  min-width: 240px;
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  color: #fff;
}

.toast--success {
  background: var(--color-success);
}

.toast--error {
  background: var(--color-danger);
}

.toast--info {
  background: var(--color-info);
}
</style>
