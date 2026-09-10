<script setup lang="ts">
import type { RfxCompareVersionsResponse, RfxVersionRecord } from '~/types/rfx-version-lifecycle'
import { swapCompareDirection } from '~/utils/rfxVersionCompare'
import { formatRfxApiError } from '~/utils/rfxApiError'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const route = useRoute()
const { t } = useI18n()
const { pushToast } = useToast()

const eventId = computed(() => String(route.params.id))
const lifecycleApi = useRfxVersionLifecycleApi(eventId)

const versions = ref<RfxVersionRecord[]>([])
const sourceId = ref('')
const targetId = ref('')
const compareResult = ref<RfxCompareVersionsResponse | null>(null)
const loading = ref(false)

onMounted(async () => {
  try {
    const data = await lifecycleApi.listVersions()
    versions.value = data.versions ?? []
    if (versions.value.length >= 2) {
      sourceId.value = versions.value[versions.value.length - 2]?.id ?? ''
      targetId.value = versions.value[versions.value.length - 1]?.id ?? ''
      await runCompare()
    }
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  }
})

async function runCompare() {
  if (!sourceId.value || !targetId.value) return
  loading.value = true
  try {
    compareResult.value = await lifecycleApi.compareVersions({
      source_version_id: sourceId.value,
      target_version_id: targetId.value,
    })
  } catch (e) {
    compareResult.value = null
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    loading.value = false
  }
}

async function swapDirection() {
  const swapped = swapCompareDirection({
    source_version_id: sourceId.value,
    target_version_id: targetId.value,
  })
  sourceId.value = swapped.source_version_id
  targetId.value = swapped.target_version_id
  await runCompare()
}
</script>

<template>
  <div class="page">
    <NuxtLink :to="`/rfx/${eventId}/versions`">{{ $t('rfx.versions.back') }}</NuxtLink>
    <h1>{{ $t('rfx.compare.title') }}</h1>

    <div class="compare-controls">
      <label>
        {{ $t('rfx.compare.from') }}
        <select v-model="sourceId">
          <option v-for="ver in versions" :key="ver.id" :value="ver.id">
            v{{ ver.version_number }} ({{ ver.status }})
          </option>
        </select>
      </label>
      <label>
        {{ $t('rfx.compare.to') }}
        <select v-model="targetId">
          <option v-for="ver in versions" :key="ver.id" :value="ver.id">
            v{{ ver.version_number }} ({{ ver.status }})
          </option>
        </select>
      </label>
      <button type="button" class="btn btn--secondary" @click="runCompare">{{ $t('rfx.compare.run') }}</button>
      <button type="button" class="btn btn--link" @click="swapDirection">{{ $t('rfx.compare.swap') }}</button>
    </div>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <RfxVersionCompareView v-else :compare="compareResult" />
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.compare-controls { display: flex; flex-wrap: wrap; gap: 0.75rem; align-items: flex-end; }
.compare-controls label { display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.875rem; }
</style>
