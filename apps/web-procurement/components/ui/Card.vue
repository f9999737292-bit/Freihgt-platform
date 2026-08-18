<template>
  <section class="ui-card">
    <header v-if="$slots.header" class="ui-card__header">
      <slot name="header" />
    </header>
    <div class="ui-card__body">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="ui-card__footer">
      <slot name="footer" />
    </footer>
  </section>
</template>

<style scoped>
.ui-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.ui-card__header,
.ui-card__footer {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--color-border);
}

.ui-card__footer {
  border-bottom: none;
  border-top: 1px solid var(--color-border);
}

.ui-card__body {
  padding: 1.25rem;
}
</style>
