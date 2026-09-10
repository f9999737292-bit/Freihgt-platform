<script setup lang="ts">
import type { RfxTemplateDetailResponse, RfxTemplateVersionRecord } from '~/types/rfx-template'
import { isTemplateVersionEditable } from '~/types/rfx-template'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { formatRfxDateTime } from '~/utils/formatRfxDateTime'
import { findTemplateVersionById } from '~/utils/rfxTemplateLifecycle'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const { t, locale } = useI18n()
const { pushToast } = useToast()
const { getTemplate } = useRfxTemplateApi()

const templateId = computed(() => String(route.params.id))
const versionId = computed(() => String(route.params.versionId))
const detail = ref<RfxTemplateDetailResponse | null>(null)
const questionnaireApi = useRfxTemplateQuestionnaireApi(templateId, detail)
const version = ref<RfxTemplateVersionRecord | null>(null)
const loading = ref(true)
const graphGap = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    detail.value = await getTemplate(templateId.value)
    version.value = findTemplateVersionById(detail.value, versionId.value) ?? null
    if (!version.value) return
    if (isTemplateVersionEditable(version.value.status) && detail.value.draft_version?.id === version.value.id) {
      await questionnaireApi.loadQuestionnaire()
    } else {
      graphGap.value = true
    }
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <NuxtLink :to="`/rfx/templates/${templateId}/versions`">{{ $t('rfx.templates.backToVersions') }}</NuxtLink>
    <h1>{{ $t('rfx.templates.versionDetail.title', { number: version?.version_number ?? '—' }) }}</h1>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else-if="!version">{{ $t('rfx.errors.notFound') }}</p>

    <template v-else>
      <p class="read-only-banner">{{ $t('rfx.templates.readOnly') }}</p>
      <dl class="meta-list">
        <div><dt>{{ $t('rfx.versions.columns.status') }}</dt><dd>{{ $t(`rfx.templates.versionStatus.${version.status}`) }}</dd></div>
        <div><dt>{{ $t('rfx.templates.versionDetail.createdAt') }}</dt><dd>{{ formatRfxDateTime(version.created_at, locale) }}</dd></div>
        <div><dt>{{ $t('rfx.versions.columns.publishedAt') }}</dt><dd>{{ formatRfxDateTime(version.published_at, locale) }}</dd></div>
        <div><dt>{{ $t('rfx.versions.columns.summary') }}</dt><dd>{{ version.change_summary || '—' }}</dd></div>
      </dl>

      <p v-if="graphGap" class="backend-gap">{{ $t('rfx.templates.versionDetail.graphGap') }}</p>
      <RfxQuestionnaireReadOnlyView
        v-else
        :questionnaire="questionnaireApi.questionnaire.value"
        :loading="questionnaireApi.loading.value"
        :error="questionnaireApi.error.value"
      />
    </template>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.read-only-banner { padding: 0.75rem 1rem; background: #f3f4f6; border-radius: var(--radius-md); }
.backend-gap { color: #b45309; }
.meta-list { display: grid; gap: 0.5rem; font-size: 0.875rem; }
.meta-list div { display: grid; grid-template-columns: 10rem 1fr; gap: 0.5rem; }
.meta-list dt { color: var(--color-text-muted); }
</style>
