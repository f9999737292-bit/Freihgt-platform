<template>
  <div class="auth-layout">
    <div class="auth-layout__panel">
      <div class="auth-layout__brand">
        <span class="auth-layout__logo">7R</span>
        <div>
          <strong>7Rights Freight Platform</strong>
          <p>{{ $t('app.subtitle') }}</p>
        </div>
      </div>
      <slot />
    </div>
  </div>
</template>

<style scoped>
.auth-layout {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 1.5rem;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.08), transparent 40%),
    var(--color-bg);
}

.auth-layout__panel {
  width: min(420px, 100%);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  padding: 2rem;
}

.auth-layout__brand {
  display: flex;
  gap: 1rem;
  align-items: center;
  margin-bottom: 1.5rem;
}

.auth-layout__logo {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border-radius: var(--radius-md);
  background: var(--color-sidebar);
  color: #fff;
  font-weight: 700;
}

.auth-layout__brand p {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
