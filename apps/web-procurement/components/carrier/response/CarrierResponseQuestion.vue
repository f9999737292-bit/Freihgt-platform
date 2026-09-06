<script setup lang="ts">
import type { CarrierQuestion, CarrierValidationErrorItem } from '~/types/carrierResponse'
import { isWave1CarrierQuestionType } from '~/types/carrierResponse'
import { localizeValidationMessage } from '~/utils/carrierResponseValidation'

const props = defineProps<{
  question: CarrierQuestion
  modelValue: unknown
  required: boolean
  disabled: boolean
  errors: CarrierValidationErrorItem[]
  autofocus?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const { t } = useI18n()
const inputRef = ref<HTMLElement | null>(null)

const supported = computed(() => isWave1CarrierQuestionType(props.question.question_type))

const errorMessages = computed(() =>
  props.errors.map((item) => localizeValidationMessage(item, t)),
)

function emitValue(value: unknown) {
  emit('update:modelValue', value)
}

function onTextInput(event: Event) {
  emitValue((event.target as HTMLInputElement).value)
}

function onNumberInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  emitValue(raw === '' ? null : Number(raw))
}

function onYesNo(value: boolean) {
  emitValue(value)
}

function onSingleSelect(event: Event) {
  emitValue((event.target as HTMLSelectElement).value || null)
}

function onMultiSelect(optionCode: string, checked: boolean) {
  const current = Array.isArray(props.modelValue) ? [...props.modelValue] : []
  if (checked) {
    if (!current.includes(optionCode)) current.push(optionCode)
  } else {
    const idx = current.indexOf(optionCode)
    if (idx >= 0) current.splice(idx, 1)
  }
  emitValue(current)
}

function isMultiChecked(code: string): boolean {
  return Array.isArray(props.modelValue) && props.modelValue.includes(code)
}

onMounted(() => {
  if (props.autofocus && inputRef.value) {
    inputRef.value.focus()
  }
})
</script>

<template>
  <div
    class="cr-question"
    :data-testid="`question-${question.question_code}`"
    :data-question-id="question.id"
  >
    <label class="cr-question__label">
      <span>{{ question.label }}</span>
      <span v-if="required" class="cr-question__required" :title="t('carrierResponse.requiredMark')">*</span>
    </label>
    <p v-if="question.help_text" class="cr-question__help">{{ question.help_text }}</p>

    <div v-if="!supported" class="cr-question__unsupported" data-testid="unsupported-question">
      <p>{{ t('carrierResponse.unsupportedType', { type: question.question_type }) }}</p>
      <p v-if="required" class="cr-question__unsupported-block">
        {{ t('carrierResponse.unsupportedRequired') }}
      </p>
    </div>

    <template v-else>
      <input
        v-if="question.question_type === 'TEXT'"
        ref="inputRef"
        class="cr-question__control"
        type="text"
        :disabled="disabled"
        :value="(modelValue as string) ?? ''"
        @input="onTextInput"
      >

      <textarea
        v-else-if="question.question_type === 'LONG_TEXT'"
        ref="inputRef"
        class="cr-question__control cr-question__control--textarea"
        :disabled="disabled"
        :value="(modelValue as string) ?? ''"
        rows="4"
        @input="onTextInput"
      />

      <input
        v-else-if="question.question_type === 'NUMBER'"
        ref="inputRef"
        class="cr-question__control"
        type="number"
        :disabled="disabled"
        :value="modelValue === null || modelValue === undefined ? '' : String(modelValue)"
        @input="onNumberInput"
      >

      <div v-else-if="question.question_type === 'YES_NO'" class="cr-question__yesno">
        <label>
          <input
            type="radio"
            :name="`yesno-${question.id}`"
            :disabled="disabled"
            :checked="modelValue === true"
            @change="onYesNo(true)"
          >
          {{ t('common.yes') }}
        </label>
        <label>
          <input
            type="radio"
            :name="`yesno-${question.id}`"
            :disabled="disabled"
            :checked="modelValue === false"
            @change="onYesNo(false)"
          >
          {{ t('common.no') }}
        </label>
      </div>

      <select
        v-else-if="question.question_type === 'SINGLE_SELECT'"
        ref="inputRef"
        class="cr-question__control"
        :disabled="disabled"
        :value="(modelValue as string) ?? ''"
        @change="onSingleSelect"
      >
        <option value="">—</option>
        <option
          v-for="opt in question.options ?? []"
          :key="opt.id"
          :value="opt.option_code"
        >
          {{ opt.label }}
        </option>
      </select>

      <div v-else-if="question.question_type === 'MULTI_SELECT'" class="cr-question__multiselect">
        <label v-for="opt in question.options ?? []" :key="opt.id">
          <input
            type="checkbox"
            :disabled="disabled"
            :checked="isMultiChecked(opt.option_code)"
            @change="onMultiSelect(opt.option_code, ($event.target as HTMLInputElement).checked)"
          >
          {{ opt.label }}
        </label>
      </div>

      <input
        v-else-if="question.question_type === 'DATE'"
        ref="inputRef"
        class="cr-question__control"
        type="date"
        :disabled="disabled"
        :value="(modelValue as string) ?? ''"
        @input="onTextInput"
      >
    </template>

    <ul v-if="errorMessages.length" class="cr-question__errors" data-testid="inline-errors">
      <li v-for="(msg, idx) in errorMessages" :key="idx">{{ msg }}</li>
    </ul>
  </div>
</template>

<style scoped>
.cr-question {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}
.cr-question__label {
  font-weight: 600;
  display: flex;
  gap: 0.25rem;
}
.cr-question__required {
  color: var(--color-danger, #c0392b);
}
.cr-question__help {
  margin: 0;
  color: var(--color-muted, #666);
  font-size: 0.875rem;
}
.cr-question__control {
  width: 100%;
  max-width: 32rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border, #ccc);
  border-radius: 4px;
}
.cr-question__control--textarea {
  min-height: 6rem;
  resize: vertical;
}
.cr-question__yesno,
.cr-question__multiselect {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
.cr-question__errors {
  margin: 0;
  padding-left: 1.25rem;
  color: var(--color-danger, #c0392b);
  font-size: 0.875rem;
}
.cr-question__unsupported {
  padding: 0.75rem;
  border: 1px dashed var(--color-border, #ccc);
  border-radius: 4px;
  background: var(--color-surface-muted, #f8f8f8);
}
.cr-question__unsupported-block {
  color: var(--color-danger, #c0392b);
  margin: 0.5rem 0 0;
}
</style>
