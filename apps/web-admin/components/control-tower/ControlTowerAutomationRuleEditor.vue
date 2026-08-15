<script setup lang="ts">
import type {
  AutomationExecutionMode,
  AutomationRule,
  ConditionClause,
  ConditionGroup,
  OperationalPlaybook,
} from '~/types/automation'
import {
  AUTOMATION_TRIGGER_TYPES,
  CONDITION_FIELDS,
} from '~/types/automation'

const CONDITION_OPERATORS = ['eq', 'neq', 'in', 'gte', 'lte', 'exists'] as const

const props = defineProps<{
  open: boolean
  rule?: AutomationRule | null
  playbooks: OperationalPlaybook[]
  saving?: boolean
}>()

const emit = defineEmits<{
  close: []
  save: [payload: {
    name: string
    description?: string
    triggerType: string
    executionMode: AutomationExecutionMode
    playbookId?: string
    priority: number
    conditions: ConditionGroup
  }]
}>()

const { t } = useI18n()

const name = ref('')
const description = ref('')
const triggerType = ref<string>(AUTOMATION_TRIGGER_TYPES[0])
const executionMode = ref<AutomationExecutionMode>('recommend')
const playbookId = ref('')
const priority = ref('100')
const conditionLogic = ref<'ALL' | 'ANY'>('ALL')
const conditions = ref<Array<{ field: string; operator: string; value: string }>>([])

const triggerOptions = computed(() =>
  AUTOMATION_TRIGGER_TYPES.map((value) => ({
    value,
    label: t(`controlTower.automation.triggers.${value}`, value),
  })),
)

const modeOptions = computed(() => [
  { value: 'observe', label: t('controlTower.automation.modes.observe') },
  { value: 'recommend', label: t('controlTower.automation.modes.recommend') },
])

const fieldOptions = computed(() =>
  CONDITION_FIELDS.map((value) => ({
    value,
    label: t(`controlTower.automation.conditionFields.${value}`, value),
  })),
)

const operatorOptions = computed(() =>
  CONDITION_OPERATORS.map((value) => ({
    value,
    label: t(`controlTower.automation.operators.${value}`, value),
  })),
)

const playbookOptions = computed(() => [
  { value: '', label: '—' },
  ...props.playbooks.map((pb) => ({ value: pb.id, label: pb.name })),
])

function formatConditionValue(operator: string, value: unknown): string {
  if (operator === 'exists' || value === undefined || value === null) return ''
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function resetForm() {
  if (props.rule) {
    name.value = props.rule.name
    description.value = props.rule.description ?? ''
    triggerType.value = props.rule.triggerType
    executionMode.value = props.rule.executionMode
    playbookId.value = props.rule.playbookId ?? ''
    priority.value = String(props.rule.priority)
    conditionLogic.value = props.rule.conditions.logic
    conditions.value = props.rule.conditions.conditions.map((clause) => ({
      field: clause.field,
      operator: clause.operator,
      value: formatConditionValue(clause.operator, clause.value),
    }))
  } else {
    name.value = ''
    description.value = ''
    triggerType.value = AUTOMATION_TRIGGER_TYPES[0]
    executionMode.value = 'recommend'
    playbookId.value = ''
    priority.value = '100'
    conditionLogic.value = 'ALL'
    conditions.value = [{ field: CONDITION_FIELDS[0], operator: 'eq', value: '' }]
  }
}

watch(
  () => [props.open, props.rule] as const,
  ([isOpen]) => {
    if (isOpen) resetForm()
  },
)

function addCondition() {
  conditions.value.push({ field: CONDITION_FIELDS[0], operator: 'eq', value: '' })
}

function removeCondition(index: number) {
  if (conditions.value.length <= 1) return
  conditions.value.splice(index, 1)
}

function serializeConditionValue(operator: string, raw: string): unknown {
  if (operator === 'exists') return undefined
  const trimmed = raw.trim()
  if (operator === 'in') {
    if (trimmed.startsWith('[')) {
      try {
        return JSON.parse(trimmed)
      } catch {
        return trimmed.split(',').map((part) => part.trim()).filter(Boolean)
      }
    }
    return trimmed.split(',').map((part) => part.trim()).filter(Boolean)
  }
  if (operator === 'gte' || operator === 'lte') {
    const parsed = Number(trimmed)
    return Number.isFinite(parsed) ? parsed : trimmed
  }
  return trimmed
}

function buildConditionGroup(): ConditionGroup {
  const clauses: ConditionClause[] = conditions.value.map((row) => ({
    field: row.field,
    operator: row.operator,
    value: serializeConditionValue(row.operator, row.value),
  }))
  return { logic: conditionLogic.value, conditions: clauses }
}

function onSubmit() {
  const trimmedName = name.value.trim()
  if (!trimmedName) return
  const parsedPriority = Number(priority.value)
  emit('save', {
    name: trimmedName,
    description: description.value.trim() || undefined,
    triggerType: triggerType.value,
    executionMode: executionMode.value,
    playbookId: playbookId.value || undefined,
    priority: Number.isFinite(parsedPriority) ? parsedPriority : 100,
    conditions: buildConditionGroup(),
  })
}

const editorTitle = computed(() =>
  props.rule
    ? t('controlTower.automation.editRule', 'Edit rule')
    : t('controlTower.automation.createRule', 'Create rule'),
)
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="automation-rule-editor">
      <div class="automation-rule-editor__backdrop" @click="emit('close')" />
      <aside
        class="automation-rule-editor__panel"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="'automation-rule-editor-title'"
      >
        <header class="automation-rule-editor__header">
          <h3 id="automation-rule-editor-title">{{ editorTitle }}</h3>
          <button type="button" class="automation-rule-editor__close" :aria-label="$t('common.close')" @click="emit('close')">
            ×
          </button>
        </header>

        <form class="automation-rule-editor__body" @submit.prevent="onSubmit">
          <UiInput v-model="name" :label="$t('controlTower.automation.ruleName')" required />
          <UiInput v-model="description" :label="$t('rfx.description')" />
          <UiSelect v-model="triggerType" :label="$t('controlTower.automation.trigger')" :options="triggerOptions" />
          <UiSelect v-model="executionMode" :label="$t('controlTower.automation.mode')" :options="modeOptions" />
          <UiSelect v-model="playbookId" :label="$t('controlTower.automation.playbook')" :options="playbookOptions" />
          <UiInput v-model="priority" type="number" :label="$t('controlTower.exceptions.priority', 'Priority')" />

          <fieldset class="automation-rule-editor__conditions">
            <legend>{{ $t('controlTower.automation.conditions') }}</legend>
            <UiSelect
              v-model="conditionLogic"
              :label="$t('controlTower.automation.conditionLogic', 'Match logic')"
              :options="[
                { value: 'ALL', label: $t('controlTower.automation.allConditions') },
                { value: 'ANY', label: $t('controlTower.automation.anyCondition') },
              ]"
            />

            <div
              v-for="(row, index) in conditions"
              :key="index"
              class="automation-rule-editor__condition-row"
            >
              <UiSelect v-model="row.field" :label="$t('lowCode.field')" :options="fieldOptions" />
              <UiSelect v-model="row.operator" :label="$t('controlTower.automation.operator', 'Operator')" :options="operatorOptions" />
              <UiInput
                v-if="row.operator !== 'exists'"
                v-model="row.value"
                :label="$t('lowCode.value')"
                :placeholder="row.operator === 'in' ? 'a, b, c' : ''"
              />
              <UiButton
                type="button"
                variant="ghost"
                size="sm"
                :disabled="conditions.length <= 1"
                @click="removeCondition(index)"
              >
                {{ $t('lowCode.removeField') }}
              </UiButton>
            </div>

            <UiButton type="button" variant="secondary" size="sm" @click="addCondition">
              {{ $t('lowCode.addField') }}
            </UiButton>
          </fieldset>
        </form>

        <footer class="automation-rule-editor__footer">
          <UiButton variant="secondary" @click="emit('close')">{{ $t('common.cancel') }}</UiButton>
          <UiButton :loading="saving" :disabled="!name.trim()" @click="onSubmit">
            {{ $t('common.save') }}
          </UiButton>
        </footer>
      </aside>
    </div>
  </Teleport>
</template>

<style scoped>
.automation-rule-editor {
  position: fixed;
  inset: 0;
  z-index: 1000;
}

.automation-rule-editor__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
}

.automation-rule-editor__panel {
  position: absolute;
  top: 0;
  right: 0;
  width: min(560px, 100vw);
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-surface);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.12);
}

.automation-rule-editor__header,
.automation-rule-editor__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--color-border);
}

.automation-rule-editor__footer {
  margin-top: auto;
  border-bottom: none;
  border-top: 1px solid var(--color-border);
  justify-content: flex-end;
}

.automation-rule-editor__body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
  overflow: auto;
}

.automation-rule-editor__close {
  border: none;
  background: transparent;
  font-size: 1.5rem;
  line-height: 1;
  cursor: pointer;
  color: var(--color-text-muted);
}

.automation-rule-editor__conditions {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 1rem;
}

.automation-rule-editor__condition-row {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--color-border);
}
</style>
