<script setup lang="ts">
import {
  MAX_TEMPLATE_IMPORT_PAYLOAD_BYTES,
  TEMPLATE_EXPORT_SCHEMA_VERSION,
  type ImportConflictStrategy,
  type ImportExecuteResponse,
  type ImportPreviewRequest,
  type ImportPreviewResponse,
} from '~/types/lowCode'
import {
  parseTemplateImportJsonText,
  TemplateImportJsonError,
} from '~/utils/lowCodeTemplateImportExport'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  executed: [result: ImportExecuteResponse]
}>()

const {
  previewImportFormTemplate,
  importFormTemplate,
  getAdminFormTemplateErrorMessage,
} = useLowCodeApi()
const { canImportTemplates } = useLowCodePermissions()
const { pushToast } = useToast()
const { t } = useI18n()

const step = ref(1)
const jsonText = ref('')
const conflictStrategy = ref<ImportConflictStrategy>('NEW_VERSION')
const targetCode = ref('')
const parseError = ref('')
const previewError = ref('')
const executeError = ref('')
const previewLoading = ref(false)
const executeLoading = ref(false)
const warningsConfirmed = ref(false)
const previewResult = ref<ImportPreviewResponse | null>(null)
const lastPreviewRequest = ref<ImportPreviewRequest | null>(null)
const executeResult = ref<ImportExecuteResponse | null>(null)
const resultEntityType = ref('')

const conflictStrategyOptions = computed(() => [
  { label: t('lowCode.templateImportConflictNewVersion'), value: 'NEW_VERSION' },
  { label: t('lowCode.templateImportConflictReplaceDraft'), value: 'REPLACE_EXISTING_DRAFT' },
  { label: t('lowCode.templateImportConflictFailIfExists'), value: 'FAIL_IF_EXISTS' },
])

const stepMeta = computed(() => {
  const map = {
    1: {
      title: t('lowCode.templateImportStepPasteTitle'),
      description: t('lowCode.templateImportStepPasteDesc'),
    },
    2: {
      title: t('lowCode.templateImportPreview'),
      description: t('lowCode.templateImportStepPreviewDesc'),
    },
    3: {
      title: t('lowCode.templateImportConfirm'),
      description: t('lowCode.templateImportStepConfirmDesc'),
    },
    4: {
      title: t('lowCode.templateImportResult'),
      description: t('lowCode.templateImportStepResultDesc'),
    },
  } as const
  return map[step.value as 1 | 2 | 3 | 4]
})

const stepIndicatorLabel = computed(() =>
  t('lowCode.templateImportStepIndicator', { current: step.value, total: 4 }),
)

const parsedTemplateMeta = computed(() => {
  if (!lastPreviewRequest.value) return null
  const template = lastPreviewRequest.value.template
  return {
    code: template.code,
    entityType: template.entity_type,
    name: template.name,
    version: template.version,
  }
})

const hasWarnings = computed(() => (previewResult.value?.warnings?.length ?? 0) > 0)
const hasBlockingErrors = computed(() => {
  if (!previewResult.value) return true
  if (previewResult.value.status === 'BLOCKED') return true
  return (previewResult.value.validation_errors?.length ?? 0) > 0
})

const predictedResultKey = computed(() => {
  const strategy = previewResult.value?.conflict_strategy ?? conflictStrategy.value
  if (strategy === 'REPLACE_EXISTING_DRAFT') return 'lowCode.templateImportPredictReplaceDraft'
  if (strategy === 'FAIL_IF_EXISTS') return 'lowCode.templateImportPredictFailIfExists'
  return 'lowCode.templateImportPredictCreateDraft'
})

const executeDisabled = computed(() => {
  if (!canImportTemplates()) return true
  if (!previewResult.value || !lastPreviewRequest.value) return true
  if (hasBlockingErrors.value) return true
  if (hasWarnings.value && !warningsConfirmed.value) return true
  return false
})

function resetWizard() {
  step.value = 1
  jsonText.value = ''
  conflictStrategy.value = 'NEW_VERSION'
  targetCode.value = ''
  parseError.value = ''
  previewError.value = ''
  executeError.value = ''
  previewLoading.value = false
  executeLoading.value = false
  warningsConfirmed.value = false
  previewResult.value = null
  lastPreviewRequest.value = null
  executeResult.value = null
  resultEntityType.value = ''
}

function handleClose() {
  resetWizard()
  emit('close')
}

function buildPreviewRequest(): ImportPreviewRequest {
  return parseTemplateImportJsonText(jsonText.value, {
    conflictStrategy: conflictStrategy.value,
    targetCode: targetCode.value.trim() || undefined,
    mode: 'CREATE_DRAFT',
  })
}

function mapParseError(error: unknown): string {
  if (error instanceof TemplateImportJsonError) {
    switch (error.code) {
      case 'INVALID_JSON':
        return t('lowCode.templateImportInvalidJson')
      case 'UNSUPPORTED_SCHEMA':
        return t('lowCode.templateImportSchemaVersionMismatch', {
          expected: TEMPLATE_EXPORT_SCHEMA_VERSION,
          actual: error.message === 'MISSING_SCHEMA' ? '—' : error.message,
        })
      case 'MISSING_TEMPLATE':
        return t('lowCode.templateImportMissingTemplate')
      case 'PAYLOAD_TOO_LARGE':
        return t('lowCode.templateImportPayloadTooLarge')
      default:
        return t('lowCode.templateImportInvalidJson')
    }
  }
  return error instanceof Error ? error.message : t('common.error')
}

function validateStep1(): boolean {
  parseError.value = ''
  try {
    buildPreviewRequest()
    return true
  } catch (error) {
    parseError.value = mapParseError(error)
    return false
  }
}

async function runPreview() {
  previewError.value = ''
  previewResult.value = null
  warningsConfirmed.value = false

  let request: ImportPreviewRequest
  try {
    request = buildPreviewRequest()
  } catch (error) {
    parseError.value = mapParseError(error)
    return
  }

  lastPreviewRequest.value = request
  previewLoading.value = true
  try {
    previewResult.value = await previewImportFormTemplate(request)
  } catch (error) {
    previewError.value = getAdminFormTemplateErrorMessage(error)
    previewResult.value = null
  } finally {
    previewLoading.value = false
  }
}

async function goToPreviewStep() {
  if (!validateStep1()) return
  step.value = 2
  await runPreview()
}

function goBack() {
  if (step.value === 2) {
    previewResult.value = null
    previewError.value = ''
    lastPreviewRequest.value = null
    warningsConfirmed.value = false
  }
  if (step.value === 3) {
    executeError.value = ''
    warningsConfirmed.value = false
  }
  if (step.value > 1) {
    step.value -= 1
  }
}

function goToConfirmStep() {
  if (!previewResult.value || previewError.value || hasBlockingErrors.value) return
  step.value = 3
}

async function executeImport() {
  if (executeDisabled.value || !lastPreviewRequest.value) return

  executeError.value = ''
  executeLoading.value = true
  try {
    const result = await importFormTemplate(lastPreviewRequest.value)
    executeResult.value = result
    resultEntityType.value = previewResult.value?.target_entity_type ?? parsedTemplateMeta.value?.entityType ?? ''
    step.value = 4
    emit('executed', result)
    pushToast('success', t('lowCode.templateImportExecuteSuccess'))
  } catch (error) {
    executeError.value = getAdminFormTemplateErrorMessage(error)
  } finally {
    executeLoading.value = false
  }
}

function importAnother() {
  resetWizard()
}

async function onFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return

  if (file.size > MAX_TEMPLATE_IMPORT_PAYLOAD_BYTES) {
    parseError.value = t('lowCode.templateImportPayloadTooLarge')
    return
  }

  try {
    jsonText.value = await file.text()
    parseError.value = ''
  } catch {
    parseError.value = t('lowCode.templateImportInvalidJson')
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) resetWizard()
  },
)

watch([jsonText, conflictStrategy, targetCode], () => {
  if (step.value >= 2) {
    previewResult.value = null
    lastPreviewRequest.value = null
    previewError.value = ''
    warningsConfirmed.value = false
  }
})
</script>

<template>
  <div class="import-wizard-root">
    <UiModal
      :open="open"
      :title="$t('lowCode.templateImportTitle')"
      @close="handleClose"
    >
      <div class="import-wizard">
        <header class="import-wizard__step-header">
          <p class="import-wizard__step-indicator">{{ stepIndicatorLabel }}</p>
          <h3 class="import-wizard__step-title">{{ stepMeta.title }}</h3>
          <p class="import-wizard__step-desc">{{ stepMeta.description }}</p>
        </header>

        <ol class="import-wizard__steps" aria-label="Wizard steps">
          <li :class="{ 'import-wizard__step--active': step === 1, 'import-wizard__step--done': step > 1 }">
            {{ $t('lowCode.templateImportStepPaste') }}
          </li>
          <li :class="{ 'import-wizard__step--active': step === 2, 'import-wizard__step--done': step > 2 }">
            {{ $t('lowCode.templateImportPreview') }}
          </li>
          <li :class="{ 'import-wizard__step--active': step === 3, 'import-wizard__step--done': step > 3 }">
            {{ $t('lowCode.templateImportConfirm') }}
          </li>
          <li :class="{ 'import-wizard__step--active': step === 4 }">
            {{ $t('lowCode.templateImportResult') }}
          </li>
        </ol>

        <div v-if="step === 1" class="import-wizard__panel">
          <label class="import-wizard__label">
            {{ $t('lowCode.templateImportPasteJson') }}
            <textarea
              v-model="jsonText"
              class="import-wizard__textarea"
              rows="10"
              spellcheck="false"
              :placeholder="$t('lowCode.templateImportPastePlaceholder')"
            />
          </label>

          <div class="import-wizard__inline-actions">
            <label class="import-wizard__file-label">
              <span class="import-wizard__file-button">{{ $t('lowCode.templateImportUploadJson') }}</span>
              <input
                type="file"
                accept="application/json,.json"
                class="import-wizard__file-input"
                @change="onFileSelected"
              >
            </label>
          </div>

          <UiSelect
            v-model="conflictStrategy"
            :label="$t('lowCode.templateImportConflictStrategy')"
            :options="conflictStrategyOptions"
          />

          <label class="import-wizard__label">
            {{ $t('lowCode.templateImportTargetCode') }}
            <span class="import-wizard__hint">{{ $t('lowCode.templateImportTargetCodeHint') }}</span>
            <input
              v-model="targetCode"
              type="text"
              class="import-wizard__input"
              spellcheck="false"
            >
          </label>

          <p class="import-wizard__schema-hint">
            {{ $t('lowCode.templateImportSchemaVersion') }}: <code>{{ TEMPLATE_EXPORT_SCHEMA_VERSION }}</code>
          </p>

          <p v-if="parseError" class="import-wizard__error" role="alert">{{ parseError }}</p>
        </div>

        <div v-else-if="step === 2" class="import-wizard__panel">
          <div v-if="previewLoading" class="text-muted">{{ $t('common.loading') }}</div>

          <p v-else-if="previewError" class="import-wizard__error" role="alert">{{ previewError }}</p>

          <template v-else-if="previewResult">
            <dl class="import-wizard__summary">
              <div>
                <dt>{{ $t('lowCode.code') }}</dt>
                <dd><code>{{ previewResult.target_code }}</code></dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.entityType') }}</dt>
                <dd>{{ previewResult.target_entity_type }}</dd>
              </div>
              <div v-if="parsedTemplateMeta">
                <dt>{{ $t('common.name') }}</dt>
                <dd>{{ parsedTemplateMeta.name }}</dd>
              </div>
              <div v-if="parsedTemplateMeta">
                <dt>{{ $t('lowCode.version') }}</dt>
                <dd>{{ parsedTemplateMeta.version }}</dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.templateImportSectionsCount') }}</dt>
                <dd>{{ previewResult.summary.sections_count }}</dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.templateImportFieldsCount') }}</dt>
                <dd>{{ previewResult.summary.fields_count }}</dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.templateImportConflictStrategy') }}</dt>
                <dd>{{ previewResult.conflict_strategy }}</dd>
              </div>
              <div class="import-wizard__summary-wide">
                <dt>{{ $t('lowCode.templateImportPredictedResult') }}</dt>
                <dd>{{ $t(predictedResultKey) }}</dd>
              </div>
            </dl>

            <div v-if="previewResult.validation_errors.length" class="import-wizard__issues">
              <strong>{{ $t('lowCode.templateImportBlockingErrors') }}</strong>
              <ul>
                <li v-for="(item, index) in previewResult.validation_errors" :key="`err-${index}`">
                  {{ item }}
                </li>
              </ul>
            </div>

            <div v-if="previewResult.warnings.length" class="import-wizard__issues import-wizard__issues--warn">
              <strong>{{ $t('lowCode.templateImportWarnings') }}</strong>
              <ul>
                <li v-for="(item, index) in previewResult.warnings" :key="`warn-${index}`">
                  {{ item }}
                </li>
              </ul>
            </div>

            <details class="import-wizard__raw">
              <summary>{{ $t('lowCode.templateImportRawPreview') }}</summary>
              <pre class="import-wizard__pre">{{ JSON.stringify(previewResult, null, 2) }}</pre>
            </details>
          </template>
        </div>

        <div v-else-if="step === 3" class="import-wizard__panel">
          <div class="import-wizard__notice">
            <strong>{{ $t('lowCode.templateImportDraftOnly') }}</strong>
            <p>{{ $t('lowCode.templateImportNoAutoPublish') }}</p>
          </div>

          <dl v-if="previewResult" class="import-wizard__summary">
            <div>
              <dt>{{ $t('lowCode.code') }}</dt>
              <dd><code>{{ previewResult.target_code }}</code></dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.entityType') }}</dt>
              <dd>{{ previewResult.target_entity_type }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.templateImportConflictStrategy') }}</dt>
              <dd>{{ previewResult.conflict_strategy }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.templateImportPredictedResult') }}</dt>
              <dd>{{ $t(predictedResultKey) }}</dd>
            </div>
          </dl>

          <label v-if="hasWarnings" class="import-wizard__checkbox">
            <input v-model="warningsConfirmed" type="checkbox">
            {{ $t('lowCode.templateImportWarningsConfirm') }}
          </label>

          <p v-if="executeError" class="import-wizard__error" role="alert">{{ executeError }}</p>
        </div>

        <div v-else class="import-wizard__panel">
          <template v-if="executeResult">
            <dl class="import-wizard__summary">
              <div>
                <dt>{{ $t('lowCode.templateImportDraftId') }}</dt>
                <dd><code>{{ executeResult.id }}</code></dd>
              </div>
              <div v-if="resultEntityType">
                <dt>{{ $t('lowCode.entityType') }}</dt>
                <dd>{{ resultEntityType }}</dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.code') }}</dt>
                <dd><code>{{ executeResult.code }}</code></dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.version') }}</dt>
                <dd>{{ executeResult.version }}</dd>
              </div>
              <div>
                <dt>{{ $t('common.status') }}</dt>
                <dd><UiBadge :status="executeResult.status" /></dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.templateImportSectionsCount') }}</dt>
                <dd>{{ executeResult.import_summary.sections_count }}</dd>
              </div>
              <div>
                <dt>{{ $t('lowCode.templateImportFieldsCount') }}</dt>
                <dd>{{ executeResult.import_summary.fields_count }}</dd>
              </div>
            </dl>

            <p class="import-wizard__audit-hint">{{ $t('lowCode.templateImportAuditHint') }}</p>
          </template>
        </div>

        <footer class="import-wizard__footer">
          <template v-if="step === 1">
            <UiButton variant="secondary" @click="handleClose">{{ $t('common.cancel') }}</UiButton>
            <UiButton :disabled="!canImportTemplates()" @click="goToPreviewStep">
              {{ $t('lowCode.templateImportPreview') }}
            </UiButton>
          </template>

          <template v-else-if="step === 2">
            <UiButton variant="secondary" @click="goBack">{{ $t('common.back') }}</UiButton>
            <UiButton
              variant="secondary"
              :loading="previewLoading"
              @click="runPreview"
            >
              {{ $t('common.refresh') }}
            </UiButton>
            <UiButton
              :disabled="!previewResult || !!previewError || hasBlockingErrors"
              @click="goToConfirmStep"
            >
              {{ $t('common.next') }}
            </UiButton>
          </template>

          <template v-else-if="step === 3">
            <UiButton variant="secondary" @click="goBack">{{ $t('common.back') }}</UiButton>
            <UiButton
              :loading="executeLoading"
              :disabled="executeDisabled"
              @click="executeImport"
            >
              {{ $t('lowCode.templateImportExecute') }}
            </UiButton>
          </template>

          <template v-else>
            <UiButton variant="secondary" @click="handleClose">{{ $t('common.close') }}</UiButton>
            <NuxtLink
              v-if="executeResult"
              :to="`/low-code/admin/form-templates/${executeResult.id}`"
            >
              <UiButton>{{ $t('lowCode.templateImportOpenDraft') }}</UiButton>
            </NuxtLink>
            <UiButton @click="importAnother">{{ $t('lowCode.templateImportAnother') }}</UiButton>
          </template>
        </footer>
      </div>
    </UiModal>
  </div>
</template>

<style scoped>
.import-wizard {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.import-wizard__step-header {
  margin: 0;
}

.import-wizard__step-indicator {
  margin: 0 0 0.25rem;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.import-wizard__step-title {
  margin: 0 0 0.25rem;
  font-size: 1.125rem;
}

.import-wizard__step-desc {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.import-wizard__steps {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.import-wizard__step--active {
  color: var(--color-text);
  font-weight: 600;
}

.import-wizard__step--done {
  color: var(--color-success, #15803d);
}

.import-wizard__panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.import-wizard__label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  font-size: 0.875rem;
}

.import-wizard__hint {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
  font-weight: normal;
}

.import-wizard__textarea,
.import-wizard__input {
  width: 100%;
  font-family: ui-monospace, monospace;
  font-size: 0.8125rem;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  background: var(--color-bg);
}

.import-wizard__inline-actions {
  display: flex;
  gap: 0.5rem;
}

.import-wizard__file-label {
  cursor: pointer;
}

.import-wizard__file-input {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}

.import-wizard__file-button {
  display: inline-block;
  padding: 0.35rem 0.75rem;
  font-size: 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: 0.375rem;
  background: var(--color-bg-muted, #f4f4f5);
}

.import-wizard__schema-hint {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.import-wizard__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem 1rem;
  margin: 0;
}

.import-wizard__summary div {
  margin: 0;
}

.import-wizard__summary dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  margin-bottom: 0.15rem;
}

.import-wizard__summary dd {
  margin: 0;
  font-size: 0.875rem;
}

.import-wizard__summary-wide {
  grid-column: 1 / -1;
}

.import-wizard__issues {
  padding: 0.65rem 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--color-danger, #dc2626);
  background: color-mix(in srgb, var(--color-danger, #dc2626) 8%, transparent);
  font-size: 0.875rem;
}

.import-wizard__issues ul {
  margin: 0.35rem 0 0;
  padding-left: 1.25rem;
}

.import-wizard__issues--warn {
  border-color: var(--color-warning, #ca8a04);
  background: color-mix(in srgb, var(--color-warning, #ca8a04) 10%, transparent);
}

.import-wizard__notice {
  padding: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--color-warning, #ca8a04);
  background: color-mix(in srgb, var(--color-warning, #ca8a04) 10%, transparent);
}

.import-wizard__notice p {
  margin: 0.35rem 0 0;
  font-size: 0.875rem;
}

.import-wizard__checkbox {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.875rem;
}

.import-wizard__raw summary {
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.import-wizard__pre {
  margin: 0.5rem 0 0;
  padding: 0.75rem;
  max-height: 240px;
  overflow: auto;
  font-size: 0.75rem;
  border-radius: 0.375rem;
  background: var(--color-bg-muted, #f4f4f5);
  white-space: pre-wrap;
  word-break: break-word;
}

.import-wizard__error {
  margin: 0;
  color: var(--color-danger, #dc2626);
  font-size: 0.875rem;
}

.import-wizard__audit-hint {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.import-wizard__footer {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
  padding-top: 0.5rem;
  border-top: 1px solid var(--color-border);
}

.text-muted {
  color: var(--color-text-muted);
}
</style>
