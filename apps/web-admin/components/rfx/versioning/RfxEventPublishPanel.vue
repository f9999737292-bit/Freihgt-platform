<script setup lang="ts">
import type { RfxPublishReadinessResult } from '~/types/rfx-questionnaire'
import type {
  RfxChangeImpactAnalysisResponse,
  RfxVersionRecord,
} from '~/types/rfx-version-lifecycle'
import { ApiError } from '~/composables/useApi'
import { IdempotentOperation } from '~/utils/idempotentOperation'
import {
  buildEventPublishPayload,
  shouldRequireImpactPreview,
} from '~/utils/rfxEventPublishOrchestration'
import { formatRfxApiError, resolveRfxConflictDetailKey } from '~/utils/rfxApiError'

const props = defineProps<{
  eventId: string
  readiness: RfxPublishReadinessResult | null
  expectedEventVersion: number
  expectedDraftVersion: number
  draftVersionId: string
}>()

const emit = defineEmits<{ published: [] }>()

const { t } = useI18n()
const { pushToast } = useToast()
const lifecycleApi = useRfxVersionLifecycleApi(computed(() => props.eventId))

const versions = ref<RfxVersionRecord[]>([])
const changeSummary = ref('')
const impactAnalysis = ref<RfxChangeImpactAnalysisResponse | null>(null)
const impactLoading = ref(false)
const impactError = ref('')
const publishError = ref('')
const serverReadiness = ref<RfxPublishReadinessResult | null>(null)
const awaitingImpactConfirm = ref(false)

const publishOp = new IdempotentOperation('evt-pub')

const republishRequired = computed(() => shouldRequireImpactPreview(versions.value))
const effectiveReadiness = computed(() => serverReadiness.value ?? props.readiness)

onMounted(async () => {
  try {
    const data = await lifecycleApi.listVersions()
    versions.value = data.versions ?? []
  } catch {
    versions.value = []
  }
})

async function runImpactPreview() {
  impactLoading.value = true
  impactError.value = ''
  impactAnalysis.value = null
  awaitingImpactConfirm.value = false
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

async function handlePublishClick() {
  publishError.value = ''
  if (!changeSummary.value.trim()) {
    pushToast('error', t('rfx.templates.publish.changeSummary'))
    return
  }
  if (!effectiveReadiness.value?.ready) {
    pushToast('error', t('rfx.studio.readyFail'))
    return
  }
  if (republishRequired.value && !impactAnalysis.value) {
    await runImpactPreview()
    if (!impactAnalysis.value) return
    if (awaitingImpactConfirm.value) return
  }
  await executePublish()
}

async function executePublish() {
  if (publishOp.isSubmitting()) return
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
    if (e instanceof ApiError) {
      if (e.status === 409) {
        const key = resolveRfxConflictDetailKey(e.code)
        publishError.value = key ? t(key) : formatRfxApiError(e, t)
        if (e.code === 'STALE_DIFF' || e.code === 'IMPACT_ANALYSIS_EXPIRED') {
          impactAnalysis.value = null
          awaitingImpactConfirm.value = false
        }
        if (e.code === 'IMPACT_ANALYSIS_CONSUMED') {
          // same key replay handled by backend; if new attempt needed user must re-preview
        }
        return
      }
      if (e.status === 422) {
        publishError.value = formatRfxApiError(e, t)
        return
      }
    }
    publishError.value = formatRfxApiError(e, t)
  }
}

function confirmImpactAndPublish() {
  awaitingImpactConfirm.value = false
  void executePublish()
}

function retryPreview() {
  impactAnalysis.value = null
  void runImpactPreview()
}
</script>

<template>
  <UiCard class="publish-panel">
    <h2>{{ $t('rfx.studio.validationTitle') }}</h2>

    <RfxStudioRfxPublishReadinessPanel :result="effectiveReadiness" />

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
        :disabled="impactLoading || !effectiveReadiness?.ready"
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
        :disabled="publishOp.isSubmitting() || !effectiveReadiness?.ready || (republishRequired && awaitingImpactConfirm)"
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
.publish-panel__actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
</style>
