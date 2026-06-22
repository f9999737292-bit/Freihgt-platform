<script setup lang="ts">
import {
  FORM_BUILDER_PALETTE_TYPES,
  LOW_CODE_ADMIN_ENTITY_TYPES,
  createEmptyDraftField,
  createEmptyDraftSection,
  createPaletteField,
  createPresetField,
  duplicateDraftField,
  formatJsonDraftText,
  nextFieldSortOrder,
  reindexFieldSortOrders,
  validateDraftTemplateDraft,
  validateJsonDraftText,
  type DraftFormTemplateDraft,
  type DraftEditorValidationIssue,
  type FormBuilderPresetId,
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
const activeSectionKey = ref('')
const collapsedSections = ref<Record<string, boolean>>({})
const jsonFeedback = ref<Record<string, { ok: boolean; message: string }>>({})
const confirmRemove = ref<{ type: 'section' | 'field'; sectionKey: string; fieldKey?: string } | null>(null)

const entityTypeOptions = computed(() =>
  LOW_CODE_ADMIN_ENTITY_TYPES.map((value) => ({ label: value, value })),
)

const fieldPresets: Array<{ id: FormBuilderPresetId; labelKey: string }> = [
  { id: 'text', labelKey: 'lowCode.presetTextField' },
  { id: 'select', labelKey: 'lowCode.presetSelectField' },
  { id: 'money', labelKey: 'lowCode.presetMoneyField' },
  { id: 'date', labelKey: 'lowCode.presetDateField' },
  { id: 'checkbox', labelKey: 'lowCode.presetCheckboxField' },
  { id: 'multi_select', labelKey: 'lowCode.presetMultiSelectField' },
  { id: 'phone_comment', labelKey: 'lowCode.presetPhoneCommentField' },
]

watch(
  () => props.modelValue.sections.map((section) => section._key).join(','),
  () => {
    if (!activeSectionKey.value && props.modelValue.sections[0]) {
      activeSectionKey.value = props.modelValue.sections[0]._key
    }
    if (!props.modelValue.sections.some((section) => section._key === activeSectionKey.value)) {
      activeSectionKey.value = props.modelValue.sections[0]?._key ?? ''
    }
  },
  { immediate: true },
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

function setSectionFields(sectionKey: string, fields: DraftFormTemplateDraft['sections'][number]['fields']) {
  updateSection(sectionKey, { fields: reindexFieldSortOrders(fields) })
}

function addSection() {
  const section = createEmptyDraftSection()
  updateDraft({ sections: [...props.modelValue.sections, section] })
  activeSectionKey.value = section._key
}

function requestRemoveSection(sectionKey: string) {
  if (props.modelValue.sections.length <= 1) return
  confirmRemove.value = { type: 'section', sectionKey }
}

function removeSection(sectionKey: string) {
  if (props.modelValue.sections.length <= 1) return
  updateDraft({
    sections: props.modelValue.sections.filter((section) => section._key !== sectionKey),
  })
  confirmRemove.value = null
}

function addField(sectionKey: string, field = createEmptyDraftField()) {
  const section = props.modelValue.sections.find((item) => item._key === sectionKey)
  if (!section) return
  const sortOrder = nextFieldSortOrder(section.fields)
  setSectionFields(sectionKey, [...section.fields, { ...field, sort_order: sortOrder }])
}

function addPaletteField(fieldType: string) {
  const sectionKey = activeSectionKey.value || props.modelValue.sections[0]?._key
  if (!sectionKey) return
  const section = props.modelValue.sections.find((item) => item._key === sectionKey)
  if (!section) return
  addField(sectionKey, createPaletteField(fieldType, nextFieldSortOrder(section.fields)))
}

function addPresetField(presetId: FormBuilderPresetId) {
  const sectionKey = activeSectionKey.value || props.modelValue.sections[0]?._key
  if (!sectionKey) return
  const section = props.modelValue.sections.find((item) => item._key === sectionKey)
  if (!section) return
  addField(sectionKey, createPresetField(presetId, nextFieldSortOrder(section.fields)))
}

function requestRemoveField(sectionKey: string, fieldKey: string) {
  confirmRemove.value = { type: 'field', sectionKey, fieldKey }
}

function removeField(sectionKey: string, fieldKey: string) {
  const section = props.modelValue.sections.find((item) => item._key === sectionKey)
  if (!section) return
  setSectionFields(
    sectionKey,
    section.fields.filter((field) => field._key !== fieldKey),
  )
  confirmRemove.value = null
}

function duplicateField(sectionKey: string, fieldKey: string) {
  const section = props.modelValue.sections.find((item) => item._key === sectionKey)
  if (!section) return
  const index = section.fields.findIndex((field) => field._key === fieldKey)
  if (index < 0) return
  const copy = duplicateDraftField(section.fields[index], nextFieldSortOrder(section.fields))
  const fields = [...section.fields]
  fields.splice(index + 1, 0, copy)
  setSectionFields(sectionKey, fields)
}

function moveField(sectionKey: string, fieldKey: string, direction: -1 | 1) {
  const section = props.modelValue.sections.find((item) => item._key === sectionKey)
  if (!section) return
  const index = section.fields.findIndex((field) => field._key === fieldKey)
  const target = index + direction
  if (index < 0 || target < 0 || target >= section.fields.length) return
  const fields = [...section.fields]
  const [item] = fields.splice(index, 1)
  fields.splice(target, 0, item)
  setSectionFields(sectionKey, fields)
}

function toggleSectionCollapsed(sectionKey: string) {
  collapsedSections.value = {
    ...collapsedSections.value,
    [sectionKey]: !collapsedSections.value[sectionKey],
  }
}

function isSectionCollapsed(sectionKey: string) {
  return collapsedSections.value[sectionKey] === true
}

function showOptionsJson(fieldType: string) {
  return fieldType === 'SELECT' || fieldType === 'MULTI_SELECT'
}

function jsonPath(sectionKey: string, fieldKey: string, kind: string) {
  return `${sectionKey}.${fieldKey}.${kind}`
}

function setJsonFeedback(path: string, ok: boolean, message: string) {
  jsonFeedback.value = { ...jsonFeedback.value, [path]: { ok, message } }
}

function formatJsonField(sectionKey: string, fieldKey: string, kind: 'options_json' | 'validation_rule_json' | 'visibility_rule_json') {
  const field = props.modelValue.sections
    .find((section) => section._key === sectionKey)
    ?.fields.find((item) => item._key === fieldKey)
  if (!field) return
  const key = `${kind}_text` as const
  const result = formatJsonDraftText(field[key])
  const path = jsonPath(sectionKey, fieldKey, kind)
  if (!result.ok) {
    setJsonFeedback(path, false, t('lowCode.invalidJson'))
    return
  }
  updateField(sectionKey, fieldKey, { [key]: result.value })
  setJsonFeedback(path, true, t('lowCode.jsonIsValid'))
}

function validateJsonField(sectionKey: string, fieldKey: string, kind: 'options_json' | 'validation_rule_json' | 'visibility_rule_json') {
  const field = props.modelValue.sections
    .find((section) => section._key === sectionKey)
    ?.fields.find((item) => item._key === fieldKey)
  if (!field) return
  const key = `${kind}_text` as const
  const path = jsonPath(sectionKey, fieldKey, kind)
  const result = validateJsonDraftText(field[key])
  setJsonFeedback(path, result.ok, result.ok ? t('lowCode.jsonIsValid') : t('lowCode.invalidJson'))
}

function confirmRemoval() {
  if (!confirmRemove.value) return
  if (confirmRemove.value.type === 'section') {
    removeSection(confirmRemove.value.sectionKey)
  } else if (confirmRemove.value.fieldKey) {
    removeField(confirmRemove.value.sectionKey, confirmRemove.value.fieldKey)
  }
}

function cancelRemoval() {
  confirmRemove.value = null
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
    sectionsRequired: t('lowCode.atLeastOneSectionRequired'),
    invalidJson: t('lowCode.invalidJson'),
    duplicateSectionCode: t('lowCode.duplicateSectionCode'),
    duplicateFieldCode: t('lowCode.duplicateFieldCode'),
    invalidOptionsJson: t('lowCode.invalidOptionsJson'),
  })
  validationIssues.value = issues
  return issues
}

function hasIssue(path: string) {
  return validationIssues.value.some((issue) => issue.path === path || issue.path.startsWith(`${path}.`))
}

function issueForPath(path: string) {
  return validationIssues.value.find((issue) => issue.path === path)?.message
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
          :class="{ 'template-editor__input--error': hasIssue('entity_type') }"
          @update:model-value="updateDraft({ entity_type: String($event) })"
        />
        <UiInput
          :model-value="modelValue.code"
          :label="$t('lowCode.code')"
          :placeholder="$t('lowCode.codePlaceholder')"
          :disabled="readonly || lockIdentity"
          :class="{ 'template-editor__input--error': hasIssue('code') }"
          @update:model-value="updateDraft({ code: $event })"
        />
        <UiInput
          :model-value="modelValue.name"
          :label="$t('common.name')"
          :disabled="readonly"
          :class="{ 'template-editor__input--error': hasIssue('name') }"
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

    <UiCard v-if="!readonly" class="template-editor__palette">
      <template #header>{{ $t('lowCode.fieldPalette') }}</template>
      <p class="template-editor__hint">{{ $t('lowCode.quickAddField') }}</p>
      <div class="template-editor__palette-grid">
        <UiButton
          v-for="fieldType in FORM_BUILDER_PALETTE_TYPES"
          :key="fieldType"
          size="sm"
          variant="secondary"
          @click="addPaletteField(fieldType)"
        >
          {{ fieldType }}
        </UiButton>
      </div>
      <div class="template-editor__presets">
        <strong>{{ $t('lowCode.fieldPresets') }}</strong>
        <div class="template-editor__palette-grid">
          <UiButton
            v-for="preset in fieldPresets"
            :key="preset.id"
            size="sm"
            variant="secondary"
            @click="addPresetField(preset.id)"
          >
            {{ $t(preset.labelKey) }}
          </UiButton>
        </div>
      </div>
      <p v-if="activeSectionKey" class="template-editor__hint">
        {{ $t('lowCode.addFieldToActiveSection') }}
      </p>
    </UiCard>

    <div v-for="(section, sectionIndex) in modelValue.sections" :key="section._key" class="template-editor__section">
      <UiCard>
        <template #header>
          <div class="template-editor__section-header">
            <button
              type="button"
              class="template-editor__collapse-btn"
              :aria-expanded="!isSectionCollapsed(section._key)"
              @click="toggleSectionCollapsed(section._key)"
            >
              {{ isSectionCollapsed(section._key) ? $t('lowCode.expand') : $t('lowCode.collapse') }}
            </button>
            <div class="template-editor__section-title">
              <span>{{ section.title || $t('lowCode.section') }} {{ sectionIndex + 1 }}</span>
              <span class="template-editor__field-count">
                {{ $t('lowCode.fieldCount', { count: section.fields.length }) }}
              </span>
            </div>
            <div class="template-editor__section-actions">
              <UiButton
                size="sm"
                :variant="activeSectionKey === section._key ? 'primary' : 'secondary'"
                @click="activeSectionKey = section._key"
              >
                {{ $t('lowCode.activeSection') }}
              </UiButton>
              <UiButton
                v-if="!readonly"
                size="sm"
                variant="secondary"
                @click="requestRemoveSection(section._key)"
              >
                {{ $t('lowCode.removeSection') }}
              </UiButton>
            </div>
          </div>
        </template>

        <div v-show="!isSectionCollapsed(section._key)">
          <div class="template-editor__grid">
            <UiInput
              :model-value="section.code"
              :label="$t('lowCode.sectionCode')"
              :disabled="readonly"
              :class="{ 'template-editor__input--error': hasIssue(`sections.${sectionIndex}.code`) }"
              @update:model-value="updateSection(section._key, { code: $event })"
            />
            <p v-if="issueForPath(`sections.${sectionIndex}.code`)" class="template-editor__inline-error">
              {{ issueForPath(`sections.${sectionIndex}.code`) }}
            </p>
            <UiInput
              :model-value="section.title"
              :label="$t('lowCode.sectionTitle')"
              :disabled="readonly"
              :class="{ 'template-editor__input--error': hasIssue(`sections.${sectionIndex}.title`) }"
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
                <div class="template-editor__field-summary">
                  <code>{{ field.code || '—' }}</code>
                  <span>{{ field.label || '—' }}</span>
                  <UiBadge status="neutral" tone="neutral">{{ field.field_type }}</UiBadge>
                  <UiBadge v-if="field.required" status="warning" tone="neutral">{{ $t('lowCode.required') }}</UiBadge>
                  <UiBadge v-if="field.read_only" status="read-only" tone="neutral">{{ $t('lowCode.readOnly') }}</UiBadge>
                  <UiBadge v-if="field.system_field" status="neutral" tone="neutral">{{ $t('lowCode.systemField') }}</UiBadge>
                </div>
                <div v-if="!readonly" class="template-editor__field-actions">
                  <UiButton size="sm" variant="secondary" :disabled="fieldIndex === 0" @click="moveField(section._key, field._key, -1)">
                    {{ $t('lowCode.moveUp') }}
                  </UiButton>
                  <UiButton
                    size="sm"
                    variant="secondary"
                    :disabled="fieldIndex === section.fields.length - 1"
                    @click="moveField(section._key, field._key, 1)"
                  >
                    {{ $t('lowCode.moveDown') }}
                  </UiButton>
                  <UiButton size="sm" variant="secondary" @click="duplicateField(section._key, field._key)">
                    {{ $t('lowCode.duplicateField') }}
                  </UiButton>
                  <UiButton size="sm" variant="secondary" @click="requestRemoveField(section._key, field._key)">
                    {{ $t('lowCode.removeField') }}
                  </UiButton>
                </div>
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
                  :options="FORM_BUILDER_PALETTE_TYPES.map((value) => ({ label: value, value }))"
                  :disabled="readonly"
                  @update:model-value="updateField(section._key, field._key, { field_type: String($event) })"
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

              <div v-if="showOptionsJson(field.field_type)" class="template-editor__json-block">
                <label class="template-editor__textarea">
                  <span>{{ $t('lowCode.optionsJsonLabel') }}</span>
                  <textarea
                    :value="field.options_json_text"
                    rows="4"
                    :readonly="readonly"
                    @input="updateField(section._key, field._key, { options_json_text: ($event.target as HTMLTextAreaElement).value })"
                  />
                </label>
                <div v-if="!readonly" class="template-editor__json-actions">
                  <UiButton size="sm" variant="secondary" @click="formatJsonField(section._key, field._key, 'options_json')">
                    {{ $t('lowCode.formatJson') }}
                  </UiButton>
                  <UiButton size="sm" variant="secondary" @click="validateJsonField(section._key, field._key, 'options_json')">
                    {{ $t('lowCode.validateJson') }}
                  </UiButton>
                </div>
                <p
                  v-if="jsonFeedback[jsonPath(section._key, field._key, 'options_json')]"
                  class="template-editor__json-feedback"
                  :class="{ 'template-editor__json-feedback--error': !jsonFeedback[jsonPath(section._key, field._key, 'options_json')].ok }"
                >
                  {{ jsonFeedback[jsonPath(section._key, field._key, 'options_json')].message }}
                </p>
              </div>

              <div class="template-editor__json-block">
                <label class="template-editor__textarea">
                  <span>{{ $t('lowCode.validationRuleJsonLabel') }}</span>
                  <textarea
                    :value="field.validation_rule_json_text"
                    rows="3"
                    :readonly="readonly"
                    @input="updateField(section._key, field._key, { validation_rule_json_text: ($event.target as HTMLTextAreaElement).value })"
                  />
                </label>
                <div v-if="!readonly" class="template-editor__json-actions">
                  <UiButton size="sm" variant="secondary" @click="formatJsonField(section._key, field._key, 'validation_rule_json')">
                    {{ $t('lowCode.formatJson') }}
                  </UiButton>
                  <UiButton size="sm" variant="secondary" @click="validateJsonField(section._key, field._key, 'validation_rule_json')">
                    {{ $t('lowCode.validateJson') }}
                  </UiButton>
                </div>
                <p
                  v-if="jsonFeedback[jsonPath(section._key, field._key, 'validation_rule_json')]"
                  class="template-editor__json-feedback"
                  :class="{ 'template-editor__json-feedback--error': !jsonFeedback[jsonPath(section._key, field._key, 'validation_rule_json')].ok }"
                >
                  {{ jsonFeedback[jsonPath(section._key, field._key, 'validation_rule_json')].message }}
                </p>
              </div>

              <div class="template-editor__json-block">
                <label class="template-editor__textarea">
                  <span>{{ $t('lowCode.visibilityRuleJsonLabel') }}</span>
                  <textarea
                    :value="field.visibility_rule_json_text"
                    rows="3"
                    :readonly="readonly"
                    @input="updateField(section._key, field._key, { visibility_rule_json_text: ($event.target as HTMLTextAreaElement).value })"
                  />
                </label>
                <div v-if="!readonly" class="template-editor__json-actions">
                  <UiButton size="sm" variant="secondary" @click="formatJsonField(section._key, field._key, 'visibility_rule_json')">
                    {{ $t('lowCode.formatJson') }}
                  </UiButton>
                  <UiButton size="sm" variant="secondary" @click="validateJsonField(section._key, field._key, 'visibility_rule_json')">
                    {{ $t('lowCode.validateJson') }}
                  </UiButton>
                </div>
                <p
                  v-if="jsonFeedback[jsonPath(section._key, field._key, 'visibility_rule_json')]"
                  class="template-editor__json-feedback"
                  :class="{ 'template-editor__json-feedback--error': !jsonFeedback[jsonPath(section._key, field._key, 'visibility_rule_json')].ok }"
                >
                  {{ jsonFeedback[jsonPath(section._key, field._key, 'visibility_rule_json')].message }}
                </p>
              </div>
            </div>

            <p v-if="section.fields.length === 0" class="template-editor__hint">
              {{ $t('lowCode.noFieldsInSection') }}
            </p>
          </div>
        </div>
      </UiCard>
    </div>

    <UiButton v-if="!readonly" variant="secondary" @click="addSection">
      {{ $t('lowCode.addSection') }}
    </UiButton>

    <UiModal
      :open="!!confirmRemove"
      :title="confirmRemove?.type === 'section' ? $t('lowCode.removeSection') : $t('lowCode.removeField')"
      @close="cancelRemoval"
    >
      <p>{{ $t('lowCode.confirmRemoveMessage') }}</p>
      <template #footer>
        <UiButton variant="secondary" @click="cancelRemoval">{{ $t('common.cancel') }}</UiButton>
        <UiButton @click="confirmRemoval">{{ $t('lowCode.confirmRemove') }}</UiButton>
      </template>
    </UiModal>
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

.template-editor__palette-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.template-editor__presets {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.template-editor__section-header,
.template-editor__fields-header,
.template-editor__field-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.template-editor__section-title {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
}

.template-editor__field-count {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.template-editor__section-actions,
.template-editor__field-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.template-editor__collapse-btn {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  border-radius: var(--radius-md);
  padding: 0.25rem 0.625rem;
  font-size: 0.8125rem;
  cursor: pointer;
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

.template-editor__field-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
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
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
  resize: vertical;
}

.template-editor__json-block {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.template-editor__json-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.template-editor__json-feedback {
  margin: 0;
  font-size: 0.8125rem;
  color: #166534;
}

.template-editor__json-feedback--error {
  color: #991b1b;
}

.template-editor__inline-error {
  margin: -0.5rem 0 0;
  grid-column: 1 / -1;
  font-size: 0.8125rem;
  color: #991b1b;
}

.template-editor__hint {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.template-editor__input--error :deep(input),
.template-editor__input--error :deep(select) {
  border-color: #f87171;
}
</style>
