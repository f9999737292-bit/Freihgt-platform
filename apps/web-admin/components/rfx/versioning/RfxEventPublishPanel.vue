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
  extractPublishBaselineFromStudio,
  isImpactPreviewBaselineStale,
  shouldRequireImpactPreview,
  type ImpactPreviewBaseline,
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
const impactPreviewBaseline = ref<ImpactPreviewBaseline | null>(null)
const impactLoading = ref(false)
const impactError = ref('')
const publishError = ref('')
const serverReadiness = ref<RfxPublishReadinessResult | null>(null)
const readinessLoading = ref(false)
const awaitingImpactConfirm = ref(false)
const confirmInFlight = ref(false)
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

async function loadPublishBaseline(): Promise<ImpactPreviewBaseline | null> {
  const studio = await questionnaireApi.getStudio()
  return extractPublishBaselineFromStudio(studio)
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
  impactPreviewBaseline.value = null
  awaitingImpactConfirm.value = false
}

function invalidateImpactPreviewForStaleDraft() {
  clearImpactAnalysis()
  publishError.value = t('rfx.changeImpact.repreviewRequired')
  pushToast('error', t('rfx.changeImpact.repreviewRequired'))
}

async function runImpactPreview() {
  impactLoading.value = true
  impactError.value = ''
  clearImpactAnalysis()
  try {
    if (!(await flushAutosaveOrBlock())) return
    const baseline = await loadPublishBaseline()
    if (!baseline) {
      impactError.value = t('rfx.studio.loadFailed')
      return
    }
    impactAnalysis.value = await lifecycleApi.previewChangeImpact({
      candidate_version_id: baseline.draftVersionId,
    })
    impactPreviewBaseline.value = baseline
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

async function runPrePublishGates(): Promise<ImpactPreviewBaseline | null> {
  publishError.value = ''
  if (!changeSummary.value.trim()) {
    pushToast('error', t('rfx.templates.publish.changeSummary'))
    return null
  }
  if (versionsState.value === 'error') {
    pushToast('error', t('rfx.versions.loadFailed'))
    return null
  }
  if (!(await flushAutosaveOrBlock())) return null
  const baseline = await loadPublishBaseline()
  if (!baseline) {
    pushToast('error', t('rfx.studio.loadFailed'))
    return null
  }
  if (!(await refreshServerReadiness())) {
    pushToast('error', t('rfx.studio.readyFail'))
    return null
  }
  return baseline
}

async function executePublish(baseline: ImpactPreviewBaseline) {
  if (publishOp.isSubmitting()) return
  publishError.value = ''
  const body = buildEventPublishPayload({
    expectedEventVersion: baseline.eventVersion,
    expectedDraftVersion: baseline.draftVersion,
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
  const baseline = await runPrePublishGates()
  if (!baseline) return
  if (republishRequired.value && !impactAnalysis.value) {
    await runImpactPreview()
    if (!impactAnalysis.value) return
    if (awaitingImpactConfirm.value) return
  }
  const publishBaseline = impactPreviewBaseline.value ?? baseline
  await executePublish(publishBaseline)
}

async function confirmImpactAndPublish() {
  if (publishBlocked.value || publishOp.isSubmitting() || confirmInFlight.value) return
  if (!impactAnalysis.value || !impactPreviewBaseline.value) return
  confirmInFlight.value = true
  awaitingImpactConfirm.value = false
  try {
    if (!(await flushAutosaveOrBlock())) {
      awaitingImpactConfirm.value = true
      return
    }
    const currentBaseline = await loadPublishBaseline()
    if (!currentBaseline) {
      awaitingImpactConfirm.value = true
      pushToast('error', t('rfx.studio.loadFailed'))
      return
    }
    if (isImpactPreviewBaselineStale(impactPreviewBaseline.value, currentBaseline, impactAnalysis.value)) {
      invalidateImpactPreviewForStaleDraft()
      return
    }
    if (!(await refreshServerReadiness())) {
      awaitingImpactConfirm.value = true
      pushToast('error', t('rfx.studio.readyFail'))
      return
    }
    await executePublish(currentBaseline)
  } finally {
    confirmInFlight.value = false
  }
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
