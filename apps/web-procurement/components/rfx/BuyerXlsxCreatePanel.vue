<script setup lang="ts">
import type { BuyerXlsxCreateClassifiedError } from '~/utils/buyerXlsxCreateErrors'
import type {
  BuyerXlsxCreateCommitResponse,
  BuyerXlsxCreateMetadata,
  BuyerXlsxCreatePreviewResponse,
} from '~/types/buyerXlsxCreate'
import { RFX_CATEGORIES, RFX_TYPES, emptyTenderWizardForm } from '~/types/rfx'
import {
  filterBuyerMemberships,
  membershipSelectOptions,
  selectDefaultOwnerCompany,
} from '~/utils/companyMembership'
import { canShowBuyerXlsxCreateEntry } from '~/utils/buyerXlsxAccess'
import {
  canCommitBuyerXlsxCreatePreview,
  classifyBuyerXlsxCreateClientError,
  classifyBuyerXlsxCreateHttpError,
  shouldInvalidateBuyerXlsxCreateAnalysis,
} from '~/utils/buyerXlsxCreateErrors'
import { createBuyerXlsxCreateIdempotencyStore } from '~/utils/buyerXlsxCreateIdempotency'
import {
  buyerXlsxCreatePreviewFingerprint,
  validateBuyerXlsxCreateInput,
} from '~/utils/buyerXlsxCreateForm'
import { BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES } from '~/utils/buyerXlsxCreateApiRoutes'
import { buyerXlsxCreateTemplateFlowSnapshot } from '~/utils/buyerXlsxCreateTemplate'
import type { BuyerXlsxCreateTemplateLabels } from '~/utils/buyerXlsxCreateTemplate'
import { resolveBuyerXlsxIssueCopy } from '~/utils/buyerXlsxIssueText'
import BuyerXlsxCreateTemplateDownload from '~/components/rfx/BuyerXlsxCreateTemplateDownload.vue'
import Button from '~/components/ui/Button.vue'
import Card from '~/components/ui/Card.vue'
import Input from '~/components/ui/Input.vue'
import Select from '~/components/ui/Select.vue'

const emit = defineEmits<{
  committed: [result: BuyerXlsxCreateCommitResponse]
}>()

const { t } = useI18n()
const { user } = useAuth()
const { getUserCompanies } = useCompanies()
const { pushToast } = useToast()
const { setCompany } = useTenantContext()
const { enabled: excelEnabled } = useRfxExcelExchangeFeature()
const { previewBuyerCreate, commitBuyerCreate, downloadBuyerCreateTemplate } = useBuyerXlsxCreateApi()
const router = useRouter()
const idempotency = createBuyerXlsxCreateIdempotencyStore()

const defaults = emptyTenderWizardForm()
const metadata = reactive<BuyerXlsxCreateMetadata>({
  owner_company_id: defaults.owner_company_id,
  rfx_number: `XLSX-${Date.now().toString().slice(-6)}`,
  title: '',
  rfx_type: 'SPOT_RFQ',
  category: 'FREIGHT',
  description: '',
  response_deadline: defaults.response_deadline,
  currency_code: defaults.currency_code,
})

const fileInput = ref<HTMLInputElement | null>(null)
const errorRegion = ref<HTMLElement | null>(null)
const resultRegion = ref<HTMLElement | null>(null)
const selectedFile = ref<File | null>(null)
const ownerOptions = ref<Array<{ label: string; value: string }>>([])
const previewing = ref(false)
const committing = ref(false)
const preview = ref<BuyerXlsxCreatePreviewResponse | null>(null)
const commitResult = ref<BuyerXlsxCreateCommitResponse | null>(null)
const lastError = ref<BuyerXlsxCreateClassifiedError | null>(null)
const analysisInvalidated = ref(false)
const statusMessage = ref('')
const previewFingerprint = ref('')

const buyerRoles = computed(() => useAuthStore().user?.roles ?? [])
const visible = computed(() =>
  canShowBuyerXlsxCreateEntry({
    excelExchangeEnabled: excelEnabled.value,
    roles: buyerRoles.value,
  }),
)
const templateLabels = computed<BuyerXlsxCreateTemplateLabels>(() => ({
  download: t('tenders.buyerXlsxCreate.template.download'),
  downloading: t('tenders.buyerXlsxCreate.template.status.downloading'),
  success: t('tenders.buyerXlsxCreate.template.status.success'),
  hint: t('tenders.buyerXlsxCreate.template.hint'),
  unauthorized: t('tenders.buyerXlsxCreate.template.errors.unauthorized'),
  forbidden: t('tenders.buyerXlsxCreate.template.errors.forbidden'),
  notFound: t('tenders.buyerXlsxCreate.template.errors.notFound'),
  rateLimited: t('tenders.buyerXlsxCreate.template.errors.rateLimited'),
  unavailable: t('tenders.buyerXlsxCreate.template.errors.unavailable'),
  invalidBinary: t('tenders.buyerXlsxCreate.template.errors.invalidBinary'),
}))
const templateFlow = computed(() => buyerXlsxCreateTemplateFlowSnapshot({
  file: selectedFile.value,
  metadata,
  preview: preview.value,
  idempotencyKey: preview.value?.analysis_id
    ? idempotency.currentKey(preview.value.analysis_id)
    : null,
}))
const busy = computed(() => previewing.value || committing.value)
const commitEnabled = computed(() =>
  canCommitBuyerXlsxCreatePreview(preview.value, {
    analysisInvalidated: analysisInvalidated.value,
    alreadyCommitted: Boolean(commitResult.value),
  }) && !busy.value,
)
const typeOptions = computed(() => RFX_TYPES.map((value) => ({ label: value, value })))
const categoryOptions = computed(() => RFX_CATEGORIES.map((value) => ({ label: value, value })))

function setStatus(message: string) {
  statusMessage.value = message
}

function invalidateCurrentPreview() {
  preview.value = null
  analysisInvalidated.value = true
  previewFingerprint.value = ''
  idempotency.reset()
  if (!commitResult.value) {
    setStatus(t('tenders.buyerXlsxCreate.status.previewReset'))
  }
}

function applyStoredOwnerFallback() {
  const stored = localStorage.getItem('freight_procurement_company_id') || metadata.owner_company_id
  if (!stored) return
  metadata.owner_company_id = stored
  if (ownerOptions.value.length === 0) {
    ownerOptions.value = [{ label: stored, value: stored }]
  }
}

if (import.meta.client) {
  applyStoredOwnerFallback()
}

async function loadOwnerCompanies() {
  applyStoredOwnerFallback()
  if (!user.value?.id) return
  try {
    const raw = await getUserCompanies(user.value.id)
    const memberships = filterBuyerMemberships(raw)
    const options = membershipSelectOptions(memberships)
    if (options.length > 0) {
      ownerOptions.value = options
    }
    if (!metadata.owner_company_id) {
      metadata.owner_company_id = selectDefaultOwnerCompany(memberships)
    }
    applyStoredOwnerFallback()
  } catch {
    applyStoredOwnerFallback()
  }
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFile.value = input.files?.[0] ?? null
  invalidateCurrentPreview()
}

watch(
  metadata,
  () => {
    if (!preview.value && !previewFingerprint.value) return
    const next = buyerXlsxCreatePreviewFingerprint(selectedFile.value, metadata)
    if (next !== previewFingerprint.value) {
      invalidateCurrentPreview()
    }
  },
  { deep: true },
)

watch(lastError, async (error) => {
  if (!error) return
  await nextTick()
  errorRegion.value?.focus()
})

watch(preview, async (value) => {
  if (!value) return
  await nextTick()
  resultRegion.value?.focus()
})

async function runPreview() {
  const validation = validateBuyerXlsxCreateInput(selectedFile.value, metadata)
  if (!validation.ok || !selectedFile.value) {
    lastError.value = classifyBuyerXlsxCreateClientError(validation.kind || 'file_required')
    setStatus(t(lastError.value.messageKey))
    return
  }
  previewing.value = true
  lastError.value = null
  commitResult.value = null
  setStatus(t('tenders.buyerXlsxCreate.status.previewing'))
  try {
    const result = await previewBuyerCreate(selectedFile.value, metadata)
    preview.value = result.preview
    previewFingerprint.value = buyerXlsxCreatePreviewFingerprint(selectedFile.value, metadata)
    idempotency.rememberPreview(result.preview.analysis_id)
    if (result.status === 422 || !result.preview.ready_to_commit) {
      analysisInvalidated.value = true
      lastError.value = {
        kind: 'preview_invalid',
        status: 422,
        machineCode: result.preview.errors[0]?.machine_code ?? null,
        retryPreview: false,
        retryCommit: false,
        messageKey: 'tenders.buyerXlsxCreate.errors.previewInvalid',
      }
      setStatus(t('tenders.buyerXlsxCreate.status.previewBlocked'))
    } else {
      analysisInvalidated.value = false
      setStatus(t('tenders.buyerXlsxCreate.status.previewReady'))
    }
  } catch (error) {
    preview.value = null
    analysisInvalidated.value = true
    lastError.value = classifyBuyerXlsxCreateHttpError(error)
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    previewing.value = false
  }
}

async function commitCreate() {
  if (
    !canCommitBuyerXlsxCreatePreview(preview.value, {
      analysisInvalidated: analysisInvalidated.value,
      alreadyCommitted: Boolean(commitResult.value),
    })
    || !preview.value?.analysis_id
    || committing.value
  ) {
    return
  }
  committing.value = true
  lastError.value = null
  setStatus(t('tenders.buyerXlsxCreate.status.committing'))
  try {
    const key = idempotency.keyForAnalysis(preview.value.analysis_id)
    const result = await commitBuyerCreate(preview.value.analysis_id, key)
    commitResult.value = result
    analysisInvalidated.value = true
    if (metadata.owner_company_id) {
      setCompany(metadata.owner_company_id)
    }
    setStatus(t('tenders.buyerXlsxCreate.status.committed'))
    pushToast('success', t('tenders.buyerXlsxCreate.commitSuccess'))
    emit('committed', result)
    await router.push(`/tenders/${result.event_id}`)
  } catch (error) {
    lastError.value = classifyBuyerXlsxCreateHttpError(error)
    if (shouldInvalidateBuyerXlsxCreateAnalysis(lastError.value.kind)) {
      analysisInvalidated.value = true
    }
    setStatus(t(lastError.value.messageKey))
    pushToast('error', t(lastError.value.messageKey))
  } finally {
    committing.value = false
  }
}

function issueCopy(issue: { message_key: string; machine_code: string; sheet?: string; row?: number }) {
  return resolveBuyerXlsxIssueCopy(issue)
}

onMounted(() => {
  void loadOwnerCompanies()
})
</script>

<template>
  <div
    data-testid="buyer-xlsx-create-gate"
    :data-enabled="excelEnabled ? 'true' : 'false'"
    :data-visible="visible ? 'true' : 'false'"
  >
    <Card v-if="!visible" data-testid="buyer-xlsx-create-unavailable">
      <p>{{ $t('tenders.buyerXlsxCreate.unavailable') }}</p>
      <div class="buyer-xlsx-create__actions">
        <Button
          variant="secondary"
          data-testid="buyer-xlsx-create-back-manual"
          @click="$router.push('/tenders/new')"
        >
          {{ $t('tenders.buyerXlsxCreate.backManual') }}
        </Button>
      </div>
    </Card>

    <Card v-else data-testid="buyer-xlsx-create-panel">
      <template #header>
        <h2>{{ $t('tenders.buyerXlsxCreate.title') }}</h2>
      </template>
      <p class="buyer-xlsx-create__hint">{{ $t('tenders.buyerXlsxCreate.hint') }}</p>

      <fieldset class="buyer-xlsx-create__fieldset" :disabled="busy || Boolean(commitResult)">
        <legend class="sr-only">{{ $t('tenders.buyerXlsxCreate.metadataLegend') }}</legend>
        <div class="form-grid form-grid--2">
          <label class="ui-select">
            <span class="ui-select__label">{{ $t('tenders.ownerCompany') }}</span>
            <select
              v-model="metadata.owner_company_id"
              data-testid="buyer-xlsx-create-owner"
              required
            >
              <option v-for="opt in ownerOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>
          </label>
          <Input
            v-model="metadata.rfx_number"
            :label="$t('tenders.number')"
            data-testid="buyer-xlsx-create-rfx-number"
            required
          />
          <Select
            v-model="metadata.rfx_type"
            :label="$t('tenders.type')"
            :options="typeOptions"
            data-testid="buyer-xlsx-create-type"
          />
          <Select
            v-model="metadata.category"
            :label="$t('tenders.category')"
            :options="categoryOptions"
            data-testid="buyer-xlsx-create-category"
          />
          <Input
            v-model="metadata.title"
            :label="$t('tenders.titleLabel')"
            data-testid="buyer-xlsx-create-title"
            required
          />
          <Input
            v-model="metadata.description"
            :label="$t('tenders.description')"
            data-testid="buyer-xlsx-create-description"
          />
          <Input
            v-model="metadata.response_deadline"
            :label="$t('tenders.deadline')"
            type="datetime-local"
            data-testid="buyer-xlsx-create-deadline"
          />
          <Input
            v-model="metadata.currency_code"
            :label="$t('tenders.currency')"
            data-testid="buyer-xlsx-create-currency"
          />
        </div>
      </fieldset>

      <div class="buyer-xlsx-create__acquire">
        <BuyerXlsxCreateTemplateDownload
          :excel-exchange-enabled="excelEnabled"
          :roles="buyerRoles"
          :download="downloadBuyerCreateTemplate"
          :labels="templateLabels"
          :flow="templateFlow"
        />
        <div class="buyer-xlsx-create__upload-column">
          <label class="buyer-xlsx-create__upload">
            <span class="buyer-xlsx-create__upload-label">{{ $t('tenders.buyerXlsxCreate.uploadLabel') }}</span>
            <input
              ref="fileInput"
              data-testid="buyer-xlsx-create-file"
              type="file"
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              :disabled="busy || Boolean(commitResult)"
              :aria-describedby="'buyer-xlsx-create-status'"
              @change="onFileChange"
            >
            <span class="buyer-xlsx-create__upload-text">
              {{ selectedFile ? selectedFile.name : $t('tenders.buyerXlsxCreate.chooseFile') }}
            </span>
          </label>
          <p class="buyer-xlsx-create__limit">
            {{ $t('tenders.buyerXlsxCreate.sizeLimit', { size: BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES / (1024 * 1024) }) }}
          </p>
        </div>
      </div>

      <div class="buyer-xlsx-create__actions">
        <Button
          variant="secondary"
          data-testid="buyer-xlsx-create-back-manual"
          :disabled="busy"
          @click="$router.push('/tenders/new')"
        >
          {{ $t('tenders.buyerXlsxCreate.backManual') }}
        </Button>
        <Button
          variant="secondary"
          data-testid="buyer-xlsx-create-preview"
          :loading="previewing"
          :disabled="busy || Boolean(commitResult)"
          @click="runPreview"
        >
          {{ $t('tenders.buyerXlsxCreate.preview') }}
        </Button>
        <Button
          data-testid="buyer-xlsx-create-commit"
          :loading="committing"
          :disabled="!commitEnabled"
          @click="commitCreate"
        >
          {{ $t('tenders.buyerXlsxCreate.commit') }}
        </Button>
      </div>

      <div
        id="buyer-xlsx-create-status"
        class="buyer-xlsx-create__status"
        data-testid="buyer-xlsx-create-status"
        role="status"
        aria-live="polite"
      >
        {{ statusMessage }}
      </div>

      <div
        v-if="lastError"
        ref="errorRegion"
        class="buyer-xlsx-create__error"
        data-testid="buyer-xlsx-create-error"
        :data-error-kind="lastError.kind"
        role="alert"
        tabindex="-1"
      >
        <p>{{ $t(lastError.messageKey) }}</p>
        <p v-if="lastError.machineCode" class="buyer-xlsx-create__machine">
          {{ $t('tenders.buyerXlsxCreate.machineCode', { code: lastError.machineCode }) }}
        </p>
      </div>

      <div
        v-if="preview"
        ref="resultRegion"
        class="buyer-xlsx-create__preview"
        data-testid="buyer-xlsx-create-result"
        tabindex="-1"
        aria-live="polite"
      >
        <dl class="detail-grid">
          <dt>{{ $t('tenders.buyerXlsxCreate.analysisId') }}</dt>
          <dd data-testid="buyer-xlsx-create-analysis-id">{{ preview.analysis_id || '—' }}</dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.readyToCommit') }}</dt>
          <dd data-testid="buyer-xlsx-create-ready">
            {{ preview.ready_to_commit ? $t('common.yes') : $t('common.no') }}
          </dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.mode') }}</dt>
          <dd>{{ preview.mode }}</dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.proposalTitle') }}</dt>
          <dd data-testid="buyer-xlsx-create-summary">
            {{ preview.normalized_draft_summary.title }}
            ({{ preview.normalized_draft_summary.rfx_number }})
          </dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.lotCount') }}</dt>
          <dd>{{ preview.normalized_draft_summary.lot_count }}</dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.sectionCount') }}</dt>
          <dd>{{ preview.normalized_draft_summary.section_count }}</dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.questionCount') }}</dt>
          <dd>{{ preview.normalized_draft_summary.question_count }}</dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.errorsCount') }}</dt>
          <dd>{{ preview.errors.length }}</dd>
          <dt>{{ $t('tenders.buyerXlsxCreate.warningsCount') }}</dt>
          <dd>{{ preview.warnings.length }}</dd>
        </dl>

        <section
          v-if="preview.errors.length || preview.warnings.length"
          data-testid="buyer-xlsx-create-findings"
          aria-labelledby="buyer-xlsx-create-findings-title"
        >
          <h3 id="buyer-xlsx-create-findings-title">{{ $t('tenders.buyerXlsxCreate.findings') }}</h3>
          <ul>
            <li
              v-for="(issue, index) in preview.errors"
              :key="`err-${index}`"
              data-testid="buyer-xlsx-create-issue"
            >
              <span>{{ $t(issueCopy(issue).i18nKey) }}</span>
              <span v-if="issueCopy(issue).location" class="buyer-xlsx-create__issue-loc">
                {{ issueCopy(issue).location }}
              </span>
              <span
                v-if="issueCopy(issue).technicalCode"
                class="buyer-xlsx-create__machine"
                data-testid="buyer-xlsx-create-issue-code"
              >
                {{ $t('tenders.buyerXlsxCreate.machineCode', { code: issueCopy(issue).technicalCode }) }}
              </span>
            </li>
            <li
              v-for="(issue, index) in preview.warnings"
              :key="`warn-${index}`"
              data-testid="buyer-xlsx-create-warning"
            >
              <span>{{ $t(issueCopy(issue).i18nKey) }}</span>
              <span v-if="issueCopy(issue).location" class="buyer-xlsx-create__issue-loc">
                {{ issueCopy(issue).location }}
              </span>
            </li>
          </ul>
        </section>
      </div>

      <p v-if="commitResult" class="buyer-xlsx-create__success" data-testid="buyer-xlsx-create-committed">
        {{ $t('tenders.buyerXlsxCreate.commitSuccess') }}
      </p>
    </Card>
  </div>
</template>

<style scoped>
.buyer-xlsx-create__hint,
.buyer-xlsx-create__limit,
.buyer-xlsx-create__status,
.buyer-xlsx-create__machine,
.buyer-xlsx-create__issue-loc {
  color: var(--color-text-muted);
  margin: 0 0 0.75rem;
}

.buyer-xlsx-create__fieldset {
  border: 0;
  margin: 0 0 1rem;
  padding: 0;
}

.buyer-xlsx-create__acquire {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem 1.5rem;
  align-items: flex-start;
  margin-bottom: 0.5rem;
}

.buyer-xlsx-create__upload-column {
  flex: 1 1 16rem;
}

.buyer-xlsx-create__upload {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin-bottom: 0.5rem;
}

.buyer-xlsx-create__upload-label {
  font-size: 0.875rem;
  font-weight: 500;
}

.buyer-xlsx-create__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  margin: 1rem 0;
}

.buyer-xlsx-create__error {
  border: 1px solid var(--color-danger, #b42318);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
}

.buyer-xlsx-create__preview {
  margin: 1rem 0;
}

.buyer-xlsx-create__success {
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
