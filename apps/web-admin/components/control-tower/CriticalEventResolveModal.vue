<script setup lang="ts">
import {
  CONTROL_TOWER_RESOLUTION_CODES,
  type ControlTowerEventResolutionCode,
} from '~/types/controlTower'

defineProps<{
  open: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [resolutionCode: ControlTowerEventResolutionCode, comment?: string]
}>()

const resolutionCode = ref<ControlTowerEventResolutionCode>('issue_resolved')
const comment = ref('')

watch(
  () => resolutionCode.value,
  () => {
    if (resolutionCode.value !== 'other') {
      comment.value = ''
    }
  },
)

function onSubmit() {
  emit(
    'submit',
    resolutionCode.value,
    comment.value.trim() || undefined,
  )
}
</script>

<template>
  <UiModal :open="open" :title="$t('controlTower.events.resolve')" @close="$emit('close')">
    <label class="resolve-modal__label">
      {{ $t('controlTower.events.resolution') }}
      <select v-model="resolutionCode" class="resolve-modal__select" :disabled="loading">
        <option
          v-for="code in CONTROL_TOWER_RESOLUTION_CODES"
          :key="code"
          :value="code"
        >
          {{ $t(`controlTower.events.resolutionCodes.${code}`) }}
        </option>
      </select>
    </label>

    <label v-if="resolutionCode === 'other'" class="resolve-modal__label">
      {{ $t('controlTower.events.comment') }}
      <textarea
        v-model="comment"
        class="resolve-modal__textarea"
        rows="3"
        :disabled="loading"
      />
    </label>

    <template #footer>
      <UiButton variant="secondary" :disabled="loading" @click="$emit('close')">
        {{ $t('common.cancel') }}
      </UiButton>
      <UiButton
        :loading="loading"
        :disabled="resolutionCode === 'other' && !comment.trim()"
        @click="onSubmit"
      >
        {{ $t('controlTower.events.resolve') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.resolve-modal__label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.875rem;
  margin-bottom: 0.875rem;
}

.resolve-modal__select,
.resolve-modal__textarea {
  width: 100%;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
}
</style>
