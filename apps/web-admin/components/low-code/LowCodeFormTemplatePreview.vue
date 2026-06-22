<script setup lang="ts">
import {
  collectConditionallyRequiredFields,
  countMissingRequiredPreviewFields,
  filterPreviewSectionsForVisibility,
  formatJsonValue,
  hasPreviewRuleContext,
  isPreviewComplexFieldType,
  previewCheckboxChecked,
  previewFieldValue,
  previewHasValue,
  previewMultiSelectValues,
  previewSelectOptions,
  previewValueToInputString,
  resolvePreviewFieldRequiredState,
  type FormTemplatePreviewField,
  type FormTemplatePreviewModel,
  type FormTemplatePreviewValues,
  type PreviewRuleContext,
} from '~/types/lowCode'

const props = withDefaults(
  defineProps<{
    template?: FormTemplatePreviewModel | null
    values?: FormTemplatePreviewValues
    readonly?: boolean
    title?: string
    previewContext?: PreviewRuleContext
  }>(),
  {
    template: null,
    values: undefined,
    readonly: true,
    title: undefined,
    previewContext: undefined,
  },
)

const { t } = useI18n()

const sortedSections = computed(() => {
  if (!props.template?.sections?.length) return []
  return [...props.template.sections].sort((a, b) => a.sort_order - b.sort_order || a.code.localeCompare(b.code))
})

const visibilityResult = computed(() =>
  filterPreviewSectionsForVisibility(sortedSections.value, props.values, props.previewContext),
)

const visibleSections = computed(() => visibilityResult.value.sections)
const hiddenFieldCount = computed(() => visibilityResult.value.hiddenFieldCount)
const showPreviewContext = computed(() => hasPreviewRuleContext(props.previewContext))

const conditionalRequiredFields = computed(() =>
  collectConditionallyRequiredFields(sortedSections.value, props.values, props.previewContext),
)

const missingRequiredCount = computed(() =>
  countMissingRequiredPreviewFields(visibleSections.value, conditionalRequiredFields.value, props.values),
)

function fieldRequiredState(field: FormTemplatePreviewField) {
  return resolvePreviewFieldRequiredState(field, conditionalRequiredFields.value, props.values)
}

function sortedFields(section: FormTemplatePreviewModel['sections'][number]) {
  return [...section.fields].sort((a, b) => a.sort_order - b.sort_order || a.code.localeCompare(b.code))
}

function fieldValue(fieldCode: string) {
  return previewFieldValue(props.values, fieldCode)
}

function displayValue(fieldType: string, fieldCode: string) {
  return previewValueToInputString(fieldType, fieldValue(fieldCode))
}

function hasValue(fieldCode: string) {
  return previewHasValue(fieldValue(fieldCode))
}

function selectOptionsFor(field: FormTemplatePreviewModel['sections'][number]['fields'][number]) {
  return previewSelectOptions(field.options_json)
}

function multiSelected(fieldCode: string) {
  return previewMultiSelectValues(fieldValue(fieldCode))
}

function moneyPreview(value: unknown): string {
  const parsed = value
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    const obj = parsed as { amount?: unknown; currency?: unknown }
    if (obj.amount !== undefined) {
      return `${obj.amount}${obj.currency ? ` ${obj.currency}` : ''}`
    }
  }
  return previewHasValue(value) ? formatJsonValue(value) : ''
}
</script>

<template>
  <UiCard class="form-preview">
    <template #header>
      <div class="form-preview__header">
        <span>{{ title || $t('lowCode.formPreview') }}</span>
        <span class="form-preview__badge">{{ $t('lowCode.previewOnly') }}</span>
      </div>
    </template>

    <div v-if="!template || !visibleSections.length" class="form-preview__empty">
      {{ $t('lowCode.noPreviewAvailable') }}
    </div>

    <div v-else class="form-preview__body">
      <p v-if="values" class="form-preview__hint">{{ $t('lowCode.currentValuesPreview') }}</p>
      <p v-if="hiddenFieldCount" class="form-preview__hint form-preview__hint--muted">
        {{ $t('lowCode.fieldsHiddenByVisibility', { count: hiddenFieldCount }) }}
      </p>
      <div v-if="showPreviewContext" class="form-preview__context">
        <span v-if="previewContext?.entity_status" class="form-preview__context-item">
          {{ $t('lowCode.previewEntityStatus') }}: <code>{{ previewContext.entity_status }}</code>
        </span>
        <span v-if="previewContext?.role" class="form-preview__context-item">
          {{ $t('lowCode.previewRole') }}: <code>{{ previewContext.role }}</code>
        </span>
      </div>
      <p v-if="missingRequiredCount" class="form-preview__hint form-preview__hint--warning">
        {{ $t('lowCode.missingRequiredValues', { count: missingRequiredCount }) }}
      </p>

      <section
        v-for="section in visibleSections"
        :key="section.code"
        class="form-preview__section"
      >
        <h3 class="form-preview__section-title">
          {{ section.title || section.code }}
          <span class="form-preview__section-code">({{ section.code }})</span>
        </h3>

        <div
          v-for="field in sortedFields(section)"
          :key="field.code"
          class="form-preview__field"
          :class="{ 'form-preview__field--missing-required': fieldRequiredState(field).isMissing }"
        >
          <div class="form-preview__field-meta">
            <label class="form-preview__label">{{ field.label || field.code }}</label>
            <div class="form-preview__badges">
              <span class="form-preview__code"><code>{{ field.code }}</code></span>
              <span class="form-preview__type">{{ field.field_type }}</span>
              <span
                v-if="field.required"
                class="form-preview__tag form-preview__tag--required"
              >
                {{ $t('lowCode.required') }}
              </span>
              <span
                v-else-if="fieldRequiredState(field).isConditional"
                class="form-preview__tag form-preview__tag--conditional-required"
              >
                {{ $t('lowCode.conditionalRequired') }}
              </span>
              <span v-if="field.read_only" class="form-preview__tag">
                {{ $t('lowCode.readOnly') }}
              </span>
              <span v-if="field.system_field" class="form-preview__tag">
                {{ $t('lowCode.systemField') }}
              </span>
            </div>
          </div>

          <p v-if="fieldRequiredState(field).isMissing" class="form-preview__missing-hint">
            {{ $t('lowCode.missingRequiredValue') }}
          </p>

          <div class="form-preview__control">
            <template v-if="field.field_type === 'CHECKBOX'">
              <label class="form-preview__checkbox">
                <input
                  type="checkbox"
                  :checked="previewCheckboxChecked(fieldValue(field.code))"
                  disabled
                  :aria-readonly="readonly"
                />
                <span>{{ field.label || field.code }}</span>
              </label>
            </template>

            <template v-else-if="field.field_type === 'SELECT'">
              <select
                class="form-preview__input"
                :value="displayValue(field.field_type, field.code)"
                disabled
                :aria-readonly="readonly"
              >
                <option value="" disabled>{{ hasValue(field.code) ? '' : $t('lowCode.noValue') }}</option>
                <option
                  v-for="option in selectOptionsFor(field)"
                  :key="option.value"
                  :value="option.value"
                >
                  {{ option.label }}
                </option>
              </select>
            </template>

            <template v-else-if="field.field_type === 'MULTI_SELECT'">
              <select
                class="form-preview__input form-preview__input--multi"
                multiple
                disabled
                :aria-readonly="readonly"
              >
                <option
                  v-for="option in selectOptionsFor(field)"
                  :key="option.value"
                  :value="option.value"
                  :selected="multiSelected(field.code).includes(option.value)"
                >
                  {{ option.label }}
                </option>
              </select>
              <p v-if="multiSelected(field.code).length" class="form-preview__selected">
                {{ multiSelected(field.code).join(', ') }}
              </p>
              <p v-else class="form-preview__placeholder">{{ $t('lowCode.noValue') }}</p>
            </template>

            <template v-else-if="field.field_type === 'FILE'">
              <div class="form-preview__placeholder form-preview__file">
                {{ hasValue(field.code) ? displayValue(field.field_type, field.code) : $t('lowCode.filePlaceholder') }}
              </div>
            </template>

            <template v-else-if="field.field_type === 'MONEY'">
              <input
                class="form-preview__input"
                type="text"
                :value="moneyPreview(fieldValue(field.code)) || $t('lowCode.noValue')"
                disabled
                :aria-readonly="readonly"
              />
            </template>

            <template v-else-if="isPreviewComplexFieldType(field.field_type)">
              <pre class="form-preview__json">{{
                hasValue(field.code) ? formatJsonValue(fieldValue(field.code)) : $t('lowCode.noValue')
              }}</pre>
            </template>

            <template v-else-if="['TEXT', 'NUMBER', 'DATE', 'DATETIME', 'CURRENCY', 'COMPANY_REFERENCE', 'DOCUMENT_REFERENCE'].includes(field.field_type)">
              <input
                class="form-preview__input"
                :type="field.field_type === 'NUMBER' ? 'number' : field.field_type === 'DATE' ? 'date' : field.field_type === 'DATETIME' ? 'datetime-local' : 'text'"
                :value="displayValue(field.field_type, field.code)"
                :placeholder="$t('lowCode.noValue')"
                disabled
                :aria-readonly="readonly"
              />
            </template>

            <template v-else>
              <pre class="form-preview__json">{{
                hasValue(field.code) ? formatJsonValue(fieldValue(field.code)) : $t('lowCode.unsupportedFieldType')
              }}</pre>
            </template>
          </div>
        </div>

        <p v-if="!section.fields.length" class="form-preview__placeholder">
          {{ $t('lowCode.noFieldsInSection') }}
        </p>
      </section>
    </div>
  </UiCard>
</template>

<style scoped>
.form-preview__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.form-preview__badge {
  font-size: 0.75rem;
  font-weight: 500;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  background: #eff6ff;
  color: #1d4ed8;
}

.form-preview__empty,
.form-preview__placeholder {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.form-preview__hint {
  margin: 0 0 1rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.form-preview__hint--muted {
  margin-top: -0.5rem;
  font-size: 0.8125rem;
}

.form-preview__hint--warning {
  color: #b45309;
}

.form-preview__field--missing-required .form-preview__input,
.form-preview__field--missing-required .form-preview__json,
.form-preview__field--missing-required .form-preview__file {
  border-color: #fca5a5;
}

.form-preview__missing-hint {
  margin: 0;
  font-size: 0.8125rem;
  color: #b91c1c;
}

.form-preview__context {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin: 0 0 1rem;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.form-preview__context-item code {
  font-size: 0.75rem;
}

.form-preview__body {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.form-preview__section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--color-border);
}

.form-preview__section:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.form-preview__section-title {
  margin: 0;
  font-size: 1rem;
}

.form-preview__section-code {
  font-size: 0.875rem;
  font-weight: 400;
  color: var(--color-text-muted);
}

.form-preview__field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-preview__field-meta {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.form-preview__label {
  font-weight: 500;
  font-size: 0.875rem;
}

.form-preview__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
  align-items: center;
  font-size: 0.75rem;
}

.form-preview__code code {
  font-size: 0.75rem;
}

.form-preview__type {
  color: var(--color-text-muted);
}

.form-preview__tag {
  padding: 0.0625rem 0.375rem;
  border-radius: var(--radius-sm);
  background: var(--color-surface-muted, #f8fafc);
  border: 1px solid var(--color-border);
}

.form-preview__tag--required {
  background: #fef2f2;
  border-color: #fecaca;
  color: #991b1b;
}

.form-preview__tag--conditional-required {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.form-preview__control {
  max-width: 520px;
}

.form-preview__input {
  width: 100%;
  min-height: 38px;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-muted, #f8fafc);
  font: inherit;
  color: var(--color-text);
}

.form-preview__input--multi {
  min-height: 88px;
}

.form-preview__input:disabled,
.form-preview__checkbox input:disabled {
  cursor: not-allowed;
  opacity: 1;
}

.form-preview__checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}

.form-preview__json {
  margin: 0;
  padding: 0.75rem;
  border-radius: var(--radius-md);
  background: var(--color-surface-muted, #f8fafc);
  border: 1px solid var(--color-border);
  font-size: 0.8125rem;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}

.form-preview__file {
  padding: 0.75rem;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface-muted, #f8fafc);
}

.form-preview__selected {
  margin: 0.375rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}
</style>
