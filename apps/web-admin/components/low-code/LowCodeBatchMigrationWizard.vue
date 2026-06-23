<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import {
  MAX_BATCH_MIGRATION_ENTITIES,
  buildLowCodeAuditLink,
  formatJsonValue,
  normalizeBatchMigrationPreviewResponse,
  parseEntityIdsTextarea,
  type BatchMigrateToActiveResponse,
  type BatchMigrationPreviewResponse,
  type MigrationPreviewItem,
  type MigrationPreviewStatus,
} from '~/types/lowCode'

const props = defineProps<{
  open: boolean
  entityType: string
  templateCode: string
  initialEntityId?: string
}>()

const emit = defineEmits<{
  close: []
  executed: [entityIds: string[]]
}>()

const {
  previewBatchMigrationToActive,
  batchMigrateCustomFieldValuesToActive,
  getMigrationErrorMessage,
  isApiUnavailableError,
} = useLowCodeApi()
const { t } = useI18n()

type WizardStep = 1 | 2 | 3 | 4
type PreviewFilter = 'ALL' | MigrationPreviewStatus
type WizardPhase = 'selecting' | 'loadingPreview' | 'previewLoaded' | 'executing' | 'executed' | 'error'

const step = ref<WizardStep>(1)
const phase = ref<WizardPhase>('selecting')
const entityIdsText = ref('')
const preview = ref<BatchMigrationPreviewResponse | null>(null)
const executeResult = ref<BatchMigrateToActiveResponse | null>(null)
const errorMessage = ref('')
const previewFilter = ref<PreviewFilter>('ALL')
const expandedRows = ref<Set<string>>(new Set())
const warningsConfirmed = ref(false)
const skipBlocked = ref(true)

const parsedIds = computed(() => parseEntityIdsTextarea(entityIdsText.value))
const entityIds = computed(() => parsedIds.value.ids)
const invalidLines = computed(() => parsedIds.value.invalidLines)

const selectionError = computed(() => {
  if (entityIds.value.length === 0) return t('lowCode.batchMigrationAtLeastOneEntity')
  if (invalidLines.value.length > 0) return t('lowCode.batchMigrationInvalidUuid')
  if (entityIds.value.length > MAX_BATCH_MIGRATION_ENTITIES) return t('lowCode.batchMigrationMaxEntities')
  return ''
})

const canProceedFromSelect = computed(() => !selectionError.value && entityIds.value.length > 0)

const isRequestInFlight = computed(() => phase.value === 'loadingPreview' || phase.value === 'executing')

const executeSummary = computed(() => executeResult.value?.summary ?? {
  total: 0,
  migrated: 0,
  skipped: 0,
  blocked: 0,
  failed: 0,
  warnings: 0,
})

const executeItems = computed(() => executeResult.value?.items ?? [])

const previewSummary = computed(() => preview.value?.summary ?? { total: 0, safe: 0, warnings: 0, blocked: 0 })

const filteredPreviewItems = computed(() => {
  const items = preview.value?.items ?? []
  if (previewFilter.value === 'ALL') return items
  return items.filter((item) => item.status === previewFilter.value)
})

const allBlocked = computed(
  () => previewSummary.value.total > 0 && previewSummary.value.blocked === previewSummary.value.total,
)

const hasWarnings = computed(() => previewSummary.value.warnings > 0)
const hasBlocked = computed(() => previewSummary.value.blocked > 0)

const canExecute = computed(() => {
  if (!preview.value || phase.value !== 'previewLoaded') return false
  if (allBlocked.value) return false
  if (hasWarnings.value && !warningsConfirmed.value) return false
  if (hasBlocked.value && !skipBlocked.value) return false
  return previewSummary.value.safe > 0 || (hasWarnings.value && warningsConfirmed.value)
})

const executeButtonLabel = computed(() => {
  if (hasWarnings.value && skipBlocked.value && hasBlocked.value) {
    return t('lowCode.batchMigrationExecuteSkipBlocked')
  }
  if (hasWarnings.value) return t('lowCode.batchMigrationExecuteWithWarnings')
  return t('lowCode.batchMigrationExecute')
})

const executeStatusLabel = computed(() => {
  const status = executeResult.value?.status
  switch (status) {
    case 'completed':
      return t('lowCode.batchMigrationCompleted')
    case 'partially_completed':
      return t('lowCode.batchMigrationPartiallyCompleted')
    case 'blocked':
      return t('lowCode.batchMigrationStatusBlocked')
    case 'failed':
      return t('lowCode.batchMigrationFailed')
    default:
      return status ?? '—'
  }
})

const batchAuditLink = computed(() =>
  buildLowCodeAuditLink({
    entity_type: props.entityType,
    category: 'batch_migrations',
    batch_id: executeResult.value?.batch_id,
    limit: 100,
  }),
)

function resetState() {
  step.value = 1
  phase.value = 'selecting'
  preview.value = null
  executeResult.value = null
  errorMessage.value = ''
  previewFilter.value = 'ALL'
  expandedRows.value = new Set()
  warningsConfirmed.value = false
  skipBlocked.value = true
  entityIdsText.value = props.initialEntityId?.trim() ? `${props.initialEntityId.trim()}\n` : ''
}

function useCurrentEntity() {
  if (!props.initialEntityId?.trim()) return
  const id = props.initialEntityId.trim()
  const { ids } = parseEntityIdsTextarea(entityIdsText.value)
  if (!ids.some((value) => value.toLowerCase() === id.toLowerCase())) {
    entityIdsText.value = entityIdsText.value.trim()
      ? `${entityIdsText.value.trim()}\n${id}\n`
      : `${id}\n`
  }
}

function toggleRowDetails(entityId: string) {
  const next = new Set(expandedRows.value)
  if (next.has(entityId)) next.delete(entityId)
  else next.add(entityId)
  expandedRows.value = next
}

function isRowExpanded(entityId: string) {
  return expandedRows.value.has(entityId)
}

function statusBadgeTone(status: MigrationPreviewStatus | string) {
  switch (status) {
    case 'SAFE':
    case 'migrated':
      return 'success'
    case 'WARNING':
    case 'migrated_with_warnings':
      return 'warning'
    case 'BLOCKED':
    case 'blocked':
    case 'failed':
      return 'danger'
    default:
      return 'neutral'
  }
}

function previewStatusLabel(status: MigrationPreviewStatus) {
  switch (status) {
    case 'SAFE':
      return t('lowCode.migrationStatusSafe')
    case 'WARNING':
      return t('lowCode.migrationStatusWarning')
    case 'BLOCKED':
      return t('lowCode.migrationStatusBlocked')
    default:
      return status
  }
}

function itemCount(fields: string[] | undefined): number {
  return Array.isArray(fields) ? fields.length : 0
}

function applyPreviewData(data: BatchMigrationPreviewResponse | null) {
  preview.value = data
  if (!data || !data.items.length) {
    phase.value = 'error'
    errorMessage.value = t('lowCode.migrationPreviewEmpty')
    return
  }
  phase.value = 'previewLoaded'
  step.value = 2
  warningsConfirmed.value = false
  skipBlocked.value = true
}

async function loadPreview() {
  if (!canProceedFromSelect.value || isRequestInFlight.value) return
  errorMessage.value = ''
  phase.value = 'loadingPreview'
  try {
    const data = await previewBatchMigrationToActive({
      entity_type: props.entityType,
      template_code: props.templateCode,
      entity_ids: entityIds.value,
    })
    applyPreviewData(data)
  } catch (error) {
    if (error instanceof ApiError && error.preview) {
      const normalized = normalizeBatchMigrationPreviewResponse(error.preview)
      if (normalized) {
        applyPreviewData(normalized)
        errorMessage.value = getMigrationErrorMessage(error)
        return
      }
    }
    phase.value = 'error'
    errorMessage.value = isApiUnavailableError(error)
      ? t('lowCode.serviceUnavailable')
      : getMigrationErrorMessage(error)
  }
}

function goToConfirm() {
  if (phase.value !== 'previewLoaded' || !preview.value) return
  step.value = 3
}

async function executeBatch() {
  if (!canExecute.value || !preview.value || isRequestInFlight.value) return
  errorMessage.value = ''
  phase.value = 'executing'
  try {
    const result = await batchMigrateCustomFieldValuesToActive({
      entity_type: props.entityType,
      template_code: props.templateCode,
      entity_ids: entityIds.value,
      allow_warnings: warningsConfirmed.value,
      skip_blocked: skipBlocked.value,
    })
    executeResult.value = result
    phase.value = 'executed'
    step.value = 4
    const migratedIds = result.items
      .filter((item) => item.status === 'migrated' || item.status === 'migrated_with_warnings')
      .map((item) => item.entity_id)
    emit('executed', migratedIds)
  } catch (error) {
    if (error instanceof ApiError && error.preview) {
      const normalized = normalizeBatchMigrationPreviewResponse(error.preview)
      if (normalized) {
        preview.value = normalized
        step.value = 2
      }
    }
    phase.value = preview.value ? 'previewLoaded' : 'error'
    step.value = preview.value ? 3 : 1
    errorMessage.value = getMigrationErrorMessage(error)
  }
}

function handleClose() {
  emit('close')
}

function backToSelect() {
  step.value = 1
  phase.value = 'selecting'
  preview.value = null
  executeResult.value = null
  errorMessage.value = ''
}

function renderItemDetails(item: MigrationPreviewItem) {
  return {
    copied_fields: item.copied_fields ?? [],
    legacy_fields: item.legacy_fields ?? [],
    missing_required_fields: item.missing_required_fields ?? [],
    incompatible_fields: item.incompatible_fields ?? [],
    warnings: item.warnings ?? [],
  }
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) resetState()
  },
)

watch(skipBlocked, (value) => {
  if (!value && hasBlocked.value) {
    errorMessage.value = t('lowCode.batchMigrationSkipBlockedHint')
  } else if (errorMessage.value === t('lowCode.batchMigrationSkipBlockedHint')) {
    errorMessage.value = ''
  }
})
</script>

<template>
  <div class="batch-wizard-root">
    <UiModal
      :open="open"
      :title="$t('lowCode.batchMigrationTitle')"
      @close="handleClose"
    >
      <div class="batch-wizard">
        <ol class="batch-wizard__steps" aria-label="Wizard steps">
          <li :class="{ 'batch-wizard__step--active': step === 1, 'batch-wizard__step--done': step > 1 }">
            {{ $t('lowCode.batchMigrationStepSelect') }}
          </li>
          <li :class="{ 'batch-wizard__step--active': step === 2, 'batch-wizard__step--done': step > 2 }">
            {{ $t('lowCode.batchMigrationStepPreview') }}
          </li>
          <li :class="{ 'batch-wizard__step--active': step === 3, 'batch-wizard__step--done': step > 3 }">
            {{ $t('lowCode.batchMigrationStepConfirm') }}
          </li>
          <li :class="{ 'batch-wizard__step--active': step === 4 }">
            {{ $t('lowCode.batchMigrationStepResult') }}
          </li>
        </ol>

        <p class="batch-wizard__context">
          {{ entityType }} · <code>{{ templateCode }}</code>
        </p>

        <div v-if="step === 1" class="batch-wizard__panel">
          <label class="batch-wizard__label">
            {{ $t('lowCode.batchMigrationEntityIds') }}
            <span class="batch-wizard__hint">{{ $t('lowCode.batchMigrationOneUuidPerLine') }}</span>
            <textarea
              v-model="entityIdsText"
              class="batch-wizard__textarea"
              rows="8"
              spellcheck="false"
              :placeholder="$t('lowCode.batchMigrationEntityIdsPlaceholder')"
            />
          </label>

          <div class="batch-wizard__inline-actions">
            <UiButton
              v-if="initialEntityId"
              size="sm"
              variant="secondary"
              @click="useCurrentEntity"
            >
              {{ $t('lowCode.batchMigrationUseCurrentEntity') }}
            </UiButton>
            <span class="batch-wizard__count">
              {{ entityIds.length }} / {{ MAX_BATCH_MIGRATION_ENTITIES }}
            </span>
          </div>

          <p v-if="invalidLines.length" class="batch-wizard__error">
            {{ $t('lowCode.batchMigrationInvalidUuid') }}:
            <code v-for="line in invalidLines" :key="line" class="batch-wizard__invalid">{{ line }}</code>
          </p>
          <p v-else-if="selectionError" class="batch-wizard__error">{{ selectionError }}</p>
        </div>

        <div v-else-if="phase === 'loadingPreview'" class="batch-wizard__muted">
          {{ $t('lowCode.batchMigrationLoadingPreview') }}
        </div>

        <div v-else-if="phase === 'error' && !preview" class="batch-wizard__error-panel">
          <p>{{ errorMessage || $t('lowCode.batchMigrationFailed') }}</p>
          <div class="batch-wizard__inline-actions">
            <UiButton
              size="sm"
              :disabled="!canProceedFromSelect || isRequestInFlight"
              :loading="phase === 'loadingPreview'"
              @click="loadPreview"
            >
              {{ $t('lowCode.batchMigrationRetryPreview') }}
            </UiButton>
            <UiButton size="sm" variant="secondary" @click="backToSelect">
              {{ $t('common.back') }}
            </UiButton>
          </div>
        </div>

        <template v-else-if="preview && (step === 2 || step === 3)">
          <dl class="batch-wizard__summary">
            <div>
              <dt>{{ $t('lowCode.batchMigrationTotal') }}</dt>
              <dd>{{ previewSummary.total }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationSafe') }}</dt>
              <dd>{{ previewSummary.safe }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationWarnings') }}</dt>
              <dd>{{ previewSummary.warnings }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationBlocked') }}</dt>
              <dd>{{ previewSummary.blocked }}</dd>
            </div>
          </dl>

          <div class="batch-wizard__filters">
            <button
              v-for="filter in (['ALL', 'SAFE', 'WARNING', 'BLOCKED'] as const)"
              :key="filter"
              type="button"
              class="batch-wizard__filter"
              :class="{ 'batch-wizard__filter--active': previewFilter === filter }"
              @click="previewFilter = filter"
            >
              {{ filter === 'ALL' ? $t('lowCode.batchMigrationFilterAll') : previewStatusLabel(filter) }}
            </button>
          </div>

          <div class="batch-wizard__table-wrap">
            <table class="batch-wizard__table">
              <thead>
                <tr>
                  <th>{{ $t('lowCode.entityId') }}</th>
                  <th>{{ $t('lowCode.migrationStatus') }}</th>
                  <th>{{ $t('lowCode.batchMigrationCopiedCount') }}</th>
                  <th>{{ $t('lowCode.batchMigrationLegacyCount') }}</th>
                  <th>{{ $t('lowCode.batchMigrationMissingRequiredCount') }}</th>
                  <th>{{ $t('lowCode.batchMigrationIncompatibleCount') }}</th>
                  <th>{{ $t('lowCode.batchMigrationWarningsCount') }}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <template v-for="item in filteredPreviewItems" :key="item.entity_id">
                  <tr>
                    <td class="batch-wizard__mono">{{ item.entity_id }}</td>
                    <td>
                      <UiBadge
                        :status="previewStatusLabel(item.status)"
                        :tone="statusBadgeTone(item.status)"
                      />
                    </td>
                    <td>{{ itemCount(item.copied_fields) }}</td>
                    <td>{{ itemCount(item.legacy_fields) }}</td>
                    <td>{{ itemCount(item.missing_required_fields) }}</td>
                    <td>{{ itemCount(item.incompatible_fields) }}</td>
                    <td>{{ itemCount(item.warnings) }}</td>
                    <td>
                      <UiButton size="sm" variant="secondary" @click="toggleRowDetails(item.entity_id)">
                        {{
                          isRowExpanded(item.entity_id)
                            ? $t('lowCode.batchMigrationHideDetails')
                            : $t('lowCode.batchMigrationShowDetails')
                        }}
                      </UiButton>
                    </td>
                  </tr>
                  <tr v-if="isRowExpanded(item.entity_id)" class="batch-wizard__details-row">
                    <td colspan="8">
                      <div class="batch-wizard__details">
                        <section>
                          <h4>{{ $t('lowCode.migrationCopiedFields') }}</h4>
                          <p>{{ item.copied_fields?.join(', ') || $t('lowCode.migrationNone') }}</p>
                        </section>
                        <section>
                          <h4>{{ $t('lowCode.migrationLegacyFields') }}</h4>
                          <p>{{ item.legacy_fields?.join(', ') || $t('lowCode.migrationNone') }}</p>
                        </section>
                        <section>
                          <h4>{{ $t('lowCode.migrationMissingRequiredFields') }}</h4>
                          <p>{{ item.missing_required_fields?.join(', ') || $t('lowCode.migrationNone') }}</p>
                        </section>
                        <section>
                          <h4>{{ $t('lowCode.migrationIncompatibleFields') }}</h4>
                          <ul v-if="item.incompatible_fields?.length">
                            <li v-for="field in item.incompatible_fields" :key="field.field_code">
                              <code>{{ field.field_code }}</code> — {{ field.reason }}
                            </li>
                          </ul>
                          <p v-else>{{ $t('lowCode.migrationNone') }}</p>
                        </section>
                        <section>
                          <h4>{{ $t('lowCode.migrationWarningsSection') }}</h4>
                          <p>{{ item.warnings?.join(', ') || $t('lowCode.migrationNone') }}</p>
                        </section>
                        <details class="batch-wizard__raw">
                          <summary>{{ $t('lowCode.batchMigrationRawItem') }}</summary>
                          <pre>{{ formatJsonValue(renderItemDetails(item)) }}</pre>
                        </details>
                      </div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>

          <div v-if="step === 3" class="batch-wizard__confirm">
            <label v-if="hasWarnings" class="batch-wizard__checkbox">
              <input v-model="warningsConfirmed" type="checkbox" />
              <span>{{ $t('lowCode.batchMigrationWarningsConfirm') }}</span>
            </label>
            <label v-if="hasBlocked" class="batch-wizard__checkbox">
              <input v-model="skipBlocked" type="checkbox" />
              <span>{{ $t('lowCode.batchMigrationSkipBlocked') }}</span>
            </label>
            <p v-if="allBlocked" class="batch-wizard__blocked">
              {{ $t('lowCode.batchMigrationAllBlocked') }}
            </p>
            <p v-else-if="hasBlocked && !skipBlocked" class="batch-wizard__blocked">
              {{ $t('lowCode.batchMigrationSkipBlockedHint') }}
            </p>
          </div>

          <p v-if="errorMessage" class="batch-wizard__error-inline">{{ errorMessage }}</p>
        </template>

        <div v-else-if="step === 4 && executeResult" class="batch-wizard__panel">
          <div class="batch-wizard__success">
            <p>{{ executeStatusLabel }}</p>
            <p class="batch-wizard__muted">
              {{ $t('lowCode.batchMigrationBatchId') }}:
              <code>{{ executeResult.batch_id || '—' }}</code>
            </p>
          </div>

          <dl class="batch-wizard__summary">
            <div>
              <dt>{{ $t('lowCode.batchMigrationTotal') }}</dt>
              <dd>{{ executeSummary.total }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationMigrated') }}</dt>
              <dd>{{ executeSummary.migrated }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationSkipped') }}</dt>
              <dd>{{ executeSummary.skipped }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationBlocked') }}</dt>
              <dd>{{ executeSummary.blocked }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationFailedCount') }}</dt>
              <dd>{{ executeSummary.failed }}</dd>
            </div>
            <div>
              <dt>{{ $t('lowCode.batchMigrationWarnings') }}</dt>
              <dd>{{ executeSummary.warnings }}</dd>
            </div>
          </dl>

          <div class="batch-wizard__table-wrap">
            <table class="batch-wizard__table">
              <thead>
                <tr>
                  <th>{{ $t('lowCode.entityId') }}</th>
                  <th>{{ $t('lowCode.migrationStatus') }}</th>
                  <th>{{ $t('lowCode.batchMigrationPreviewStatus') }}</th>
                  <th>{{ $t('lowCode.migrationCopiedFields') }}</th>
                  <th>{{ $t('lowCode.batchMigrationReason') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in executeItems" :key="item.entity_id">
                  <td class="batch-wizard__mono">{{ item.entity_id }}</td>
                  <td>
                    <UiBadge :status="item.status || '—'" :tone="statusBadgeTone(item.status || '')" />
                  </td>
                  <td>{{ item.preview_status || '—' }}</td>
                  <td>{{ item.copied_fields?.join(', ') || $t('lowCode.migrationNone') }}</td>
                  <td>{{ item.reason || '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <NuxtLink :to="batchAuditLink" class="batch-wizard__audit-link">
            {{ $t('lowCode.batchMigrationViewBatchAudit') }}
          </NuxtLink>
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" :disabled="phase === 'executing'" @click="handleClose">
          {{ $t('lowCode.migrationClose') }}
        </UiButton>

        <template v-if="step === 1">
          <UiButton
            :disabled="!canProceedFromSelect || isRequestInFlight"
            :loading="phase === 'loadingPreview'"
            @click="loadPreview"
          >
            {{ $t('lowCode.batchMigrationPreviewAction') }}
          </UiButton>
        </template>

        <template v-else-if="step === 2 && phase === 'previewLoaded'">
          <UiButton variant="secondary" :disabled="isRequestInFlight" @click="backToSelect">
            {{ $t('common.back') }}
          </UiButton>
          <UiButton
            variant="secondary"
            :disabled="isRequestInFlight"
            :loading="phase === 'loadingPreview'"
            @click="loadPreview"
          >
            {{ $t('lowCode.batchMigrationRetryPreview') }}
          </UiButton>
          <UiButton :disabled="isRequestInFlight" @click="goToConfirm">
            {{ $t('lowCode.batchMigrationContinueConfirm') }}
          </UiButton>
        </template>

        <template v-else-if="step === 3">
          <UiButton variant="secondary" :disabled="phase === 'executing'" @click="step = 2">
            {{ $t('common.back') }}
          </UiButton>
          <UiButton
            :loading="phase === 'executing'"
            :disabled="!canExecute || phase === 'executing'"
            @click="executeBatch"
          >
            {{ executeButtonLabel }}
          </UiButton>
        </template>
      </template>
    </UiModal>
  </div>
</template>

<style scoped>
.batch-wizard-root :deep(.ui-modal__dialog) {
  width: min(960px, 100%);
}

.batch-wizard {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.batch-wizard__steps {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 0.8125rem;
}

.batch-wizard__steps li {
  color: var(--color-text-muted);
}

.batch-wizard__step--active {
  color: var(--color-primary);
  font-weight: 600;
}

.batch-wizard__step--done {
  color: var(--color-text);
}

.batch-wizard__context {
  margin: 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.batch-wizard__label {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
}

.batch-wizard__hint {
  font-weight: 400;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.batch-wizard__textarea {
  width: 100%;
  min-height: 160px;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
  resize: vertical;
}

.batch-wizard__inline-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
}

.batch-wizard__count {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.batch-wizard__invalid {
  display: inline-block;
  margin-right: 0.5rem;
  word-break: break-all;
}

.batch-wizard__muted {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.batch-wizard__error,
.batch-wizard__error-inline,
.batch-wizard__error-panel,
.batch-wizard__blocked {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  font-size: 0.875rem;
}

.batch-wizard__error,
.batch-wizard__error-inline,
.batch-wizard__error-panel {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.batch-wizard__blocked {
  background: #fffbeb;
  border: 1px solid #fde68a;
  color: #92400e;
}

.batch-wizard__success {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  color: #065f46;
  font-size: 0.875rem;
}

.batch-wizard__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 0.75rem;
  margin: 0;
}

.batch-wizard__summary div {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.batch-wizard__summary dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.batch-wizard__summary dd {
  margin: 0;
  font-weight: 600;
}

.batch-wizard__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.batch-wizard__filter {
  padding: 0.25rem 0.625rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  font-size: 0.8125rem;
  cursor: pointer;
}

.batch-wizard__filter--active {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.batch-wizard__table-wrap {
  overflow-x: auto;
}

.batch-wizard__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.batch-wizard__table th,
.batch-wizard__table td {
  padding: 0.5rem 0.625rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.batch-wizard__table th {
  font-weight: 600;
  white-space: nowrap;
}

.batch-wizard__mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
  max-width: 220px;
}

.batch-wizard__details-row td {
  background: var(--color-surface-muted, #f8fafc);
}

.batch-wizard__details {
  display: grid;
  gap: 0.75rem;
}

.batch-wizard__details h4 {
  margin: 0 0 0.25rem;
  font-size: 0.8125rem;
}

.batch-wizard__details p,
.batch-wizard__details ul {
  margin: 0;
  font-size: 0.8125rem;
  word-break: break-word;
}

.batch-wizard__raw summary {
  cursor: pointer;
  color: var(--color-primary);
  font-size: 0.8125rem;
}

.batch-wizard__raw pre {
  margin: 0.5rem 0 0;
  max-height: 180px;
  overflow: auto;
  padding: 0.5rem;
  background: var(--color-surface);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
}

.batch-wizard__confirm {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.batch-wizard__checkbox {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.875rem;
}

.batch-wizard__audit-link {
  color: var(--color-primary);
  font-weight: 500;
  text-decoration: none;
}

.batch-wizard__audit-link:hover {
  text-decoration: underline;
}
</style>
