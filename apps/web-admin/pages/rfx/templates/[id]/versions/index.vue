<script setup lang="ts">
import type { RfxTemplateVersionRecord } from '~/types/rfx-template'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { formatRfxDateTime } from '~/utils/formatRfxDateTime'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const { pushToast } = useToast()
const { getTemplate } = useRfxTemplateApi()

const templateId = computed(() => String(route.params.id))
const versions = ref<RfxTemplateVersionRecord[]>([])
const loading = ref(true)
const loadFailed = ref(false)

async function loadVersions() {
  loading.value = true
  loadFailed.value = false
  try {
    const detail = await getTemplate(templateId.value)
    versions.value = detail.versions ?? []
  } catch (e) {
    loadFailed.value = true
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
}

onMounted(() => void loadVersions())

function openVersion(versionId: string) {
  void router.push(`/rfx/templates/${templateId.value}/versions/${versionId}`)
}
</script>

<template>
  <div class="page">
    <NuxtLink :to="`/rfx/templates/${templateId}`">{{ $t('rfx.templates.backToTemplate') }}</NuxtLink>
    <h1>{{ $t('rfx.templates.versionHistoryTitle') }}</h1>
    <p v-if="loading">{{ $t('common.loading') }}</p>
    <div v-else-if="loadFailed" class="error-row">
      <p>{{ $t('common.loadFailed') }}</p>
      <button type="button" class="btn btn--secondary" @click="loadVersions">{{ $t('common.retry') }}</button>
    </div>
    <p v-else-if="versions.length === 0">{{ $t('rfx.templates.versionHistoryEmpty') }}</p>
    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('rfx.versions.columns.number') }}</th>
          <th>{{ $t('rfx.versions.columns.status') }}</th>
          <th>{{ $t('rfx.versions.columns.publishedAt') }}</th>
          <th>{{ $t('rfx.versions.columns.summary') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ver in versions" :key="ver.id">
          <td>v{{ ver.version_number }}</td>
          <td><RfxTemplateStatusBadge :version-status="ver.status" /></td>
          <td>{{ formatRfxDateTime(ver.published_at, locale) }}</td>
          <td>{{ ver.change_summary || '—' }}</td>
          <td>
            <button type="button" class="btn btn--link" @click="openVersion(ver.id)">
              {{ $t('common.open') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.error-row { display: flex; align-items: center; gap: 0.75rem; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { text-align: left; padding: 0.625rem; border-bottom: 1px solid var(--color-border); }
</style>
