<script setup lang="ts">
defineProps<{ open: boolean; title?: string }>()
defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="ui-modal">
      <div class="ui-modal__backdrop" @click="$emit('close')" />
      <div class="ui-modal__dialog" role="dialog" aria-modal="true">
        <header v-if="title" class="ui-modal__header">
          <h3>{{ title }}</h3>
          <button type="button" class="ui-modal__close" @click="$emit('close')">×</button>
        </header>
        <div class="ui-modal__body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="ui-modal__footer">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.ui-modal {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.ui-modal__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
}

.ui-modal__dialog {
  position: relative;
  width: min(560px, 100%);
  max-height: 90vh;
  overflow: auto;
  background: var(--color-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
}

.ui-modal__header,
.ui-modal__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--color-border);
}

.ui-modal__footer {
  border-bottom: none;
  border-top: 1px solid var(--color-border);
  justify-content: flex-end;
  gap: 0.75rem;
}

.ui-modal__body {
  padding: 1.25rem;
}

.ui-modal__close {
  border: none;
  background: transparent;
  font-size: 1.5rem;
  line-height: 1;
  cursor: pointer;
  color: var(--color-text-muted);
}
</style>
