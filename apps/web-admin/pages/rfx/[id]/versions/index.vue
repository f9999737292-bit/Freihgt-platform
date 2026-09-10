<script setup lang="ts">
import type { RfxVersionRecord } from '~/types/rfx-version-lifecycle'
import { isEventVersionEditable } from '~/types/rfx-version-lifecycle'
import { formatRfxApiError } from '~/utils/rfxApiError'
import { createIdempotencyKey } from '~/utils/idempotencyKey'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const { pushToast } = useToast()
const { canManageEventVersions } = useRfxBuyerPermissions()

const eventId = computed(() => String(route.params.id))
const lifecycleApi = useRfxVersionLifecycleApi(eventId)

const versions = ref<RfxVersionRecord[]>([])
const loading = ref(true)
const restoreOpen = ref(false)
const restoreTarget = ref<RfxVersionRecord | null>(null)
const restoreError = ref('')
const restoring = ref(false)

async function loadVersions() {
  loading.value = true
  try {
    const data = await lifecycleApi.listVersions()
    versions.value = data.versions ?? []
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
}

onMounted(() => void loadVersions())

function openRestore(version: RfxVersionRecord) {
  restoreTarget.value = version
  restoreError.value = ''
  restoreOpen.value = true
}

async function handleRestore(changeSummary: string) {
  if (!restoreTarget.value) return
  restoring.value = true
  restoreError.value = ''
  try {
    await lifecycleApi.restoreAsDraft(
      restoreTarget.value.id,
      { change_summary: changeSummary },
      createIdempotencyKey('evt-restore'),
    )
    restoreOpen.value = false
    pushToast('success', t('rfx.restore.success'))
    await loadVersions()
    await router.push(`/rfx/${eventId.value}/studio`)
  } catch (e) {
    restoreError.value = formatRfxApiError(e, t)
  } finally {
    restoring.value = false
  }
}

function goCompare() {
  void router.push(`/rfx/${eventId.value}/versions/compare`)
}
</script>

<template>
  <div class="page">
    <NuxtLink :to="`/rfx/${eventId}`">{{ $t('rfx.backToDetail') }}</NuxtLink>
    <header class="page__header">
      <h1>{{ $t('rfx.versions.title') }}</h1>
      <button type="button" class="btn btn--secondary" @click="goCompare">
        {{ $t('rfx.compare.open') }}
      </button>
    </header>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('rfx.versions.columns.number') }}</th>
          <th>{{ $t('rfx.versions.columns.status') }}</th>
          <th>{{ $t('rfx.versions.columns.publishedAt') }}</th>
          <th>{{ $t('rfx.versions.columns.summary') }}</th>
          <th v-if="canManageEventVersions()">{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ver in versions" :key="ver.id">
          <td>v{{ ver.version_number }}</td>
          <td><RfxVersionStatusBadge :status="ver.status" /></td>
          <td>{{ ver.published_at || '—' }}</td>
          <td>{{ ver.change_summary || '—' }}</td>
          <td v-if="canManageEventVersions()">
            <button
              v-if="!isEventVersionEditable(ver.status)"
              type="button"
              class="btn btn--link"
              @click="openRestore(ver)"
            >
              {{ $t('rfx.restore.action') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <RfxRestoreDraftDialog
      :open="restoreOpen"
      :source-version="restoreTarget"
      :submitting="restoring"
      :error-message="restoreError"
      @close="restoreOpen = false"
      @confirm="handleRestore"
    />
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.page__header { display: flex; justify-content: space-between; align-items: center; gap: 1rem; flex-wrap: wrap; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { text-align: left; padding: 0.625rem; border-bottom: 1px solid var(--color-border); }
</style>
