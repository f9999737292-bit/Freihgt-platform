<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import {
  formatJsonValue,
  normalizeMigrationPreviewResponse,
  type MigrationPreviewItem,
  type MigrationPreviewResponse,
  type MigrationPreviewStatus,
} from '~/types/lowCode'

const props = defineProps<{
  open: boolean
  entityType: string
  entityId: string
  templateCode: string
}>()

const emit = defineEmits<{
  close: []
  migrated: []
}>()

const {
  previewMigrationToActive,
  migrateCustomFieldValuesToActive,
  getMigrationErrorMessage,
  isApiUnavailableError,
} = useLowCodeApi()
const { t } = useI18n()

type ModalPhase = 'loading' | 'preview' | 'migrated' | 'error'

const phase = ref<ModalPhase>('loading')
const executing = ref(false)
const preview = ref<MigrationPreviewResponse | null>(null)
const errorMessage = ref('')
const warningsConfirmed = ref(false)
const executeResultStatus = ref('')

const previewItem = computed<MigrationPreviewItem | null>(() => preview.value?.items?.[0] ?? null)

const previewSummary = computed(() => preview.value?.summary ?? {
  entities_checked: 0,
  safe_to_migrate: 0,
  warnings: 0,
  blocked: 0,
})

const entityStatus = computed<MigrationPreviewStatus | null>(() => previewItem.value?.status ?? null)

const canExecute = computed(() => {
  if (!previewItem.value) return false
  if (entityStatus.value === 'SAFE') return true
  if (entityStatus.value === 'WARNING') return warningsConfirmed.value
  return false
})

const statusBadgeTone = computed(() => {
  switch (entityStatus.value) {
    case 'SAFE':
      return 'success'
    case 'WARNING':
      return 'warning'
    case 'BLOCKED':
      return 'danger'
    default:
      return 'neutral'
  }
})

const statusLabel = computed(() => {
  switch (entityStatus.value) {
    case 'SAFE':
      return t('lowCode.migrationStatusSafe')
    case 'WARNING':
      return t('lowCode.migrationStatusWarning')
    case 'BLOCKED':
      return t('lowCode.migrationStatusBlocked')
    default:
      return '—'
  }
})

function resetState() {
  phase.value = 'loading'
  executing.value = false
  preview.value = null
  errorMessage.value = ''
  warningsConfirmed.value = false
  executeResultStatus.value = ''
}

function formatFieldList(fields: string[] | undefined): string {
  return fields?.length ? fields.join(', ') : t('lowCode.migrationNone')
}

function fieldList(fields: string[] | undefined): string[] {
  return Array.isArray(fields) ? fields : []
}

function applyPreviewData(data: MigrationPreviewResponse | null) {
  preview.value = data
  if (!data || !data.items.length) {
    phase.value = 'error'
    errorMessage.value = t('lowCode.migrationPreviewEmpty')
    return
  }
  phase.value = 'preview'
}

async function loadPreview() {
  resetState()
  phase.value = 'loading'
  try {
    const data = await previewMigrationToActive({
      entity_type: props.entityType,
      template_code: props.templateCode,
      entity_ids: [props.entityId],
    })
    applyPreviewData(data)
  } catch (error) {
    if (error instanceof ApiError && error.preview) {
      const normalized = normalizeMigrationPreviewResponse(error.preview)
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

async function executeMigration() {
  if (!canExecute.value || !previewItem.value) return

  executing.value = true
  errorMessage.value = ''
  try {
    const result = await migrateCustomFieldValuesToActive({
      entity_type: props.entityType,
      template_code: props.templateCode,
      entity_id: props.entityId,
      allow_warnings: entityStatus.value === 'WARNING',
    })
    executeResultStatus.value = result.status
    phase.value = 'migrated'
    emit('migrated')
  } catch (error) {
    if (error instanceof ApiError && error.preview) {
      const normalized = normalizeMigrationPreviewResponse(error.preview)
      if (normalized) {
        preview.value = normalized
      }
    }
    errorMessage.value = getMigrationErrorMessage(error)
  } finally {
    executing.value = false
  }
}

function handleClose() {
  emit('close')
}

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      loadPreview()
    } else {
      resetState()
    }
  },
)
</script>

<template>
  <UiModal
    :open="open"
    :title="$t('lowCode.migrationModalTitle')"
    @close="handleClose"
  >
    <div class="migration-modal">
      <div v-if="phase === 'loading'" class="migration-modal__muted">
        {{ $t('lowCode.migrationLoadingPreview') }}
      </div>

      <div v-else-if="phase === 'error'" class="migration-modal__error">
        <p>{{ errorMessage || $t('lowCode.migrationFailed') }}</p>
        <UiButton size="sm" variant="secondary" @click="loadPreview">
          {{ $t('lowCode.migrationRetry') }}
        </UiButton>
      </div>

      <template v-else-if="preview && previewItem">
        <div v-if="phase === 'migrated'" class="migration-modal__success">
          <p>{{ $t('lowCode.migrationCompleted') }}</p>
          <p v-if="executeResultStatus" class="migration-modal__muted">
            {{ executeResultStatus }}
          </p>
        </div>

        <dl v-if="phase !== 'migrated'" class="migration-modal__summary">
          <div>
            <dt>{{ $t('lowCode.migrationEntitiesChecked') }}</dt>
            <dd>{{ previewSummary.entities_checked }}</dd>
          </div>
          <div>
            <dt>{{ $t('lowCode.migrationStatusSafe') }}</dt>
            <dd>{{ previewSummary.safe_to_migrate }}</dd>
          </div>
          <div>
            <dt>{{ $t('lowCode.migrationStatusWarning') }}</dt>
            <dd>{{ previewSummary.warnings }}</dd>
          </div>
          <div>
            <dt>{{ $t('lowCode.migrationStatusBlocked') }}</dt>
            <dd>{{ previewSummary.blocked }}</dd>
          </div>
        </dl>

        <div v-if="phase !== 'migrated'" class="migration-modal__entity">
          <div class="migration-modal__entity-header">
            <span class="migration-modal__entity-id">{{ previewItem.entity_id }}</span>
            <UiBadge :status="statusLabel" :tone="statusBadgeTone" />
          </div>
          <p class="migration-modal__muted">
            {{ $t('lowCode.code') }}: <code>{{ preview.target_template.code }}</code>
            · {{ $t('lowCode.version') }}: {{ preview.target_template.version }}
          </p>
        </div>

        <div v-if="phase !== 'migrated'" class="migration-modal__sections">
          <section>
            <h4>{{ $t('lowCode.migrationCopiedFields') }}</h4>
            <p>{{ formatFieldList(fieldList(previewItem.copied_fields)) }}</p>
          </section>
          <section>
            <h4>{{ $t('lowCode.migrationLegacyFields') }}</h4>
            <p>{{ formatFieldList(fieldList(previewItem.legacy_fields)) }}</p>
          </section>
          <section>
            <h4>{{ $t('lowCode.migrationMissingRequiredFields') }}</h4>
            <p>{{ formatFieldList(fieldList(previewItem.missing_required_fields)) }}</p>
          </section>
          <section>
            <h4>{{ $t('lowCode.migrationIncompatibleFields') }}</h4>
            <div v-if="!(previewItem.incompatible_fields?.length)" class="migration-modal__muted">
              {{ $t('lowCode.migrationNone') }}
            </div>
            <ul v-else class="migration-modal__incompatible">
              <li v-for="field in previewItem.incompatible_fields" :key="field.field_code">
                <code>{{ field.field_code }}</code> — {{ field.reason }}
              </li>
            </ul>
          </section>
          <section>
            <h4>{{ $t('lowCode.migrationWarningsSection') }}</h4>
            <p v-if="fieldList(previewItem.warnings).length === 0" class="migration-modal__muted">
              {{ $t('lowCode.migrationNoIssues') }}
            </p>
            <ul v-else class="migration-modal__warnings">
              <li v-for="(warning, index) in fieldList(previewItem.warnings)" :key="`${warning}-${index}`">
                {{ warning }}
              </li>
            </ul>
          </section>
        </div>

        <details v-if="preview" class="migration-modal__raw">
          <summary>{{ $t('lowCode.migrationRawPreview') }}</summary>
          <pre>{{ formatJsonValue(preview) }}</pre>
        </details>

        <p v-if="errorMessage && phase !== 'error'" class="migration-modal__error-inline">
          {{ errorMessage }}
        </p>

        <label
          v-if="phase === 'preview' && entityStatus === 'WARNING'"
          class="migration-modal__confirm"
        >
          <input v-model="warningsConfirmed" type="checkbox" />
          <span>{{ $t('lowCode.migrationWarningsConfirm') }}</span>
        </label>

        <p
          v-if="phase === 'preview' && entityStatus === 'BLOCKED'"
          class="migration-modal__blocked"
        >
          {{ $t('lowCode.migrationBlockedMessage') }}
        </p>
      </template>
    </div>

    <template #footer>
      <UiButton variant="secondary" :disabled="executing" @click="handleClose">
        {{ $t('lowCode.migrationClose') }}
      </UiButton>
      <UiButton
        v-if="phase === 'preview' && entityStatus !== 'BLOCKED'"
        :loading="executing"
        :disabled="!canExecute || executing"
        @click="executeMigration"
      >
        {{
          entityStatus === 'WARNING'
            ? $t('lowCode.migrationMigrateWithWarnings')
            : $t('lowCode.migrationMigrate')
        }}
      </UiButton>
      <UiButton
        v-else-if="phase === 'error'"
        variant="secondary"
        @click="loadPreview"
      >
        {{ $t('lowCode.migrationRetry') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.migration-modal {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.migration-modal__muted {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.migration-modal__error,
.migration-modal__error-inline,
.migration-modal__blocked {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  font-size: 0.875rem;
}

.migration-modal__error,
.migration-modal__error-inline {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #991b1b;
}

.migration-modal__blocked {
  background: #fffbeb;
  border: 1px solid #fde68a;
  color: #92400e;
}

.migration-modal__success {
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  color: #065f46;
  font-size: 0.875rem;
}

.migration-modal__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem 1rem;
  margin: 0;
}

.migration-modal__summary div {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}

.migration-modal__summary dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.migration-modal__summary dd {
  margin: 0;
  font-weight: 600;
  font-size: 0.9375rem;
}

.migration-modal__entity-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.migration-modal__entity-id {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
  word-break: break-all;
}

.migration-modal__sections {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.migration-modal__sections h4 {
  margin: 0 0 0.375rem;
  font-size: 0.8125rem;
  font-weight: 600;
}

.migration-modal__sections p {
  margin: 0;
  font-size: 0.875rem;
  word-break: break-word;
}

.migration-modal__incompatible,
.migration-modal__warnings {
  margin: 0;
  padding-left: 1.25rem;
  font-size: 0.875rem;
}

.migration-modal__raw summary {
  cursor: pointer;
  color: var(--color-primary);
  font-size: 0.875rem;
}

.migration-modal__raw pre {
  margin: 0.5rem 0 0;
  max-height: 220px;
  overflow: auto;
  padding: 0.75rem;
  background: var(--color-surface-muted, #f8fafc);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
}

.migration-modal__confirm {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.875rem;
}
</style>
