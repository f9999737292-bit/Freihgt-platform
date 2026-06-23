<script setup lang="ts">

import {

  buildCustomFieldValuesEditorLink,

  customFieldValuesToPreviewMap,

  flattenFormFields,

  formTemplateDetailToPreview,

  formatCustomFieldDisplayValue,

  formatJsonValue,

  formatLowCodeDate,

  isCustomFieldComplexValue,

  moneyDraftKeys,

  parseEditDraftToValueJson,

  parseSelectOptions,

  seedEditDraftForField,

  usesJsonTextareaFallback,

  type FormField,

  type FormTemplatePreviewModel,

  type LowCodeEntityType,

  type PreviewRuleContext,

} from '~/types/lowCode'

import type { LowCodeValidationContext } from '~/utils/lowCodeValidationContext'

import { compactValidationContextForPut, mergeLowCodeValidationContext } from '~/utils/lowCodeValidationContext'

import { TenantRequiredError } from '~/composables/useApi'



const emit = defineEmits<{

  saved: []

}>()



const props = withDefaults(

  defineProps<{

    entityType: LowCodeEntityType | string

    entityId?: string | null

    entityStatus?: string | null

    title?: string

    editable?: boolean

    showPreview?: boolean

    showFullEditorLink?: boolean

    previewContext?: PreviewRuleContext

    validationContext?: LowCodeValidationContext | PreviewRuleContext | null

  }>(),

  {

    entityId: null,

    entityStatus: null,

    title: undefined,

    editable: false,

    showPreview: true,

    showFullEditorLink: false,

    previewContext: undefined,

    validationContext: undefined,

  },

)



const {

  getCustomFieldValues,

  saveCustomFieldValues,

  resolvePublishedTemplate,

  getSaveErrorMessage,

  isApiUnavailableError,

} = useLowCodeApi()

const { hasTenant } = useTenantContext()

const { buildPreviewContext } = useLowCodePreviewContext()

const { pushToast } = useToast()

const { t } = useI18n()



const loading = ref(false)

const loaded = ref(false)

const loadFailed = ref(false)

const templateLoading = ref(false)

const saving = ref(false)

const editing = ref(false)

const formTemplateId = ref<string | null>(null)

const previewTemplate = ref<FormTemplatePreviewModel | null>(null)

const fieldsByCode = ref<Record<string, FormField>>({})

const items = ref<Array<{ field_id: string; field_code: string; value_json: unknown; updated_at: string }>>([])

const editDraft = ref<Record<string, string>>({})

const editMultiDraft = ref<Record<string, string[]>>({})



const panelTitle = computed(() => props.title || t('lowCode.customFields'))

const canLoad = computed(() => hasTenant.value && !!props.entityId?.trim())

const canEdit = computed(

  () => props.editable && !loadFailed.value && !loading.value && !!formTemplateId.value,

)

const editDisabled = computed(() => loadFailed.value || loading.value || templateLoading.value)

const editButtonLabel = computed(() =>

  items.value.length ? t('lowCode.edit') : t('lowCode.createValues'),

)

const valuesByCode = computed(() => {

  const map: Record<string, unknown> = {}

  for (const item of items.value) {

    map[item.field_code] = item.value_json

  }

  return map

})



const editableTemplateFields = computed(() =>
  Object.values(fieldsByCode.value)
    .filter((field) => !field.system_field && !field.read_only)
    .sort((a, b) => a.sort_order - b.sort_order),
)



function fieldMeta(fieldCode: string): FormField | undefined {

  return fieldsByCode.value[fieldCode]

}



function isFieldEditable(fieldCode: string): boolean {

  const meta = fieldMeta(fieldCode)

  if (!meta) return true

  return !meta.system_field && !meta.read_only

}



function fieldTypeLabel(fieldCode: string): string {

  return fieldMeta(fieldCode)?.field_type ?? 'JSON'

}



function fieldLabel(fieldCode: string): string {

  return fieldMeta(fieldCode)?.label || fieldCode

}



function selectOptions(fieldCode: string) {

  const meta = fieldMeta(fieldCode)

  return parseSelectOptions(meta?.options_json)

}



function resetEditDraft() {

  const draft: Record<string, string> = {}

  const multi: Record<string, string[]> = {}

  for (const field of editableTemplateFields.value) {

    const existing = valuesByCode.value[field.code]

    seedEditDraftForField(draft, field.field_type, field.code, existing)

    if (field.field_type === 'MULTI_SELECT') {

      const parsed = existing

      const values = Array.isArray(parsed)

        ? parsed.map(String)

        : typeof parsed === 'string' && parsed.includes(',')

          ? parsed.split(',').map((part) => part.trim()).filter(Boolean)

          : parsed != null && parsed !== ''

            ? [String(parsed)]

            : []

      multi[field.code] = values

    }

  }

  editDraft.value = draft

  editMultiDraft.value = multi

}



const previewValues = computed(() => customFieldValuesToPreviewMap(items.value))



const previewTitle = computed(() =>

  items.value.length ? t('lowCode.currentValuesPreview') : t('lowCode.templateOnlyPreview'),

)



const effectivePreviewContext = computed(() =>

  buildPreviewContext(props.entityStatus, props.previewContext),

)



const effectiveValidationContext = computed(() =>

  compactValidationContextForPut(

    mergeLowCodeValidationContext(

      props.validationContext,

      props.previewContext,

      buildPreviewContext(props.entityStatus, props.validationContext ?? props.previewContext),

    ),

  ),

)



const fullEditorLink = computed(() => {

  if (!props.entityId?.trim()) return '/low-code/custom-field-values'

  return buildCustomFieldValuesEditorLink(

    props.entityType,

    props.entityId.trim(),

    props.entityStatus,

  )

})



async function loadTemplateMetadata() {

  if (!canLoad.value) {

    formTemplateId.value = null

    previewTemplate.value = null

    fieldsByCode.value = {}

    return

  }



  templateLoading.value = true

  try {

    const template = await resolvePublishedTemplate(props.entityType)

    if (!template) {

      formTemplateId.value = null

      previewTemplate.value = null

      fieldsByCode.value = {}

      return

    }

    formTemplateId.value = template.id

    previewTemplate.value = formTemplateDetailToPreview(template)

    const map: Record<string, FormField> = {}

    for (const field of flattenFormFields(template.sections ?? [])) {

      map[field.code] = field

    }

    fieldsByCode.value = map

  } catch {

    formTemplateId.value = null

    previewTemplate.value = null

    fieldsByCode.value = {}

  } finally {

    templateLoading.value = false

  }

}



async function load() {

  editing.value = false

  if (!canLoad.value) {

    loaded.value = false

    items.value = []

    return

  }



  loading.value = true

  loadFailed.value = false

  loaded.value = false

  try {

    await loadTemplateMetadata()

    const data = await getCustomFieldValues(props.entityType, props.entityId!.trim())

    items.value = data.items

    loaded.value = true

    resetEditDraft()

  } catch (error) {

    items.value = []

    if (error instanceof TenantRequiredError) return

    loadFailed.value = isApiUnavailableError(error)

    loaded.value = true

  } finally {

    loading.value = false

  }

}



function startEdit() {

  if (!canEdit.value) return

  resetEditDraft()

  editing.value = true

}



function cancelEdit() {

  editing.value = false

  resetEditDraft()

}



function parseFieldValue(fieldCode: string, fieldType: string) {

  if (fieldType === 'MULTI_SELECT') {

    return editMultiDraft.value[fieldCode] ?? []

  }

  const draft = editDraft.value[fieldCode] ?? ''

  return parseEditDraftToValueJson(fieldType, draft, fieldCode, editDraft.value)

}



async function saveEdit() {

  if (!formTemplateId.value || !props.entityId?.trim()) return



  saving.value = true

  try {

    const values = editableTemplateFields.value.map((field) => {

      try {

        return {

          field_code: field.code,

          value_json: parseFieldValue(field.code, field.field_type),

        }

      } catch {

        throw new Error('INVALID_JSON')

      }

    })



    await saveCustomFieldValues({

      entity_type: props.entityType,

      entity_id: props.entityId.trim(),

      form_template_id: formTemplateId.value,

      validation_context: effectiveValidationContext.value,

      values,

    })



    pushToast('success', t('lowCode.saved'))

    editing.value = false

    await load()

    emit('saved')

  } catch (error) {

    pushToast('error', getSaveErrorMessage(error))

  } finally {

    saving.value = false

  }

}



watch(

  () => [props.entityType, props.entityId, props.editable] as const,

  () => {

    load()

  },

  { immediate: true },

)

</script>



<template>

  <div class="low-code-panel-stack">

  <UiCard class="low-code-panel">

    <template #header>

      <div class="low-code-panel__header">

        <h3 class="low-code-panel__title">{{ panelTitle }}</h3>

        <div class="low-code-panel__actions">

          <NuxtLink

            v-if="showFullEditorLink && editable && !editing && entityId"

            :to="fullEditorLink"

            class="low-code-panel__template-link"

          >

            {{ $t('lowCode.openFullEditor') }}

          </NuxtLink>

          <NuxtLink

            v-if="formTemplateId && !editing"

            :to="`/low-code/form-templates/${formTemplateId}`"

            class="low-code-panel__template-link"

          >

            {{ $t('lowCode.viewFormTemplate') }}

          </NuxtLink>

          <UiBadge

            v-if="!editing && !editable"

            status="read-only"

            tone="neutral"

          >

            {{ $t('lowCode.readOnly') }}

          </UiBadge>

          <UiButton

            v-if="editable && loaded"

            size="sm"

            variant="secondary"

            :disabled="loading"

            @click="load"

          >

            {{ $t('lowCode.reloadValues') }}

          </UiButton>

          <UiButton

            v-if="editable && canEdit && !editing"

            size="sm"

            :disabled="editDisabled"

            @click="startEdit"

          >

            {{ editButtonLabel }}

          </UiButton>

        </div>

      </div>

    </template>



    <p class="low-code-panel__hint">

      {{ editable ? $t('lowCode.coreEntityNotChanged') : $t('lowCode.lowCodeFieldsHint') }}

    </p>



    <div v-if="!canLoad" class="low-code-panel__muted">{{ $t('lowCode.entityIdRequired') }}</div>



    <div v-else-if="loading || templateLoading" class="low-code-panel__muted">{{ $t('common.loading') }}</div>



    <CommonApiUnavailableState

      v-else-if="loadFailed"

      :title="$t('lowCode.customFieldsLoadFailed')"

      :message="$t('lowCode.serviceUnavailable')"

      @retry="load"

    />



    <UiEmptyState

      v-else-if="loaded && !items.length && !previewTemplate && !editing"

      :title="$t('lowCode.noCustomFieldsFound')"

      :description="editable ? $t('lowCode.emptySeedHint') : undefined"

    />



    <form v-else-if="editing" class="low-code-panel__edit-form" @submit.prevent="saveEdit">

      <p v-if="!items.length" class="low-code-panel__muted">

        {{ $t('lowCode.createValuesHint') }}

      </p>

      <div

        v-for="field in editableTemplateFields"

        :key="field.code"

        class="low-code-panel__edit-row"

      >

        <div class="low-code-panel__edit-label">

          <code>{{ field.code }}</code>

          <span class="text-muted">{{ field.field_type }}</span>

        </div>



        <UiSelect

          v-if="field.field_type === 'SELECT'"

          v-model="editDraft[field.code]"

          :label="fieldLabel(field.code)"

          :options="selectOptions(field.code)"

        />



        <label v-else-if="field.field_type === 'CHECKBOX'" class="low-code-panel__checkbox">

          <input

            type="checkbox"

            :checked="editDraft[field.code] === 'true'"

            @change="editDraft[field.code] = ($event.target as HTMLInputElement).checked ? 'true' : 'false'"

          />

          <span>{{ fieldLabel(field.code) }}</span>

        </label>



        <UiInput

          v-else-if="field.field_type === 'NUMBER'"

          v-model="editDraft[field.code]"

          type="number"

          :label="fieldLabel(field.code)"

        />



        <UiInput

          v-else-if="field.field_type === 'TEXT' || field.field_type === 'CURRENCY'"

          v-model="editDraft[field.code]"

          :label="fieldLabel(field.code)"

        />



        <UiInput

          v-else-if="field.field_type === 'DATE'"

          v-model="editDraft[field.code]"

          type="date"

          :label="fieldLabel(field.code)"

        />



        <UiInput

          v-else-if="field.field_type === 'DATETIME'"

          v-model="editDraft[field.code]"

          type="datetime-local"

          :label="fieldLabel(field.code)"

        />



        <label v-else-if="field.field_type === 'MULTI_SELECT'" class="low-code-panel__multi-select">

          <span class="ui-input__label">{{ fieldLabel(field.code) }}</span>

          <select

            v-model="editMultiDraft[field.code]"

            class="low-code-panel__select"

            multiple

          >

            <option

              v-for="option in selectOptions(field.code)"

              :key="option.value"

              :value="option.value"

            >

              {{ option.label }}

            </option>

          </select>

        </label>



        <div v-else-if="field.field_type === 'MONEY'" class="low-code-panel__money">

          <UiInput

            v-model="editDraft[moneyDraftKeys(field.code).amount]"

            type="number"

            :label="`${fieldLabel(field.code)} — ${$t('lowCode.moneyAmount')}`"

          />

          <UiInput

            v-model="editDraft[moneyDraftKeys(field.code).currency]"

            :label="`${fieldLabel(field.code)} — ${$t('lowCode.moneyCurrency')}`"

          />

        </div>



        <label v-else-if="usesJsonTextareaFallback(field.field_type)" class="low-code-panel__json-editor">

          <span class="ui-input__label">{{ field.code }} (JSON)</span>

          <textarea

            v-model="editDraft[field.code]"

            class="low-code-panel__textarea"

            rows="4"

          />

        </label>

      </div>



      <div class="low-code-panel__edit-actions">

        <UiButton type="button" variant="secondary" :disabled="saving" @click="cancelEdit">

          {{ $t('common.cancel') }}

        </UiButton>

        <UiButton type="submit" :loading="saving">

          {{ $t('common.save') }}

        </UiButton>

      </div>

    </form>



    <p

      v-else-if="loaded && !items.length && previewTemplate"

      class="low-code-panel__muted"

    >

      {{ $t('lowCode.noCustomFieldValuesYet') }}

    </p>



    <template v-else-if="items.length">

      <UiTable :columns="[$t('lowCode.field'), $t('lowCode.label'), $t('lowCode.value'), $t('lowCode.updatedAt')]">

        <tr v-for="item in items" :key="item.field_id">

          <td>

            <code>{{ item.field_code }}</code>

            <span v-if="fieldMeta(item.field_code)?.system_field" class="low-code-panel__tag">

              {{ $t('lowCode.systemField') }}

            </span>

            <span v-else-if="fieldMeta(item.field_code)?.read_only" class="low-code-panel__tag">

              {{ $t('lowCode.readOnlyField') }}

            </span>

          </td>

          <td>{{ fieldLabel(item.field_code) }}</td>

          <td>

            <span

              v-if="!isCustomFieldComplexValue(item.value_json)"

              class="low-code-panel__value"

            >

              {{ formatCustomFieldDisplayValue(item.value_json) }}

            </span>

            <pre v-else class="low-code-panel__value-json">{{ formatCustomFieldDisplayValue(item.value_json) }}</pre>

          </td>

          <td>{{ formatLowCodeDate(item.updated_at) }}</td>

        </tr>

      </UiTable>

    </template>

  </UiCard>



  <LowCodeFormTemplatePreview

    v-if="showPreview && loaded && previewTemplate"

    :template="previewTemplate"

    :values="previewValues"

    :title="previewTitle"

    :preview-context="effectivePreviewContext"

  />

  </div>

</template>



<style scoped>

.low-code-panel-stack {

  display: flex;

  flex-direction: column;

  gap: 1rem;

}



.low-code-panel__header {

  display: flex;

  align-items: center;

  justify-content: space-between;

  gap: 0.75rem;

  flex-wrap: wrap;

}



.low-code-panel__actions {

  display: flex;

  align-items: center;

  gap: 0.75rem;

  flex-wrap: wrap;

}



.low-code-panel__template-link {

  font-size: 0.875rem;

  color: var(--color-primary);

  text-decoration: none;

}



.low-code-panel__template-link:hover {

  text-decoration: underline;

}



.low-code-panel__title {

  margin: 0;

  font-size: 1rem;

  font-weight: 600;

}



.low-code-panel__hint {

  margin: 0 0 1rem;

  font-size: 0.8125rem;

  color: var(--color-text-muted);

}



.low-code-panel__muted {

  font-size: 0.875rem;

  color: var(--color-text-muted);

}



.low-code-panel__value {

  font-size: 0.875rem;

}



.low-code-panel__value-json {

  margin: 0;

  font-size: 0.75rem;

  white-space: pre-wrap;

  word-break: break-word;

  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;

}



.low-code-panel__tag {

  display: inline-block;

  margin-left: 0.5rem;

  font-size: 0.6875rem;

  color: var(--color-text-muted);

  text-transform: uppercase;

}



.low-code-panel__edit-form {

  display: flex;

  flex-direction: column;

  gap: 1rem;

}



.low-code-panel__edit-row {

  display: grid;

  gap: 0.5rem;

}



.low-code-panel__edit-label {

  display: flex;

  align-items: center;

  gap: 0.5rem;

  font-size: 0.8125rem;

}



.low-code-panel__checkbox {

  display: flex;

  align-items: center;

  gap: 0.5rem;

  font-size: 0.875rem;

}



.low-code-panel__json-editor {

  display: flex;

  flex-direction: column;

  gap: 0.375rem;

}



.low-code-panel__textarea {

  min-height: 96px;

  padding: 0.5rem 0.75rem;

  border: 1px solid var(--color-border);

  border-radius: var(--radius-md);

  background: var(--color-surface);

  font: inherit;

  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;

  font-size: 0.8125rem;

}



.low-code-panel__select {

  min-height: 96px;

  padding: 0.5rem 0.75rem;

  border: 1px solid var(--color-border);

  border-radius: var(--radius-md);

  background: var(--color-surface);

  font: inherit;

  font-size: 0.875rem;

}



.low-code-panel__multi-select {

  display: flex;

  flex-direction: column;

  gap: 0.375rem;

}



.low-code-panel__money {

  display: grid;

  gap: 0.75rem;

}



@media (min-width: 640px) {

  .low-code-panel__money {

    grid-template-columns: 1fr 1fr;

  }

}



.low-code-panel__edit-actions {

  display: flex;

  gap: 0.5rem;

  justify-content: flex-end;

}

</style>

