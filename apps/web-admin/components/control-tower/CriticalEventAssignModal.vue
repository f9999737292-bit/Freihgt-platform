<script setup lang="ts">
import type { User } from '~/types/user'

const props = defineProps<{
  open: boolean
  loading?: boolean
  users: User[]
  usersLoading?: boolean
  reassign?: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [userId: string]
}>()

const selectedUserId = ref('')

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) selectedUserId.value = ''
  },
)

function onSubmit() {
  if (!selectedUserId.value) return
  emit('submit', selectedUserId.value)
}
</script>

<template>
  <UiModal
    :open="open"
    :title="reassign ? $t('controlTower.events.reassign') : $t('controlTower.events.assign')"
    @close="$emit('close')"
  >
    <label class="assign-modal__label">
      {{ $t('controlTower.events.assignedTo') }}
      <select v-model="selectedUserId" class="assign-modal__select" :disabled="usersLoading || loading">
        <option value="">{{ $t('controlTower.events.selectUser') }}</option>
        <option v-for="user in users" :key="user.id" :value="user.id">
          {{ user.full_name || user.email || user.id }}
        </option>
      </select>
    </label>

    <template #footer>
      <UiButton variant="secondary" :disabled="loading" @click="$emit('close')">
        {{ $t('common.cancel') }}
      </UiButton>
      <UiButton :loading="loading" :disabled="!selectedUserId || usersLoading" @click="onSubmit">
        {{ reassign ? $t('controlTower.events.reassign') : $t('controlTower.events.assign') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.assign-modal__label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.875rem;
}

.assign-modal__select {
  width: 100%;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
}
</style>
