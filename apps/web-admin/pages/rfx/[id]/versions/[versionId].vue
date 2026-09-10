<script setup lang="ts">
import type { RfxVersionDetailResponse } from '~/types/rfx-version-lifecycle'
import { isEventVersionEditable } from '~/types/rfx-version-lifecycle'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { formatRfxDateTime } from '~/utils/formatRfxDateTime'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const { t, locale } = useI18n()
const { pushToast } = useToast()
const { canManageEventVersions } = useRfxBuyerPermissions()

const eventId = computed(() => String(route.params.id))
const versionId = computed(() => String(route.params.versionId))
const lifecycleApi = useRfxVersionLifecycleApi(eventId)

const detail = ref<RfxVersionDetailResponse | null>(null)
const loading = ref(true)

onMounted(async () => {
  loading.value = true
  try {
    detail.value = await lifecycleApi.getVersionDetail(versionId.value)
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <NuxtLink :to="`/rfx/${eventId}/versions`">{{ $t('rfx.versions.back') }}</NuxtLink>
    <h1>{{ $t('rfx.versions.detailTitle', { number: detail?.version.version_number ?? '—' }) }}</h1>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else-if="!detail">{{ $t('rfx.errors.notFound') }}</p>

    <template v-else>
      <p v-if="!isEventVersionEditable(detail.version.status)" class="read-only-banner">
        {{ $t('rfx.versions.immutableBanner') }}
      </p>
      <p v-if="detail.version.rescoring_required" class="warning-banner">
        {{ $t('rfx.changeImpact.rescoringRequired') }}
      </p>

      <dl class="meta-list">
        <div><dt>{{ $t('rfx.versions.columns.status') }}</dt><dd>{{ $t(`rfx.versions.status.${detail.version.status}`) }}</dd></div>
        <div><dt>{{ $t('rfx.versions.columns.publishedAt') }}</dt><dd>{{ formatRfxDateTime(detail.version.published_at, locale) }}</dd></div>
        <div><dt>{{ $t('rfx.versions.columns.summary') }}</dt><dd>{{ detail.version.change_summary || '—' }}</dd></div>
      </dl>

      <RfxQuestionnaireReadOnlyView :questionnaire="detail.questionnaire" />
    </template>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.read-only-banner { padding: 0.75rem 1rem; background: #f3f4f6; border-radius: var(--radius-md); }
.warning-banner { padding: 0.75rem 1rem; background: #fffbeb; color: #b45309; border-radius: var(--radius-md); }
.meta-list { display: grid; gap: 0.5rem; font-size: 0.875rem; }
.meta-list div { display: grid; grid-template-columns: 10rem 1fr; gap: 0.5rem; }
.meta-list dt { color: var(--color-text-muted); }
</style>
