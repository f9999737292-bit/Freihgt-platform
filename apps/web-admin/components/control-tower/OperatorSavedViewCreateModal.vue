<script setup lang="ts">
const props = defineProps<{
  open: boolean
  loading?: boolean
  canShare?: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [payload: { name: string; scope: 'private' | 'shared'; isDefault: boolean }]
}>()

const { t } = useI18n()
const name = ref('')
const scope = ref<'private' | 'shared'>('private')
const isDefault = ref(false)

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      name.value = ''
      scope.value = 'private'
      isDefault.value = false
    }
  },
)

function onSubmit() {
  const trimmed = name.value.trim()
  if (!trimmed) return
  emit('submit', { name: trimmed, scope: scope.value, isDefault: isDefault.value })
}
</script>

<template>
  <UiModal :open="open" :title="$t('controlTower.workspace.saveView')" @close="emit('close')">
    <label class="save-view-modal__field">
      {{ $t('controlTower.workspace.viewName') }}
      <input v-model="name" type="text" maxlength="128" required />
    </label>
    <fieldset class="save-view-modal__field">
      <legend>{{ $t('controlTower.workspace.scopeLabel') }}</legend>
      <label>
        <input v-model="scope" type="radio" value="private" />
        {{ $t('controlTower.workspace.private') }}
      </label>
      <label v-if="canShare">
        <input v-model="scope" type="radio" value="shared" />
        {{ $t('controlTower.workspace.shared') }}
      </label>
    </fieldset>
    <label class="save-view-modal__checkbox">
      <input v-model="isDefault" type="checkbox" />
      {{ $t('controlTower.workspace.setAsDefault') }}
    </label>
    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="loading" :disabled="!name.trim()" @click="onSubmit">
        {{ $t('common.save') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.save-view-modal__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}
.save-view-modal__checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}
</style>
