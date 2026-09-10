<script setup lang="ts">
import type { RfxTemplateVersionRecord } from '~/types/rfx-template'
import { formatRfxApiError } from '~/utils/rfxApiError'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const { t } = useI18n()
const { pushToast } = useToast()
const { getTemplate } = useRfxTemplateApi()

const templateId = computed(() => String(route.params.id))
const versions = ref<RfxTemplateVersionRecord[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const detail = await getTemplate(templateId.value)
    versions.value = detail.versions ?? []
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <NuxtLink :to="`/rfx/templates/${templateId}`">{{ $t('rfx.templates.backToTemplate') }}</NuxtLink>
    <h1>{{ $t('rfx.templates.versionHistoryTitle') }}</h1>
    <p v-if="loading">{{ $t('common.loading') }}</p>
    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('rfx.versions.columns.number') }}</th>
          <th>{{ $t('rfx.versions.columns.status') }}</th>
          <th>{{ $t('rfx.versions.columns.publishedAt') }}</th>
          <th>{{ $t('rfx.versions.columns.summary') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ver in versions" :key="ver.id">
          <td>v{{ ver.version_number }}</td>
          <td><RfxTemplateStatusBadge :version-status="ver.status" /></td>
          <td>{{ ver.published_at || '—' }}</td>
          <td>{{ ver.change_summary || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { text-align: left; padding: 0.625rem; border-bottom: 1px solid var(--color-border); }
</style>
