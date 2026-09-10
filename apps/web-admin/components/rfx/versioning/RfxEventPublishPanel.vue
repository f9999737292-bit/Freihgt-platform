<script setup lang="ts">
import type { RfxPublishReadinessResult } from '~/types/rfx-questionnaire'
import type {
  RfxChangeImpactAnalysisResponse,
  RfxVersionRecord,
} from '~/types/rfx-version-lifecycle'
import { ApiError } from '~/composables/useApi'
import { useInjectedRfxQuestionnaireApi } from '~/composables/useRfxQuestionnaireApi'
import { useRfxVersionLifecycleApi } from '~/composables/useRfxVersionLifecycleApi'
import { IdempotentOperation } from '~/utils/idempotentOperation'
import {
  buildEventPublishPayload,
  shouldRequireImpactPreview,
} from '~/utils/rfxEventPublishOrchestration'
import { formatRfxApiError, resolveRfxConflictDetailKey } from '~/utils/rfxApiError'

const props = defineProps<{
  eventId: string
  expectedEventVersion: number
  expectedDraftVersion: number
  draftVersionId: string
}>()

const emit = defineEmits<{ published: [] }>()

const { t } = useI18n()
const { pushToast } = useToast()
const questionnaireApi = useInjectedRfxQuestionnaireApi()
const lifecycleApi = useRfxVersionLifecycleApi(computed(() => props.eventId))

const versions = ref<RfxVersionRecord[]>([])
const versionsState = ref<'loading' | 'loaded' | 'error'>('loading')
const changeSummary = ref('')
const impactAnalysis = ref<RfxChangeImpactAnalysisResponse | null>(null)
const impactLoading = ref(false)
const impactError = ref('')
const publishError = ref('')
const serverReadiness = ref<RfxPublishReadinessResult | null>(null)
const readinessLoading = ref(false)
const awaitingImpactConfirm = ref(false)
const autosaveBlocked = ref(false)

const publishOp = new IdempotentOperation('evt-pub')

const republishRequired = computed(() =>
  versionsState.value === 'loaded' && shouldRequireImpactPreview(versions.value),
)
const publishBlocked = computed(() =>
  versionsState.value !== 'loaded' || autosaveBlocked.value || readinessLoading.value,
)
const serverReadinessBlocksPublish = computed(() =>
  serverReadiness.value != null && !serverReadiness.value.ready,
)

async function loadVersions() {
  versionsState.value = 'loading'
  try {
    const data = await lifecycleApi.listVersions()
    versions.value = data.versions ?? []
    versionsState.value = 'loaded'
  } catch {
    versions.value = []
    versionsState.value = 'error'
  }
}

onMounted(() => void loadVersions())

async function flushAutosaveOrBlock(): Promise<boolean> {
  autosaveBlocked.value = false
  try {
    await questionnaireApi.flushPendingPatches()
    return true
  } catch {
    autosaveBlocked.value = true
    pushToast('error', t('rfx.studio.autosaveError'))
    return false
  }
}

async function refreshServerReadiness(): Promise<boolean> {
  readinessLoading.value = true
  serverReadiness.value = null
  try {
    serverReadiness.value = await questionnaireApi.validatePublish()
    return Boolean(serverReadiness.value?.ready)
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
    return false
  } finally {
    readinessLoading.value = false
  }
}

function clearImpactAnalysis() {
  impactAnalysis.value = null
  awaitingImpactConfirm.value = false
}

async function runImpactPreview() {
  impactLoading.value = true
  impactError.value = ''
  clearImpactAnalysis()
  try {
    impactAnalysis.value = await lifecycleApi.previewChangeImpact({
      candidate_version_id: props.draftVersionId,
    })
    if (impactAnalysis.value.impact_classes.some((c) => c !== 'NON_MATERIAL')) {
      awaitingImpactConfirm.value = true
    }
  } catch (e) {
    impactError.value = formatRfxApiError(e, t)
  } finally {
    impactLoading.value = false
  }
}

function handleImpactPublishError(e: unknown): boolean {
  if (!(e instanceof ApiError)) return false
  if (e.status === 409) {
    const key = resolveRfxConflictDetailKey(e.code)
    publishError.value = key ? t(key) : formatRfxApiError(e, t)
    if (e.code === 'STALE_DIFF') {
      clearImpactAnalysis()
      publishError.value = `${publishError.value} ${t('rfx.changeImpact.repreviewRequired')}`
    }
    return true
  }
  if (e.status === 422 && e.code === 'IMPACT_ANALYSIS_EXPIRED') {
    clearImpactAnalysis()
    publishError.value = t('rfx.errors.impactAnalysisExpired')
    return true
  }
  return false
}

async function runPrePublishGates(): Promise<boolean> {
  publishError.value = ''
  if (!changeSummary.value.trim()) {
    pushToast('error', t('rfx.templates.publish.changeSummary'))
    return false
  }
  if (versionsState.value === 'error') {
    pushToast('error', t('rfx.versions.loadFailed'))
    return false
  }
  if (!(await flushAutosaveOrBlock())) return false
  if (!(await refreshServerReadiness())) {
    pushToast('error', t('rfx.studio.readyFail'))
    return false
  }
  return true
}

async function executePublish() {
  if (publishOp.isSubmitting()) return
  publishError.value = ''
  const body = buildEventPublishPayload({
    expectedEventVersion: props.expectedEventVersion,
    expectedDraftVersion: props.expectedDraftVersion,
    changeSummary: changeSummary.value,
    impact: republishRequired.value ? impactAnalysis.value : null,
  })
  try {
    await publishOp.execute(body, (key, payload) => lifecycleApi.publishQuestionnaire(payload, key))
    pushToast('success', t('rfx.studio.publishSuccess'))
    emit('published')
  } catch (e) {
    if (!handleImpactPublishError(e)) {
      publishError.value = formatRfxApiError(e, t)
    }
  }
}

async function handlePublishClick() {
  if (publishBlocked.value || publishOp.isSubmitting()) return
  if (!(await runPrePublishGates())) return
  if (republishRequired.value && !impactAnalysis.value) {
    await runImpactPreview()
    if (!impactAnalysis.value) return
    if (awaitingImpactConfirm.value) return
  }
  await executePublish()
}

async function confirmImpactAndPublish() {
  if (publishBlocked.value || publishOp.isSubmitting()) return
  if (!impactAnalysis.value) return
  awaitingImpactConfirm.value = false
  if (!(await runPrePublishGates())) {
    awaitingImpactConfirm.value = true
    return
  }
  await executePublish()
}

function retryPreview() {
  clearImpactAnalysis()
  void runImpactPreview()
}
</script>

<template>
  <UiCard class="publish-panel">
    <h2>{{ $t('rfx.studio.validationTitle') }}</h2>

    <p v-if="versionsState === 'loading'">{{ $t('rfx.versions.loading') }}</p>
    <div v-else-if="versionsState === 'error'" class="publish-panel__error-row">
      <p>{{ $t('rfx.versions.loadFailed') }}</p>
      <button type="button" class="btn btn--link" @click="loadVersions">{{ $t('common.retry') }}</button>
    </div>

    <p v-if="autosaveBlocked" class="publish-panel__error">{{ $t('rfx.studio.autosaveError') }}</p>
    <p v-if="readinessLoading">{{ $t('common.loading') }}</p>

    <RfxStudioRfxPublishReadinessPanel v-else-if="serverReadiness" :result="serverReadiness" />

    <label class="publish-panel__label">
      {{ $t('rfx.templates.publish.changeSummary') }}
      <textarea v-model="changeSummary" rows="2" />
    </label>

    <p v-if="republishRequired" class="publish-panel__hint">
      {{ $t('rfx.changeImpact.republishHint') }}
    </p>

    <RfxChangeImpactPanel
      :analysis="impactAnalysis"
      :loading="impactLoading"
      @confirm="confirmImpactAndPublish"
    />

    <p v-if="impactError" class="publish-panel__error">{{ impactError }}</p>
    <p v-if="publishError" class="publish-panel__error">{{ publishError }}</p>

    <div class="publish-panel__actions">
      <button
        v-if="republishRequired && !impactAnalysis"
        type="button"
        class="btn btn--secondary"
        :disabled="publishBlocked || impactLoading || serverReadinessBlocksPublish"
        @click="runImpactPreview"
      >
        {{ $t('rfx.changeImpact.preview') }}
      </button>
      <button
        v-if="impactError"
        type="button"
        class="btn btn--link"
        @click="retryPreview"
      >
        {{ $t('common.retry') }}
      </button>
      <button
        type="button"
        class="btn btn--primary"
        :disabled="publishBlocked || publishOp.isSubmitting() || serverReadinessBlocksPublish || (republishRequired && awaitingImpactConfirm)"
        @click="handlePublishClick"
      >
        {{ republishRequired ? $t('rfx.changeImpact.republish') : $t('rfx.studio.publish') }}
      </button>
    </div>
  </UiCard>
</template>

<style scoped>
.publish-panel { padding: 1rem; display: flex; flex-direction: column; gap: 1rem; }
.publish-panel h2 { margin: 0; font-size: 1.125rem; }
.publish-panel__label { display: flex; flex-direction: column; gap: 0.375rem; font-size: 0.875rem; }
.publish-panel__hint { font-size: 0.875rem; color: var(--color-text-muted); margin: 0; }
.publish-panel__error { color: var(--color-danger, #b91c1c); margin: 0; }
.publish-panel__error-row { display: flex; align-items: center; gap: 0.75rem; }
.publish-panel__actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
</style>
