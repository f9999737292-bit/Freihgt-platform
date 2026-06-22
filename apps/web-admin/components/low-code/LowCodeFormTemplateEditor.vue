<script setup lang="ts">
import {
  LOW_CODE_ADMIN_ENTITY_TYPES,
  LOW_CODE_FIELD_TYPES,
  createEmptyDraftField,
  createEmptyDraftSection,
  validateDraftTemplateDraft,
  type DraftFormTemplateDraft,
  type DraftEditorValidationIssue,
} from '~/types/lowCode'

const props = withDefaults(
  defineProps<{
    modelValue: DraftFormTemplateDraft
    readonly?: boolean
    lockIdentity?: boolean
  }>(),
  {
    readonly: false,
    lockIdentity: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: DraftFormTemplateDraft]
}>()

const { t } = useI18n()

const validationIssues = ref<DraftEditorValidationIssue[]>([])

const entityTypeOptions = computed(() =>
  LOW_CODE_ADMIN_ENTITY_TYPES.map((value) => ({ label: value, value })),
)

const fieldTypeOptions = computed(() =>
  LOW_CODE_FIELD_TYPES.map((value) => ({ label: value, value })),
)

function updateDraft(patch: Partial<DraftFormTemplateDraft>) {
  emit('update:modelValue', { ...props.modelValue, ...patch })
}

function updateSection(sectionKey: string, patch: Partial<DraftFormTemplateDraft['sections'][number]>) {
  updateDraft({
    sections: props.modelValue.sections.map((section) =>
      section._key === sectionKey ? { ...section, ...patch } : section,
    ),
  })
}

function updateField(
  sectionKey: string,
  fieldKey: string,
  patch: Partial<DraftFormTemplateDraft['sections'][number]['fields'][number]>,
) {
  updateDraft({
    sections: props.modelValue.sections.map((section) => {
      if (section._key !== sectionKey) return section
      return {
        ...section,
        fields: section.fields.map((field) =>
          field._key === fieldKey ? { ...field, ...patch } : field,
        ),
      }
    }),
  })
}

function addSection() {
  updateDraft({
    sections: [...props.modelValue.sections, createEmptyDraftSection()],
  })
}

function removeSection(sectionKey: string) {
  if (props.modelValue.sections.length <= 1) return
  updateDraft({
    sections: props.modelValue.sections.filter((section) => section._key !== sectionKey),
  })
}

function addField(sectionKey: string) {
  updateDraft({
    sections: props.modelValue.sections.map((section) =>
      section._key === sectionKey
        ? { ...section, fields: [...section.fields, createEmptyDraftField()] }
        : section,
    ),
  })
}

function removeField(sectionKey: string, fieldKey: string) {
  updateDraft({
    sections: props.modelValue.sections.map((section) =>
      section._key === sectionKey
        ? { ...section, fields: section.fields.filter((field) => field._key !== fieldKey) }
        : section,
    ),
  })
}

function showOptionsJson(fieldType: string) {
  return fieldType === 'SELECT' || fieldType === 'MULTI_SELECT'
}

function validate(): DraftEditorValidationIssue[] {
  const issues = validateDraftTemplateDraft(props.modelValue, {
    entityTypeRequired: t('lowCode.entityTypeRequired'),
    codeRequired: t('lowCode.codeRequired'),
    nameRequired: t('lowCode.nameRequired'),
    sectionCodeRequired: t('lowCode.sectionCodeRequired'),
    sectionTitleRequired: t('lowCode.sectionTitleRequired'),
    fieldCodeRequired: t('lowCode.fieldCodeRequired'),
    fieldLabelRequired: t('lowCode.fieldLabelRequired'),
    fieldTypeRequired: t('lowCode.fieldTypeRequired'),
    sectionsRequired: t('lowCode.sectionsRequired'),
    fieldsRequired: t('lowCode.fieldsRequired'),
    invalidJson: t('lowCode.invalidJson'),
  })
  validationIssues.value = issues
  return issues
}

function hasIssue(path: string) {
  return validationIssues.value.some((issue) => issue.path === path || issue.path.startsWith(`${path}.`))
}

defineExpose({ validate })
</script>

<template>
  <div class="template-editor">
    <div v-if="validationIssues.length" class="template-editor__errors" role="alert">
      <strong>{{ $t('lowCode.fixValidationIssues') }}</strong>
      <ul>
        <li v-for="issue in validationIssues" :key="issue.path">{{ issue.message }}</li>
      </ul>
    </div>

    <UiCard>
      <template #header>{{ $t('lowCode.templateMetadata') }}</template>
      <div class="template-editor__grid">
        <UiSelect
          :model-value="modelValue.entity_type"
          :label="$t('lowCode.entityType')"
          :options="entityTypeOptions"
          :disabled="readonly || lockIdentity"
          @update:model-value="updateDraft({ entity_type: String($event) })"
        />
        <UiInput
          :model-value="modelValue.code"
          :label="$t('lowCode.code')"
          :placeholder="$t('lowCode.codePlaceholder')"
          :disabled="readonly || lockIdentity"
          @update:model-value="updateDraft({ code: $event })"
        />
        <UiInput
          :model-value="modelValue.name"
          :label="$t('common.name')"
          :disabled="readonly"
          @update:model-value="updateDraft({ name: $event })"
        />
        <UiInput
          :model-value="modelValue.description"
          :label="$t('lowCode.description')"
          :disabled="readonly"
          @update:model-value="updateDraft({ description: $event })"
        />
      </div>
    </UiCard>

    <div v-for="(section, sectionIndex) in modelValue.sections" :key="section._key" class="template-editor__section">
      <UiCard>
        <template #header>
          <div class="template-editor__section-header">
            <span>{{ $t('lowCode.section') }} {{ sectionIndex + 1 }}</span>
            <UiButton
              v-if="!readonly && modelValue.sections.length > 1"
              size="sm"
              variant="secondary"
              @click="removeSection(section._key)"
            >
              {{ $t('lowCode.removeSection') }}
            </UiButton>
          </div>
        </template>

        <div class="template-editor__grid">
          <UiInput
            :model-value="section.code"
            :label="$t('lowCode.sectionCode')"
            :disabled="readonly"
            @update:model-value="updateSection(section._key, { code: $event })"
          />
          <UiInput
            :model-value="section.title"
            :label="$t('lowCode.sectionTitle')"
            :disabled="readonly"
            @update:model-value="updateSection(section._key, { title: $event })"
          />
          <UiInput
            :model-value="String(section.sort_order)"
            :label="$t('lowCode.sortOrder')"
            type="number"
            :disabled="readonly"
            @update:model-value="updateSection(section._key, { sort_order: Number($event) || 0 })"
          />
        </div>

        <div class="template-editor__fields">
          <div class="template-editor__fields-header">
            <strong>{{ $t('lowCode.fields') }}</strong>
            <UiButton v-if="!readonly" size="sm" variant="secondary" @click="addField(section._key)">
              {{ $t('lowCode.addField') }}
            </UiButton>
          </div>

          <div
            v-for="(field, fieldIndex) in section.fields"
            :key="field._key"
            class="template-editor__field"
            :class="{ 'template-editor__field--error': hasIssue(`sections.${sectionIndex}.fields.${fieldIndex}`) }"
          >
            <div class="template-editor__field-header">
              <strong>{{ $t('lowCode.field') }} {{ fieldIndex + 1 }}</strong>
              <UiButton
                v-if="!readonly"
                size="sm"
                variant="secondary"
                @click="removeField(section._key, field._key)"
              >
                {{ $t('lowCode.removeField') }}
              </UiButton>
            </div>

            <div class="template-editor__grid">
              <UiInput
                :model-value="field.code"
                :label="$t('lowCode.fieldCode')"
                :disabled="readonly"
                @update:model-value="updateField(section._key, field._key, { code: $event })"
              />
              <UiInput
                :model-value="field.label"
                :label="$t('lowCode.label')"
                :disabled="readonly"
                @update:model-value="updateField(section._key, field._key, { label: $event })"
              />
              <UiSelect
                :model-value="field.field_type"
                :label="$t('lowCode.fieldType')"
                :options="fieldTypeOptions"
                :disabled="readonly"
                @update:model-value="updateField(section._key, field._key, { field_type: String($event) })"
              />
              <UiInput
                :model-value="String(field.sort_order)"
                :label="$t('lowCode.sortOrder')"
                type="number"
                :disabled="readonly"
                @update:model-value="updateField(section._key, field._key, { sort_order: Number($event) || 0 })"
              />
            </div>

            <div class="template-editor__checkboxes">
              <label class="template-editor__checkbox">
                <input
                  :checked="field.required"
                  type="checkbox"
                  :disabled="readonly"
                  @change="updateField(section._key, field._key, { required: ($event.target as HTMLInputElement).checked })"
                />
                {{ $t('lowCode.required') }}
              </label>
              <label class="template-editor__checkbox">
                <input
                  :checked="field.read_only"
                  type="checkbox"
                  :disabled="readonly"
                  @change="updateField(section._key, field._key, { read_only: ($event.target as HTMLInputElement).checked })"
                />
                {{ $t('lowCode.readOnly') }}
              </label>
            </div>

            <label v-if="showOptionsJson(field.field_type)" class="template-editor__textarea">
              <span>{{ $t('lowCode.optionsJsonLabel') }}</span>
              <textarea
                :value="field.options_json_text"
                rows="4"
                :readonly="readonly"
                placeholder='{"options":["GENERAL","DANGEROUS"]}'
                @input="updateField(section._key, field._key, { options_json_text: ($event.target as HTMLTextAreaElement).value })"
              />
            </label>

            <label class="template-editor__textarea">
              <span>{{ $t('lowCode.validationRuleJsonLabel') }}</span>
              <textarea
                :value="field.validation_rule_json_text"
                rows="3"
                :readonly="readonly"
                @input="updateField(section._key, field._key, { validation_rule_json_text: ($event.target as HTMLTextAreaElement).value })"
              />
            </label>

            <label class="template-editor__textarea">
              <span>{{ $t('lowCode.visibilityRuleJsonLabel') }}</span>
              <textarea
                :value="field.visibility_rule_json_text"
                rows="3"
                :readonly="readonly"
                @input="updateField(section._key, field._key, { visibility_rule_json_text: ($event.target as HTMLTextAreaElement).value })"
              />
            </label>
          </div>

          <p v-if="section.fields.length === 0" class="template-editor__hint">
            {{ $t('lowCode.noFieldsInSection') }}
          </p>
        </div>
      </UiCard>
    </div>

    <UiButton v-if="!readonly" variant="secondary" @click="addSection">
      {{ $t('lowCode.addSection') }}
    </UiButton>
  </div>
</template>

<style scoped>
.template-editor {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.template-editor__errors {
  padding: 1rem 1.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid #fecaca;
  background: #fef2f2;
  color: #991b1b;
}

.template-editor__errors ul {
  margin: 0.5rem 0 0;
  padding-left: 1.25rem;
}

.template-editor__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
}

.template-editor__section-header,
.template-editor__fields-header,
.template-editor__field-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.template-editor__fields {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.template-editor__field {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.template-editor__field--error {
  border-color: #f87171;
}

.template-editor__checkboxes {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}

.template-editor__checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}

.template-editor__textarea {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  font-size: 0.875rem;
}

.template-editor__textarea textarea {
  width: 100%;
  min-height: 80px;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  font: inherit;
  resize: vertical;
}

.template-editor__hint {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
