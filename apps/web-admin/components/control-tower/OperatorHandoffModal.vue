<script setup lang="ts">
import type { User } from '~/types/user'
import type { ControlTowerHandoffCreateResult, ControlTowerWorkItem } from '~/types/controlTower'

const props = defineProps<{
  open: boolean
  items: ControlTowerWorkItem[]
  users: User[]
  usersLoading?: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: [toUserId: string, note: string | undefined]
  done: [result: ControlTowerHandoffCreateResult]
}>()

const { t } = useI18n()
const step = ref<'form' | 'preview' | 'result'>('form')
const selectedUserId = ref('')
const note = ref('')
const result = ref<ControlTowerHandoffCreateResult | null>(null)

const preview = computed(() => {
  const items = props.items
  return {
    total: items.length,
    exceptions: items.filter((i) => i.itemType === 'exception').length,
    risks: items.filter((i) => i.itemType === 'risk').length,
    critical: items.filter((i) => i.urgency === 'critical').length,
    slaBreached: items.filter((i) => i.slaStatus === 'breached').length,
  }
})

const selectedUser = computed(() => props.users.find((u) => u.id === selectedUserId.value))

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      step.value = 'form'
      selectedUserId.value = ''
      note.value = ''
      result.value = null
    }
  },
)

function goPreview() {
  if (!selectedUserId.value) return
  step.value = 'preview'
}

function onConfirm() {
  emit('confirm', selectedUserId.value, note.value.trim() || undefined)
}
</script>

<template>
  <UiModal
    :open="open"
    :title="$t('controlTower.workspace.handoffSelectedWork')"
    @close="emit('close')"
  >
    <template v-if="step === 'form'">
      <label class="handoff-modal__field">
        {{ $t('controlTower.workspace.recipient') }}
        <select v-model="selectedUserId" class="handoff-modal__select" :disabled="usersLoading || loading">
          <option value="">{{ $t('controlTower.events.selectUser') }}</option>
          <option v-for="user in users" :key="user.id" :value="user.id">
            {{ user.full_name || user.email }}
          </option>
        </select>
      </label>
      <label class="handoff-modal__field">
        {{ $t('controlTower.workspace.handoffNote') }}
        <textarea v-model="note" rows="3" maxlength="2000" class="handoff-modal__textarea" />
      </label>
      <p class="handoff-modal__hint">
        {{ $t('controlTower.workspace.handoffSelectedCount', { count: preview.total }) }}
      </p>
    </template>

    <template v-else-if="step === 'preview'">
      <h4 class="handoff-modal__preview-title">{{ $t('controlTower.workspace.previewTransfer') }}</h4>
      <dl class="handoff-modal__preview">
        <dt>{{ $t('controlTower.workspace.recipient') }}</dt>
        <dd>{{ selectedUser?.full_name || selectedUser?.email }}</dd>
        <dt>{{ $t('controlTower.workspace.selected') }}</dt>
        <dd>{{ preview.total }}</dd>
        <dt>{{ $t('controlTower.workspace.actualException') }}</dt>
        <dd>{{ preview.exceptions }}</dd>
        <dt>{{ $t('controlTower.workspace.predictiveRisk') }}</dt>
        <dd>{{ preview.risks }}</dd>
        <dt>{{ $t('controlTower.workspace.presets.critical') }}</dt>
        <dd>{{ preview.critical }}</dd>
        <dt>{{ $t('controlTower.workspace.slaBreached') }}</dt>
        <dd>{{ preview.slaBreached }}</dd>
        <template v-if="note.trim()">
          <dt>{{ $t('controlTower.workspace.handoffNote') }}</dt>
          <dd>{{ note.trim() }}</dd>
        </template>
      </dl>
    </template>

    <template v-else-if="result">
      <p>{{ $t('controlTower.workspace.transferred', { count: result.outcome.succeeded }) }}</p>
      <p v-if="result.outcome.failed > 0" class="handoff-modal__warn">
        {{ $t('controlTower.workspace.failed', { count: result.outcome.failed }) }}
      </p>
    </template>

    <template #footer>
      <UiButton variant="secondary" :disabled="loading" @click="emit('close')">
        {{ $t('common.cancel') }}
      </UiButton>
      <UiButton v-if="step === 'form'" :disabled="!selectedUserId" @click="goPreview">
        {{ $t('controlTower.workspace.previewTransfer') }}
      </UiButton>
      <UiButton v-if="step === 'preview'" variant="secondary" @click="step = 'form'">
        {{ $t('common.back') }}
      </UiButton>
      <UiButton v-if="step === 'preview'" :loading="loading" @click="onConfirm">
        {{ $t('controlTower.workspace.confirmHandoff') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.handoff-modal__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}
.handoff-modal__select,
.handoff-modal__textarea {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: var(--radius-sm, 4px);
}
.handoff-modal__hint {
  font-size: 0.875rem;
  color: var(--color-text-muted, #666);
}
.handoff-modal__preview {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 1rem;
  font-size: 0.875rem;
}
.handoff-modal__preview-title {
  margin: 0 0 0.75rem;
}
.handoff-modal__warn {
  color: var(--color-warning-text, #9a6700);
}
</style>
