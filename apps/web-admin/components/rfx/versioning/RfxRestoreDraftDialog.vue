<script setup lang="ts">
import type { RfxVersionRecord } from '~/types/rfx-version-lifecycle'

const props = defineProps<{
  open: boolean
  sourceVersion: RfxVersionRecord | null
  submitting?: boolean
  errorMessage?: string
}>()

const emit = defineEmits<{ close: []; confirm: [changeSummary: string] }>()

const changeSummary = ref('')

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) changeSummary.value = ''
  },
)

function handleConfirm() {
  if (!changeSummary.value.trim()) return
  emit('confirm', changeSummary.value.trim())
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" role="dialog" aria-modal="true" @click.self="emit('close')">
    <div class="modal">
      <h2>{{ $t('rfx.restore.title') }}</h2>
      <p v-if="sourceVersion">
        {{ $t('rfx.restore.sourceVersion', { number: sourceVersion.version_number }) }}
      </p>
      <p class="modal__warning">{{ $t('rfx.restore.warning') }}</p>

      <label class="modal__label">
        {{ $t('rfx.restore.changeSummary') }}
        <textarea v-model="changeSummary" rows="3" required />
      </label>

      <p v-if="errorMessage" class="modal__error">{{ errorMessage }}</p>

      <div class="modal__actions">
        <button type="button" class="btn btn--secondary" @click="emit('close')">
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn--primary"
          :disabled="submitting || !changeSummary.trim()"
          @click="handleConfirm"
        >
          {{ $t('rfx.restore.confirm') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: grid;
  place-items: center;
  z-index: 1000;
  padding: 1rem;
}
.modal {
  background: var(--color-surface, #fff);
  border-radius: var(--radius-lg);
  padding: 1.5rem;
  width: min(100%, 32rem);
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.modal__warning { color: var(--color-warning-text, #b45309); margin: 0; }
.modal__label { display: flex; flex-direction: column; gap: 0.375rem; font-size: 0.875rem; }
.modal__error { color: var(--color-danger, #b91c1c); margin: 0; }
.modal__actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 0.5rem; }
</style>
