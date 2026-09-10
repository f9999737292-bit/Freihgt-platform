<script setup lang="ts">
import type { RfxQuestionnaireDefinition } from '~/types/rfx-questionnaire'
import type { RfxTemplateDetailResponse, RfxTemplateVersionRecord } from '~/types/rfx-template'
import { isTemplateVersionEditable } from '~/types/rfx-template'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { formatRfxDateTime } from '~/utils/formatRfxDateTime'
import { findTemplateVersionById } from '~/utils/rfxTemplateLifecycle'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const { t, locale } = useI18n()
const { pushToast } = useToast()
const { getTemplate, getTemplateVersionQuestionnaire } = useRfxTemplateApi()

const templateId = computed(() => String(route.params.id))
const versionId = computed(() => String(route.params.versionId))
const detail = ref<RfxTemplateDetailResponse | null>(null)
const version = ref<RfxTemplateVersionRecord | null>(null)
const questionnaire = ref<RfxQuestionnaireDefinition | null>(null)
const loading = ref(true)
const graphLoading = ref(false)
const graphError = ref('')

onMounted(async () => {
  loading.value = true
  try {
    detail.value = await getTemplate(templateId.value)
    version.value = findTemplateVersionById(detail.value, versionId.value) ?? null
    if (!version.value) return
    if (isTemplateVersionEditable(version.value.status) && detail.value.draft_version?.id === version.value.id) {
      await navigateTo(`/rfx/templates/${templateId.value}`)
      return
    }
    graphLoading.value = true
    try {
      questionnaire.value = await getTemplateVersionQuestionnaire(templateId.value, versionId.value)
    } catch (e) {
      graphError.value = formatRfxApiError(e, t)
    } finally {
      graphLoading.value = false
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

      <RfxQuestionnaireReadOnlyView
        :questionnaire="questionnaire"
        :loading="graphLoading"
        :error="graphError"
      />
    </template>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.read-only-banner { padding: 0.75rem 1rem; background: #f3f4f6; border-radius: var(--radius-md); }
.meta-list { display: grid; gap: 0.5rem; font-size: 0.875rem; }
.meta-list div { display: grid; grid-template-columns: 10rem 1fr; gap: 0.5rem; }
.meta-list dt { color: var(--color-text-muted); }
</style>
