<script setup lang="ts">
import { RFX_QUESTIONNAIRE_API_KEY, type RfxQuestionnaireApi } from '~/composables/useRfxQuestionnaireApi'
import { useRfxTemplateQuestionnaireApi } from '~/composables/useRfxTemplateQuestionnaireApi'
import type { RfxTemplateDetailResponse } from '~/types/rfx-template'
import { isTemplateVersionEditable, isTemplateVersionReadOnly } from '~/types/rfx-template'
import { resolveI18nMapValue } from '~/utils/rfxTemplateI18n'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { IdempotentOperation } from '~/utils/idempotentOperation'
import { canForkTemplateDraft } from '~/utils/rfxTemplateLifecycle'
import { localTemplatePublishPrecheck, parseTemplatePublish422 } from '~/utils/rfxTemplatePublishErrors'
import type { RfxPublishReadinessResult } from '~/types/rfx-questionnaire'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const { t, locale } = useI18n()
const { pushToast } = useToast()
const { getTemplate, publishTemplateVersion, forkTemplateDraft, archiveTemplate } = useRfxTemplateApi()

const templateId = computed(() => String(route.params.id))
const detail = ref<RfxTemplateDetailResponse | null>(null)
const loading = ref(true)
const publishSummary = ref('')
const publishing = ref(false)
const selectedQuestionId = ref<string | null>(null)
const showArchiveConfirm = ref(false)
const publishReadiness = ref<RfxPublishReadinessResult | null>(null)
const readOnlyQuestionnaire = ref(false)

const questionnaireApi = useRfxTemplateQuestionnaireApi(templateId, detail)
provide(RFX_QUESTIONNAIRE_API_KEY, questionnaireApi as unknown as RfxQuestionnaireApi)

const publishOp = new IdempotentOperation('tpl-pub')
const forkOp = new IdempotentOperation('tpl-fork')

const displayName = computed(() =>
  detail.value ? resolveI18nMapValue(detail.value.template.name_i18n, locale.value) : '',
)

const draftVersion = computed(() => detail.value?.draft_version ?? null)
const publishedVersion = computed(() => detail.value?.published_version ?? null)
const canEdit = computed(() =>
  draftVersion.value ? isTemplateVersionEditable(draftVersion.value.status) : false,
)
const forkGate = computed(() => canForkTemplateDraft(detail.value))
const isArchived = computed(() => detail.value?.template.status === 'ARCHIVED')

async function loadDetail() {
  loading.value = true
  readOnlyQuestionnaire.value = false
  try {
    detail.value = await getTemplate(templateId.value)
    if (canEdit.value) {
      await questionnaireApi.loadStudio()
      publishReadiness.value = localTemplatePublishPrecheck(questionnaireApi.studio.value?.sections.length ?? 0)
    } else if (publishedVersion.value && isTemplateVersionReadOnly(publishedVersion.value.status)) {
      readOnlyQuestionnaire.value = true
    }
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
}

onMounted(() => void loadDetail())

async function handlePublish() {
  if (!detail.value?.draft_version || !publishSummary.value.trim()) return
  if (publishOp.isSubmitting()) return
  publishing.value = true
  publishReadiness.value = null
  try {
    await questionnaireApi.flushPendingPatches()
    await loadDetail()
    const body = {
      expected_template_version: detail.value!.template.version,
      expected_draft_version: detail.value!.draft_version!.version,
      change_summary: publishSummary.value.trim(),
    }
    await publishOp.execute(body, (key, payload) =>
      publishTemplateVersion(templateId.value, payload, key),
    )
    pushToast('success', t('rfx.templates.publish.success'))
    publishSummary.value = ''
    publishOp.reset()
    await loadDetail()
  } catch (e) {
    const parsed = parseTemplatePublish422(e)
    if (parsed) publishReadiness.value = parsed
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    publishing.value = false
  }
}

async function handleFork() {
  if (!forkGate.value.allowed || forkOp.isSubmitting()) return
  try {
    await forkOp.execute({}, (key) => forkTemplateDraft(templateId.value, key))
    pushToast('success', t('rfx.templates.fork.success'))
    forkOp.reset()
    await loadDetail()
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  }
}

async function handleArchive() {
  if (!detail.value) return
  try {
    await archiveTemplate(templateId.value)
    showArchiveConfirm.value = false
    pushToast('success', t('rfx.templates.archive.success'))
    await loadDetail()
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  }
}
</script>

<template>
  <div class="page">
    <header class="page__header">
      <div>
        <NuxtLink to="/rfx/templates">{{ $t('rfx.templates.backToLibrary') }}</NuxtLink>
        <h1>{{ displayName }}</h1>
        <p v-if="detail">
          <RfxTemplateStatusBadge :aggregate-status="detail.template.status" />
          <RfxTemplateStatusBadge v-if="draftVersion" :version-status="draftVersion.status" class="ml-2" />
          <RfxTemplateStatusBadge v-else-if="publishedVersion" :version-status="publishedVersion.status" class="ml-2" />
        </p>
      </div>
      <div v-if="!isArchived" class="page__actions">
        <NuxtLink :to="`/rfx/templates/${templateId}/versions`" class="btn btn--secondary">
          {{ $t('rfx.templates.versionHistory') }}
        </NuxtLink>
        <button
          v-if="forkGate.allowed"
          type="button"
          class="btn btn--secondary"
          :disabled="forkOp.isSubmitting()"
          @click="handleFork"
        >
          {{ $t('rfx.templates.fork.createDraft') }}
        </button>
        <p v-else-if="forkGate.reasonKey" class="fork-hint">{{ $t(forkGate.reasonKey) }}</p>
        <button
          v-if="detail?.template.status === 'ACTIVE'"
          type="button"
          class="btn btn--secondary"
          @click="showArchiveConfirm = true"
        >
          {{ $t('rfx.templates.archive.action') }}
        </button>
      </div>
    </header>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else-if="isArchived" class="read-only-banner">{{ $t('rfx.templates.archivedBanner') }}</p>
    <p v-else-if="readOnlyQuestionnaire" class="read-only-banner">{{ $t('rfx.templates.readOnly') }}</p>

    <section v-if="canEdit && questionnaireApi.studio.value" class="editor">
      <RfxStudioRfxQuestionnaireBuilder
        :sections="questionnaireApi.studio.value.sections"
        v-model:selected-question-id="selectedQuestionId"
      />
      <RfxStudioRfxPublishReadinessPanel v-if="publishReadiness" :result="publishReadiness" />
      <div class="publish-box">
        <label>
          {{ $t('rfx.templates.publish.changeSummary') }}
          <textarea v-model="publishSummary" rows="2" />
        </label>
        <button
          type="button"
          class="btn btn--primary"
          :disabled="publishing || publishOp.isSubmitting() || !publishSummary.trim()"
          @click="handlePublish"
        >
          {{ $t('rfx.templates.publish.action') }}
        </button>
      </div>
    </section>

    <section v-else-if="readOnlyQuestionnaire && publishedVersion" class="readonly-meta">
      <p class="backend-gap">{{ $t('rfx.templates.versionDetail.graphGap') }}</p>
      <dl class="meta-list">
        <div><dt>{{ $t('rfx.versions.columns.number') }}</dt><dd>v{{ publishedVersion.version_number }}</dd></div>
        <div><dt>{{ $t('rfx.versions.columns.summary') }}</dt><dd>{{ publishedVersion.change_summary || '—' }}</dd></div>
      </dl>
      <NuxtLink :to="`/rfx/templates/${templateId}/versions/${publishedVersion.id}`" class="btn btn--link">
        {{ $t('rfx.templates.versionDetail.open') }}
      </NuxtLink>
    </section>

    <div v-if="showArchiveConfirm" class="modal-backdrop" role="dialog" aria-modal="true" @click.self="showArchiveConfirm = false">
      <div class="confirm-modal">
        <h2>{{ $t('rfx.templates.archive.confirmTitle') }}</h2>
        <p>{{ $t('rfx.templates.archive.confirmBody') }}</p>
        <div class="confirm-modal__actions">
          <button type="button" class="btn btn--secondary" @click="showArchiveConfirm = false">{{ $t('common.cancel') }}</button>
          <button type="button" class="btn btn--primary" @click="handleArchive">{{ $t('rfx.templates.archive.confirm') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.page__header { display: flex; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.page__actions { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: flex-start; flex-direction: column; }
.read-only-banner { padding: 0.75rem 1rem; background: #f3f4f6; border-radius: var(--radius-md); }
.publish-box { margin-top: 1rem; display: flex; flex-direction: column; gap: 0.5rem; max-width: 32rem; }
.fork-hint { font-size: 0.875rem; color: var(--color-text-muted); margin: 0; }
.backend-gap { color: #b45309; }
.meta-list { display: grid; gap: 0.5rem; font-size: 0.875rem; }
.meta-list div { display: grid; grid-template-columns: 10rem 1fr; gap: 0.5rem; }
.meta-list dt { color: var(--color-text-muted); }
.ml-2 { margin-left: 0.5rem; }
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: grid; place-items: center; z-index: 1000; }
.confirm-modal { background: #fff; padding: 1.5rem; border-radius: var(--radius-lg); max-width: 28rem; display: flex; flex-direction: column; gap: 0.75rem; }
.confirm-modal__actions { display: flex; justify-content: flex-end; gap: 0.5rem; }
</style>
