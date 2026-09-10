<script setup lang="ts">
import { RFX_QUESTIONNAIRE_API_KEY, type RfxQuestionnaireApi } from '~/composables/useRfxQuestionnaireApi'
import { useRfxTemplateQuestionnaireApi } from '~/composables/useRfxTemplateQuestionnaireApi'
import type { RfxTemplateDetailResponse } from '~/types/rfx-template'
import { isTemplateVersionEditable } from '~/types/rfx-template'
import { resolveI18nMapValue } from '~/utils/rfxTemplateI18n'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { createIdempotencyKey } from '~/utils/idempotencyKey'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const { pushToast } = useToast()
const { getTemplate, updateTemplate, publishTemplateVersion, forkTemplateDraft, archiveTemplate } = useRfxTemplateApi()

const templateId = computed(() => String(route.params.id))
const detail = ref<RfxTemplateDetailResponse | null>(null)
const loading = ref(true)
const readOnly = ref(false)
const publishSummary = ref('')
const publishing = ref(false)
const selectedQuestionId = ref<string | null>(null)

const questionnaireApi = useRfxTemplateQuestionnaireApi(templateId, detail)
provide(RFX_QUESTIONNAIRE_API_KEY, questionnaireApi as unknown as RfxQuestionnaireApi)

const displayName = computed(() =>
  detail.value ? resolveI18nMapValue(detail.value.template.name_i18n, locale.value) : '',
)

const draftVersion = computed(() => detail.value?.draft_version ?? null)
const canEdit = computed(() =>
  draftVersion.value ? isTemplateVersionEditable(draftVersion.value.status) : false,
)

async function loadDetail() {
  loading.value = true
  try {
    detail.value = await getTemplate(templateId.value)
    readOnly.value = !canEdit.value || detail.value.template.status === 'ARCHIVED'
    if (canEdit.value) {
      await questionnaireApi.loadStudio()
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
  publishing.value = true
  try {
    await publishTemplateVersion(templateId.value, {
      expected_template_version: detail.value.template.version,
      expected_draft_version: detail.value.draft_version.version,
      change_summary: publishSummary.value.trim(),
    }, createIdempotencyKey('tpl-pub'))
    pushToast('success', t('rfx.templates.publish.success'))
    publishSummary.value = ''
    await loadDetail()
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    publishing.value = false
  }
}

async function handleFork() {
  try {
    await forkTemplateDraft(templateId.value, createIdempotencyKey('tpl-fork'))
    pushToast('success', t('rfx.templates.fork.success'))
    await loadDetail()
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  }
}

async function handleArchive() {
  if (!detail.value) return
  try {
    await archiveTemplate(templateId.value)
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
          <RfxTemplateStatusBadge
            v-if="draftVersion"
            :version-status="draftVersion.status"
            class="ml-2"
          />
        </p>
      </div>
      <div class="page__actions">
        <NuxtLink :to="`/rfx/templates/${templateId}/versions`" class="btn btn--secondary">
          {{ $t('rfx.templates.versionHistory') }}
        </NuxtLink>
        <button v-if="canEdit" type="button" class="btn btn--secondary" @click="handleFork">
          {{ $t('rfx.templates.fork') }}
        </button>
        <button
          v-if="detail?.template.status === 'ACTIVE'"
          type="button"
          class="btn btn--secondary"
          @click="handleArchive"
        >
          {{ $t('rfx.templates.archive.action') }}
        </button>
      </div>
    </header>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else-if="readOnly" class="read-only-banner">{{ $t('rfx.templates.readOnly') }}</p>

    <section v-if="canEdit && questionnaireApi.studio.value" class="editor">
      <RfxStudioRfxQuestionnaireBuilder
        :sections="questionnaireApi.studio.value.sections"
        v-model:selected-question-id="selectedQuestionId"
      />
      <div class="publish-box">
        <label>
          {{ $t('rfx.templates.publish.changeSummary') }}
          <textarea v-model="publishSummary" rows="2" />
        </label>
        <button
          type="button"
          class="btn btn--primary"
          :disabled="publishing || !publishSummary.trim()"
          @click="handlePublish"
        >
          {{ $t('rfx.templates.publish.action') }}
        </button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.page__header { display: flex; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.page__actions { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: flex-start; }
.read-only-banner { padding: 0.75rem 1rem; background: #f3f4f6; border-radius: var(--radius-md); }
.publish-box { margin-top: 1rem; display: flex; flex-direction: column; gap: 0.5rem; max-width: 32rem; }
.ml-2 { margin-left: 0.5rem; }
</style>
