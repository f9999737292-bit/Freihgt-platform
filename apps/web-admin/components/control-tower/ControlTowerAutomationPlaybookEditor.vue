<script setup lang="ts">
import type { OperationalPlaybook, PlaybookStep } from '~/types/automation'
import { OPERATOR_ACTION_CODES } from '~/types/automation'

type EditableStep = {
  sequence: number
  title: string
  description: string
  stepType: PlaybookStep['stepType']
  required: boolean
  actionCode: string
}

const props = defineProps<{
  open: boolean
  playbook?: OperationalPlaybook | null
  saving?: boolean
}>()

const emit = defineEmits<{
  close: []
  save: [payload: { name: string; description?: string; steps: PlaybookStep[] }]
}>()

const { t } = useI18n()

const name = ref('')
const description = ref('')
const steps = ref<EditableStep[]>([])

const stepTypeOptions = computed(() => [
  { value: 'instruction', label: t('controlTower.automation.stepTypes.instruction', 'Instruction') },
  { value: 'checklist', label: t('controlTower.automation.stepTypes.checklist', 'Checklist') },
  { value: 'operator_action', label: t('controlTower.automation.stepTypes.operator_action', 'Operator action') },
])

const actionCodeOptions = computed(() =>
  OPERATOR_ACTION_CODES.map((value) => ({
    value,
    label: t(`controlTower.automation.actionCodes.${value}`, value),
  })),
)

function defaultStep(sequence: number): EditableStep {
  return {
    sequence,
    title: '',
    description: '',
    stepType: 'instruction',
    required: true,
    actionCode: OPERATOR_ACTION_CODES[0],
  }
}

function resetForm() {
  if (props.playbook) {
    name.value = props.playbook.name
    description.value = props.playbook.description ?? ''
    const sourceSteps = props.playbook.steps?.length
      ? props.playbook.steps
      : [{ sequence: 1, title: '', stepType: 'instruction' as const, required: true }]
    steps.value = sourceSteps.map((step, index) => ({
      sequence: step.sequence ?? index + 1,
      title: step.title,
      description: step.description ?? '',
      stepType: step.stepType,
      required: step.required ?? true,
      actionCode: step.actionCode ?? OPERATOR_ACTION_CODES[0],
    }))
  } else {
    name.value = ''
    description.value = ''
    steps.value = [defaultStep(1)]
  }
}

watch(
  () => [props.open, props.playbook] as const,
  ([isOpen]) => {
    if (isOpen) resetForm()
  },
)

function addStep() {
  steps.value.push(defaultStep(steps.value.length + 1))
}

function removeStep(index: number) {
  if (steps.value.length <= 1) return
  steps.value.splice(index, 1)
  steps.value.forEach((step, idx) => {
    step.sequence = idx + 1
  })
}

function buildSteps(): PlaybookStep[] {
  return steps.value.map((step, index) => ({
    sequence: index + 1,
    title: step.title.trim(),
    description: step.description.trim() || undefined,
    stepType: step.stepType,
    required: step.required,
    actionCode: step.stepType === 'operator_action' ? step.actionCode : undefined,
  }))
}

function onSubmit() {
  const trimmedName = name.value.trim()
  if (!trimmedName || steps.value.some((step) => !step.title.trim())) return
  emit('save', {
    name: trimmedName,
    description: description.value.trim() || undefined,
    steps: buildSteps(),
  })
}

const editorTitle = computed(() =>
  props.playbook
    ? t('controlTower.automation.editPlaybook', 'Edit playbook')
    : t('controlTower.automation.createPlaybook', 'Create playbook'),
)
</script>

<template>
  <UiModal :open="open" :title="editorTitle" @close="emit('close')">
    <div class="playbook-editor">
      <UiInput v-model="name" :label="$t('controlTower.automation.playbookName')" required />
      <UiInput v-model="description" :label="$t('rfx.description')" />

      <section class="playbook-editor__steps">
        <h4>{{ $t('controlTower.automation.stepCount') }}</h4>
        <article
          v-for="(step, index) in steps"
          :key="index"
          class="playbook-editor__step"
        >
          <UiInput
            v-model="step.title"
            :label="`${$t('controlTower.automation.startStep')} ${step.sequence}`"
            required
          />
          <UiInput v-model="step.description" :label="$t('rfx.description')" />
          <UiSelect v-model="step.stepType" :label="$t('common.type')" :options="stepTypeOptions" />
          <UiSelect
            v-if="step.stepType === 'operator_action'"
            v-model="step.actionCode"
            :label="$t('controlTower.automation.recommendedAction')"
            :options="actionCodeOptions"
          />
          <label class="playbook-editor__checkbox">
            <input v-model="step.required" type="checkbox">
            {{ $t('controlTower.automation.requiredStep') }}
          </label>
          <UiButton
            type="button"
            variant="ghost"
            size="sm"
            :disabled="steps.length <= 1"
            @click="removeStep(index)"
          >
            {{ $t('lowCode.removeField') }}
          </UiButton>
        </article>
        <UiButton type="button" variant="secondary" size="sm" @click="addStep">
          {{ $t('lowCode.addField') }}
        </UiButton>
      </section>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton
        :loading="saving"
        :disabled="!name.trim() || steps.some((step) => !step.title.trim())"
        @click="onSubmit"
      >
        {{ $t('common.save') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.playbook-editor {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.playbook-editor__steps {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.playbook-editor__step {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.playbook-editor__checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}
</style>
