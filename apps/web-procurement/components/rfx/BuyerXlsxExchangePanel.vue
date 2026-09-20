<script setup lang="ts">
import type { BuyerXlsxClassifiedError } from '~/utils/buyerXlsxErrors'
import type { BuyerXlsxCommitResponse, BuyerXlsxPreviewResponse } from '~/types/buyerXlsx'
import { canCommitBuyerXlsxPreview, classifyBuyerXlsxHttpError } from '~/utils/buyerXlsxErrors'
import { createBuyerXlsxIdempotencyStore } from '~/utils/buyerXlsxIdempotency'
import { BUYER_XLSX_MAX_UPLOAD_BYTES } from '~/utils/buyerXlsxApiRoutes'

const props = defineProps<{
  eventId: string
  eventStatus: string
}>()

const emit = defineEmits<{
  committed: [result: BuyerXlsxCommitResponse]
}>()

const { t } = useI18n()
const { pushToast } = useToast()
const { enabled: excelEnabled } = useRfxExcelExchangeFeature()
const { canBuyerXlsxExchange } = usePermissions()
const { exportBuyerDraft, previewBuyerDraft, commitBuyerDraft } = useBuyerXlsxApi()
const idempotency = createBuyerXlsxIdempotencyStore()

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const exporting = ref(false)
const previewing = ref(false)
const committing = ref(false)
const preview = ref<BuyerXlsxPreviewResponse | null>(null)
const commitResult = ref<BuyerXlsxCommitResponse | null>(null)
const lastError = ref<BuyerXlsxClassifiedError | null>(null)
const statusMessage = ref('')

const visible = computed(
  () => excelEnabled.value && canBuyerXlsxExchange() && props.eventStatus === 'DRAFT',
)
const busy = computed(() => exporting.value || previewing.value || committing.value)
const commitEnabled = computed(() => canCommitBuyerXlsxPreview(preview.value) && !busy.value)
const showRetryPreview = computed(() => Boolean(lastError.value?.retryPreview))

function setStatus(message: string) {
  statusMessage.value = message
}

function clearImportState() {
  preview.value = null
  commitResult.value = null
  lastError.value = null
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
  previewing.value = true
  lastError.value = null
  commitResult.value = null
  setStatus(t('tenders.buyerXlsx.status.previewing'))
  try {
    const result = await previewBuyerDraft(props.eventId, file)
    preview.value = result.preview
    idempotency.rememberPreview(result.preview.analysis_id)
    if (result.status === 422 || !result.preview.ready_to_commit) {
      lastError.value = {
        kind: 'preview_invalid',
        status: 422,
        machineCode: result.preview.errors[0]?.machine_code ?? null,
        retryPreview: false,
        messageKey: 'tenders.buyerXlsx.errors.previewInvalid',
      }
      setStatus(t('tenders.buyerXlsx.status.previewBlocked'))
    } else {
      setStatus(t('tenders.buyerXlsx.status.previewReady'))
    }
  } catch (error) {
    preview.value = null
    lastError.value = classifyBuyerXlsxHttpError(error)
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
  setStatus(t('tenders.buyerXlsx.status.exporting'))
  try {
    const result = await exportBuyerDraft(props.eventId)
    const url = URL.createObjectURL(result.blob)
    const link = document.createElement('a')
    link.href = url
    link.download = result.filename || `rfx-${props.eventId}.xlsx`
    link.click()
    URL.revokeObjectURL(url)
    setStatus(t('tenders.buyerXlsx.status.exported'))
    pushToast('success', t('tenders.buyerXlsx.exportSuccess'))
  } catch (error) {
    lastError.value = classifyBuyerXlsxHttpError(error)
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    exporting.value = false
  }
}

async function commitImport() {
  if (!canCommitBuyerXlsxPreview(preview.value) || !preview.value?.analysis_id) return
  committing.value = true
  lastError.value = null
  setStatus(t('tenders.buyerXlsx.status.committing'))
  try {
    const key = idempotency.keyForAnalysis(preview.value.analysis_id)
    const result = await commitBuyerDraft(
      props.eventId,
      { analysis_id: preview.value.analysis_id },
      key,
    )
    commitResult.value = result
    setStatus(t('tenders.buyerXlsx.status.committed'))
    pushToast('success', t('tenders.buyerXlsx.commitSuccess'))
    emit('committed', result)
  } catch (error) {
    lastError.value = classifyBuyerXlsxHttpError(error)
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    committing.value = false
  }
}

function issueLabel(issue: { message_key: string; machine_code: string; sheet?: string; row?: number }) {
  const location = [issue.sheet, issue.row != null ? String(issue.row) : '']
    .filter(Boolean)
    .join(':')
  return location
    ? `${issue.machine_code} · ${issue.message_key} (${location})`
    : `${issue.machine_code} · ${issue.message_key}`
}
</script>

<template>
  <Card v-if="visible" data-testid="buyer-xlsx-panel">
    <template #header>
      <h3>{{ $t('tenders.buyerXlsx.title') }}</h3>
    </template>
    <p class="buyer-xlsx__hint">{{ $t('tenders.buyerXlsx.hint') }}</p>

    <div class="buyer-xlsx__actions">
      <Button
        variant="secondary"
        data-testid="buyer-xlsx-export"
        :loading="exporting"
        :disabled="busy"
        @click="exportWorkbook"
      >
        {{ $t('tenders.buyerXlsx.export') }}
      </Button>
      <label class="buyer-xlsx__upload">
        <span class="sr-only">{{ $t('tenders.buyerXlsx.uploadLabel') }}</span>
        <input
          ref="fileInput"
          data-testid="buyer-xlsx-file"
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          :disabled="busy"
          :aria-describedby="'buyer-xlsx-status'"
          @change="onFileChange"
        >
        <span class="buyer-xlsx__upload-text">
          {{ selectedFile ? selectedFile.name : $t('tenders.buyerXlsx.chooseFile') }}
        </span>
      </label>
    </div>
    <p class="buyer-xlsx__limit">{{ $t('tenders.buyerXlsx.sizeLimit', { size: BUYER_XLSX_MAX_UPLOAD_BYTES / (1024 * 1024) }) }}</p>

    <div
      id="buyer-xlsx-status"
      class="buyer-xlsx__status"
      data-testid="buyer-xlsx-status"
      role="status"
      aria-live="polite"
    >
      {{ statusMessage }}
    </div>

    <div
      v-if="lastError"
      class="buyer-xlsx__error"
      data-testid="buyer-xlsx-error"
      :data-error-kind="lastError.kind"
      role="alert"
    >
      <p>{{ $t(lastError.messageKey) }}</p>
      <p v-if="lastError.machineCode" class="buyer-xlsx__machine">
        {{ $t('tenders.buyerXlsx.machineCode', { code: lastError.machineCode }) }}
      </p>
      <Button
        v-if="showRetryPreview"
        variant="secondary"
        size="sm"
        data-testid="buyer-xlsx-retry-preview"
        :disabled="busy || !selectedFile"
        @click="retryPreview"
      >
        {{ $t('tenders.buyerXlsx.retryPreview') }}
      </Button>
    </div>

    <div v-if="preview" class="buyer-xlsx__preview" data-testid="buyer-xlsx-preview">
      <dl class="detail-grid">
        <dt>{{ $t('tenders.buyerXlsx.readyToCommit') }}</dt>
        <dd>{{ preview.ready_to_commit ? $t('common.yes') : $t('common.no') }}</dd>
        <dt>{{ $t('tenders.buyerXlsx.mode') }}</dt>
        <dd>{{ preview.mode }}</dd>
        <dt>{{ $t('tenders.buyerXlsx.errorsCount') }}</dt>
        <dd>{{ preview.summary.errors ?? preview.errors.length }}</dd>
        <dt>{{ $t('tenders.buyerXlsx.warningsCount') }}</dt>
        <dd>{{ preview.summary.warnings ?? preview.warnings.length }}</dd>
      </dl>

      <section v-if="preview.errors.length" aria-labelledby="buyer-xlsx-errors-title">
        <h4 id="buyer-xlsx-errors-title">{{ $t('tenders.buyerXlsx.validationErrors') }}</h4>
        <ul>
          <li v-for="(issue, index) in preview.errors" :key="`err-${index}`">{{ issueLabel(issue) }}</li>
        </ul>
      </section>
      <section v-if="preview.warnings.length" aria-labelledby="buyer-xlsx-warnings-title">
        <h4 id="buyer-xlsx-warnings-title">{{ $t('tenders.buyerXlsx.validationWarnings') }}</h4>
        <ul>
          <li v-for="(issue, index) in preview.warnings" :key="`warn-${index}`">{{ issueLabel(issue) }}</li>
        </ul>
      </section>
    </div>

    <div class="buyer-xlsx__commit">
      <Button
        data-testid="buyer-xlsx-commit"
        :disabled="!commitEnabled"
        :loading="committing"
        @click="commitImport"
      >
        {{ $t('tenders.buyerXlsx.commit') }}
      </Button>
      <p v-if="commitResult" class="buyer-xlsx__success" data-testid="buyer-xlsx-committed">
        {{ $t('tenders.buyerXlsx.commitSuccess') }}
      </p>
    </div>
  </Card>
</template>

<style scoped>
.buyer-xlsx__hint,
.buyer-xlsx__limit,
.buyer-xlsx__status,
.buyer-xlsx__machine {
  color: var(--color-text-muted);
  margin: 0 0 0.75rem;
}

.buyer-xlsx__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  margin-bottom: 0.5rem;
}

.buyer-xlsx__upload {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.buyer-xlsx__upload input {
  max-width: 16rem;
}

.buyer-xlsx__error {
  border: 1px solid var(--color-danger, #b42318);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
}

.buyer-xlsx__preview {
  margin: 1rem 0;
}

.buyer-xlsx__commit {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}

.buyer-xlsx__success {
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
