<script setup lang="ts">
import type { CarrierXlsxClassifiedError } from '~/utils/carrierXlsxErrors'
import type { CarrierXlsxCommitResponse, CarrierXlsxPreviewResponse } from '~/types/carrierXlsx'
import {
  canCommitCarrierXlsxPreview,
  classifyCarrierXlsxHttpError,
  shouldInvalidateCarrierXlsxAnalysis,
} from '~/utils/carrierXlsxErrors'
import { createCarrierXlsxIdempotencyStore } from '~/utils/carrierXlsxIdempotency'
import { resolveCarrierXlsxIssueCopy } from '~/utils/carrierXlsxIssueText'
import { CARRIER_XLSX_MAX_UPLOAD_BYTES } from '~/utils/carrierXlsxApiRoutes'
import { canImportCarrierXlsx, canShowCarrierXlsxPanel } from '~/utils/carrierXlsxAccess'
import Button from '~/components/ui/Button.vue'
import Card from '~/components/ui/Card.vue'

const props = defineProps<{
  eventId: string
  responseId: string
  responseStatus: string
}>()

const emit = defineEmits<{
  committed: [result: CarrierXlsxCommitResponse]
}>()

const { t } = useI18n()
const { pushToast } = useToast()
const { enabled: excelEnabled } = useRfxExcelExchangeFeature()
const { exportCarrierDraft, previewCarrierDraft, commitCarrierDraft } = useCarrierXlsxApi()
const idempotency = createCarrierXlsxIdempotencyStore()

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const exporting = ref(false)
const previewing = ref(false)
const committing = ref(false)
const preview = ref<CarrierXlsxPreviewResponse | null>(null)
const commitResult = ref<CarrierXlsxCommitResponse | null>(null)
const lastError = ref<CarrierXlsxClassifiedError | null>(null)
const analysisInvalidated = ref(false)
const importDisabled = ref(false)
const statusMessage = ref('')

const visible = computed(() =>
  canShowCarrierXlsxPanel({
    excelExchangeEnabled: excelEnabled.value,
    roles: useAuthStore().user?.roles ?? [],
    responseId: props.responseId,
  }),
)
const importEnabled = computed(() =>
  canImportCarrierXlsx({
    responseStatus: props.responseStatus,
    responseNotEditable: importDisabled.value,
  }),
)
const busy = computed(() => exporting.value || previewing.value || committing.value)
const commitEnabled = computed(() =>
  importEnabled.value
  && canCommitCarrierXlsxPreview(preview.value, {
    analysisInvalidated: analysisInvalidated.value,
    alreadyCommitted: Boolean(commitResult.value),
    importDisabled: importDisabled.value,
  })
  && !busy.value,
)
const showRetryPreview = computed(() => Boolean(lastError.value?.retryPreview) && importEnabled.value)

function setStatus(message: string) {
  statusMessage.value = message
}

function clearImportState() {
  preview.value = null
  commitResult.value = null
  lastError.value = null
  analysisInvalidated.value = false
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  selectedFile.value = file
  clearImportState()
  if (!file) return
  void runPreview(file)
}

async function runPreview(file: File) {
  if (!importEnabled.value) return
  previewing.value = true
  lastError.value = null
  commitResult.value = null
  setStatus(t('carrierTenders.xlsx.status.previewing'))
  try {
    const result = await previewCarrierDraft(props.eventId, props.responseId, file)
    preview.value = result.preview
    idempotency.rememberPreview(result.preview.analysis_id)
    if (result.status === 422 || !result.preview.ready_to_commit) {
      analysisInvalidated.value = true
      lastError.value = {
        kind: 'preview_invalid',
        status: 422,
        machineCode: result.preview.errors[0]?.machine_code ?? null,
        retryPreview: false,
        disableImport: result.preview.errors.some((issue) => issue.machine_code === 'response_not_editable'),
        messageKey: 'carrierTenders.xlsx.errors.previewInvalid',
      }
      if (lastError.value.disableImport) {
        importDisabled.value = true
      }
      setStatus(t('carrierTenders.xlsx.status.previewBlocked'))
    } else {
      analysisInvalidated.value = false
      setStatus(t('carrierTenders.xlsx.status.previewReady'))
    }
  } catch (error) {
    preview.value = null
    analysisInvalidated.value = true
    lastError.value = classifyCarrierXlsxHttpError(error)
    if (lastError.value.disableImport) {
      importDisabled.value = true
    }
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    previewing.value = false
  }
}

function retryPreview() {
  if (!selectedFile.value) {
    fileInput.value?.focus()
    return
  }
  void runPreview(selectedFile.value)
}

async function exportWorkbook() {
  exporting.value = true
  lastError.value = null
  setStatus(t('carrierTenders.xlsx.status.exporting'))
  try {
    const result = await exportCarrierDraft(props.eventId, props.responseId)
    const url = URL.createObjectURL(result.blob)
    const link = document.createElement('a')
    link.href = url
    link.download = result.filename || `rfx-carrier-${props.responseId}.xlsx`
    link.click()
    URL.revokeObjectURL(url)
    setStatus(t('carrierTenders.xlsx.status.exported'))
    pushToast('success', t('carrierTenders.xlsx.exportSuccess'))
  } catch (error) {
    lastError.value = classifyCarrierXlsxHttpError(error)
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    exporting.value = false
  }
}

async function commitImport() {
  if (
    !canCommitCarrierXlsxPreview(preview.value, {
      analysisInvalidated: analysisInvalidated.value,
      alreadyCommitted: Boolean(commitResult.value),
      importDisabled: importDisabled.value,
    })
    || !preview.value?.analysis_id
    || !importEnabled.value
  ) {
    return
  }
  committing.value = true
  lastError.value = null
  setStatus(t('carrierTenders.xlsx.status.committing'))
  try {
    const key = idempotency.keyForAnalysis(preview.value.analysis_id)
    const result = await commitCarrierDraft(
      props.eventId,
      props.responseId,
      { analysis_id: preview.value.analysis_id },
      key,
    )
    commitResult.value = result
    analysisInvalidated.value = true
    setStatus(t('carrierTenders.xlsx.status.committed'))
    pushToast('success', t('carrierTenders.xlsx.commitSuccess'))
    emit('committed', result)
  } catch (error) {
    lastError.value = classifyCarrierXlsxHttpError(error)
    if (shouldInvalidateCarrierXlsxAnalysis(lastError.value.kind)) {
      analysisInvalidated.value = true
    }
    if (lastError.value.disableImport) {
      importDisabled.value = true
    }
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    committing.value = false
  }
}

function issueCopy(issue: { message_key: string; machine_code: string; sheet?: string; row?: number }) {
  return resolveCarrierXlsxIssueCopy(issue)
}
</script>

<template>
  <div
    data-testid="carrier-xlsx-gate"
    :data-enabled="excelEnabled ? 'true' : 'false'"
    :data-status="responseStatus"
    :data-import-enabled="importEnabled ? 'true' : 'false'"
    :data-response-id="responseId || ''"
  >
  <Card v-if="visible" data-testid="carrier-xlsx-panel">
    <template #header>
      <h3>{{ $t('carrierTenders.xlsx.title') }}</h3>
    </template>
    <p class="carrier-xlsx__hint">{{ $t('carrierTenders.xlsx.hint') }}</p>

    <div class="carrier-xlsx__actions">
      <Button
        variant="secondary"
        data-testid="carrier-xlsx-export"
        :loading="exporting"
        :disabled="busy"
        @click="exportWorkbook"
      >
        {{ $t('carrierTenders.xlsx.export') }}
      </Button>
      <label v-if="importEnabled" class="carrier-xlsx__upload">
        <span class="sr-only">{{ $t('carrierTenders.xlsx.uploadLabel') }}</span>
        <input
          ref="fileInput"
          data-testid="carrier-xlsx-file"
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          :disabled="busy"
          :aria-describedby="'carrier-xlsx-status'"
          @change="onFileChange"
        >
        <span class="carrier-xlsx__upload-text">
          {{ selectedFile ? selectedFile.name : $t('carrierTenders.xlsx.chooseFile') }}
        </span>
      </label>
    </div>
    <p v-if="importEnabled" class="carrier-xlsx__limit">
      {{ $t('carrierTenders.xlsx.sizeLimit', { size: CARRIER_XLSX_MAX_UPLOAD_BYTES / (1024 * 1024) }) }}
    </p>
    <p v-else class="carrier-xlsx__limit">{{ $t('carrierTenders.xlsx.exportOnlyHint') }}</p>

    <div
      id="carrier-xlsx-status"
      class="carrier-xlsx__status"
      data-testid="carrier-xlsx-status"
      role="status"
      aria-live="polite"
    >
      {{ statusMessage }}
    </div>

    <div
      v-if="lastError"
      class="carrier-xlsx__error"
      data-testid="carrier-xlsx-error"
      :data-error-kind="lastError.kind"
      role="alert"
    >
      <p>{{ $t(lastError.messageKey) }}</p>
      <p v-if="lastError.machineCode" class="carrier-xlsx__machine">
        {{ $t('carrierTenders.xlsx.machineCode', { code: lastError.machineCode }) }}
      </p>
      <Button
        v-if="showRetryPreview"
        variant="secondary"
        size="sm"
        data-testid="carrier-xlsx-retry-preview"
        :disabled="busy || !selectedFile"
        @click="retryPreview"
      >
        {{ $t('carrierTenders.xlsx.retryPreview') }}
      </Button>
    </div>

    <div v-if="preview" class="carrier-xlsx__preview" data-testid="carrier-xlsx-preview">
      <dl class="detail-grid">
        <dt>{{ $t('carrierTenders.xlsx.readyToCommit') }}</dt>
        <dd>{{ preview.ready_to_commit ? $t('common.yes') : $t('common.no') }}</dd>
        <dt>{{ $t('carrierTenders.xlsx.mode') }}</dt>
        <dd>{{ preview.mode }}</dd>
        <dt>{{ $t('carrierTenders.xlsx.errorsCount') }}</dt>
        <dd>{{ preview.summary.errors ?? preview.errors.length }}</dd>
        <dt>{{ $t('carrierTenders.xlsx.warningsCount') }}</dt>
        <dd>{{ preview.summary.warnings ?? preview.warnings.length }}</dd>
      </dl>

      <section v-if="preview.errors.length" aria-labelledby="carrier-xlsx-errors-title">
        <h4 id="carrier-xlsx-errors-title">{{ $t('carrierTenders.xlsx.validationErrors') }}</h4>
        <ul>
          <li
            v-for="(issue, index) in preview.errors"
            :key="`err-${index}`"
            data-testid="carrier-xlsx-issue"
          >
            <span data-testid="carrier-xlsx-issue-text">{{ $t(issueCopy(issue).i18nKey) }}</span>
            <span v-if="issueCopy(issue).location" class="carrier-xlsx__issue-loc">
              {{ issueCopy(issue).location }}
            </span>
            <span class="carrier-xlsx__machine" data-testid="carrier-xlsx-issue-code">
              {{ $t('carrierTenders.xlsx.machineCode', { code: issueCopy(issue).technicalCode }) }}
            </span>
          </li>
        </ul>
      </section>
      <section v-if="preview.warnings.length" aria-labelledby="carrier-xlsx-warnings-title">
        <h4 id="carrier-xlsx-warnings-title">{{ $t('carrierTenders.xlsx.validationWarnings') }}</h4>
        <ul>
          <li
            v-for="(issue, index) in preview.warnings"
            :key="`warn-${index}`"
            data-testid="carrier-xlsx-warning"
          >
            <span data-testid="carrier-xlsx-warning-text">{{ $t(issueCopy(issue).i18nKey) }}</span>
            <span v-if="issueCopy(issue).location" class="carrier-xlsx__issue-loc">
              {{ issueCopy(issue).location }}
            </span>
            <span class="carrier-xlsx__machine" data-testid="carrier-xlsx-warning-code">
              {{ $t('carrierTenders.xlsx.machineCode', { code: issueCopy(issue).technicalCode }) }}
            </span>
          </li>
        </ul>
      </section>
    </div>

    <div v-if="importEnabled" class="carrier-xlsx__commit">
      <Button
        data-testid="carrier-xlsx-commit"
        :disabled="!commitEnabled"
        :loading="committing"
        @click="commitImport"
      >
        {{ $t('carrierTenders.xlsx.commit') }}
      </Button>
      <p v-if="commitResult" class="carrier-xlsx__success" data-testid="carrier-xlsx-committed">
        {{ $t('carrierTenders.xlsx.commitSuccess') }}
      </p>
    </div>
  </Card>
  </div>
</template>

<style scoped>
.carrier-xlsx__hint,
.carrier-xlsx__limit,
.carrier-xlsx__status,
.carrier-xlsx__machine,
.carrier-xlsx__issue-loc {
  color: var(--color-text-muted);
  margin: 0 0 0.75rem;
}

.carrier-xlsx__issue-loc,
li .carrier-xlsx__machine {
  display: inline;
  margin: 0 0 0 0.5rem;
}

.carrier-xlsx__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  margin-bottom: 0.5rem;
}

.carrier-xlsx__upload {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.carrier-xlsx__upload input {
  max-width: 16rem;
}

.carrier-xlsx__error {
  border: 1px solid var(--color-danger, #b42318);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
}

.carrier-xlsx__preview {
  margin: 1rem 0;
}

.carrier-xlsx__commit {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}

.carrier-xlsx__success {
  color: var(--color-success, #067647);
  margin: 0;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}

.detail-grid {
  display: grid;
  grid-template-columns: 12rem 1fr;
  gap: 0.75rem 1rem;
  margin: 0 0 1rem;
}

.detail-grid dt {
  color: var(--color-text-muted);
  font-weight: 500;
}

.detail-grid dd {
  margin: 0;
}
</style>
