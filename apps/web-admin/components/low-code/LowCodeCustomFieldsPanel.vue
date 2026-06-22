<script setup lang="ts">
import {
  flattenFormFields,
  formatCustomFieldDisplayValue,
  formatJsonValue,
  formatLowCodeDate,
  isCustomFieldComplexValue,
  parseEditDraftToValueJson,
  parseSelectOptions,
  valueToEditDraft,
  type FormField,
  type LowCodeEntityType,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

const props = withDefaults(
  defineProps<{
    entityType: LowCodeEntityType | string
    entityId?: string | null
    title?: string
    editable?: boolean
  }>(),
  {
    entityId: null,
    title: undefined,
    editable: false,
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
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(false)
const loaded = ref(false)
const loadFailed = ref(false)
const templateLoading = ref(false)
const saving = ref(false)
const editing = ref(false)
const formTemplateId = ref<string | null>(null)
const fieldsByCode = ref<Record<string, FormField>>({})
const items = ref<Array<{ field_id: string; field_code: string; value_json: unknown; updated_at: string }>>([])
const editDraft = ref<Record<string, string>>({})

const panelTitle = computed(() => props.title || t('lowCode.customFields'))
const canLoad = computed(() => hasTenant.value && !!props.entityId?.trim())
const canEdit = computed(
  () => props.editable && !loadFailed.value && !loading.value && !!formTemplateId.value && items.value.length > 0,
)
const editDisabled = computed(() => loadFailed.value || loading.value || templateLoading.value)

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

function selectOptions(fieldCode: string) {
  const meta = fieldMeta(fieldCode)
  return parseSelectOptions(meta?.options_json)
}

function resetEditDraft() {
  const draft: Record<string, string> = {}
  for (const item of items.value) {
    if (!isFieldEditable(item.field_code)) continue
    draft[item.field_code] = valueToEditDraft(fieldTypeLabel(item.field_code), item.value_json)
  }
  editDraft.value = draft
}

async function loadTemplateMetadata() {
  if (!props.editable || !canLoad.value) {
    formTemplateId.value = null
    fieldsByCode.value = {}
    return
  }

  templateLoading.value = true
  try {
    const template = await resolvePublishedTemplate(props.entityType)
    if (!template) {
      formTemplateId.value = null
      fieldsByCode.value = {}
      return
    }
    formTemplateId.value = template.id
    const map: Record<string, FormField> = {}
    for (const field of flattenFormFields(template.sections ?? [])) {
      map[field.code] = field
    }
    fieldsByCode.value = map
  } catch {
    formTemplateId.value = null
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

async function saveEdit() {
  if (!formTemplateId.value || !props.entityId?.trim()) return

  saving.value = true
  try {
    const values = items.value
      .filter((item) => isFieldEditable(item.field_code))
      .map((item) => {
        const draft = editDraft.value[item.field_code] ?? ''
        try {
          return {
            field_code: item.field_code,
            value_json: parseEditDraftToValueJson(fieldTypeLabel(item.field_code), draft),
          }
        } catch {
          throw new Error('INVALID_JSON')
        }
      })

    await saveCustomFieldValues({
      entity_type: props.entityType,
      entity_id: props.entityId.trim(),
      form_template_id: formTemplateId.value,
      values,
    })

    pushToast('success', t('lowCode.saved'))
    editing.value = false
    await load()
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
  <UiCard class="low-code-panel">
    <template #header>
      <div class="low-code-panel__header">
        <h3 class="low-code-panel__title">{{ panelTitle }}</h3>
        <div class="low-code-panel__actions">
          <UiBadge
            v-if="!editing"
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
            {{ $t('lowCode.edit') }}
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
      v-else-if="loaded && !items.length"
      :title="$t('lowCode.noCustomFieldsFound')"
      :description="editable ? $t('lowCode.emptySeedHint') : undefined"
    />

    <template v-else-if="items.length && !editing">
      <UiTable :columns="[$t('lowCode.field'), $t('lowCode.value'), $t('lowCode.updatedAt')]">
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

    <form v-else-if="editing" class="low-code-panel__edit-form" @submit.prevent="saveEdit">
      <div
        v-for="item in items"
        :key="item.field_id"
        class="low-code-panel__edit-row"
      >
        <div class="low-code-panel__edit-label">
          <code>{{ item.field_code }}</code>
          <span class="text-muted">{{ fieldTypeLabel(item.field_code) }}</span>
        </div>

        <div v-if="!isFieldEditable(item.field_code)" class="low-code-panel__readonly-value">
          <span class="low-code-panel__tag">{{ $t('lowCode.readOnlyField') }}</span>
          <pre>{{ formatJsonValue(item.value_json) }}</pre>
        </div>

        <UiSelect
          v-else-if="fieldTypeLabel(item.field_code) === 'SELECT'"
          v-model="editDraft[item.field_code]"
          :label="item.field_code"
          :options="selectOptions(item.field_code)"
        />

        <label v-else-if="fieldTypeLabel(item.field_code) === 'CHECKBOX'" class="low-code-panel__checkbox">
          <input
            type="checkbox"
            :checked="editDraft[item.field_code] === 'true'"
            @change="editDraft[item.field_code] = ($event.target as HTMLInputElement).checked ? 'true' : 'false'"
          />
          <span>{{ item.field_code }}</span>
        </label>

        <UiInput
          v-else-if="fieldTypeLabel(item.field_code) === 'NUMBER'"
          v-model="editDraft[item.field_code]"
          type="number"
          :label="item.field_code"
        />

        <UiInput
          v-else-if="fieldTypeLabel(item.field_code) === 'TEXT' || fieldTypeLabel(item.field_code) === 'CURRENCY'"
          v-model="editDraft[item.field_code]"
          :label="item.field_code"
        />

        <label v-else class="low-code-panel__json-editor">
          <span class="ui-input__label">{{ item.field_code }} (JSON)</span>
          <textarea
            v-model="editDraft[item.field_code]"
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
  </UiCard>
</template>

<style scoped>
.low-code-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.low-code-panel__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.low-code-panel__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
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

.low-code-panel__readonly-value pre {
  margin: 0.25rem 0 0;
  font-size: 0.75rem;
  white-space: pre-wrap;
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

.low-code-panel__edit-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
}
</style>
