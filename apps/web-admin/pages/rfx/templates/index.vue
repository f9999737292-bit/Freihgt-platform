<script setup lang="ts">
import type { RfxTemplateRecord } from '~/types/rfx-template'
import { RFX_TEMPLATE_AGGREGATE_STATUSES } from '~/types/rfx-template'
import { resolveI18nMapValue } from '~/utils/rfxTemplateI18n'
import { formatRfxApiError } from '~/utils/rfxApiError'

definePageMeta({ middleware: ['auth', 'rfx-buyer-manage'], layout: 'default' })

const { listTemplates } = useRfxTemplateApi()
const { canManageRfxTemplates } = useRfxBuyerPermissions()
const { pushToast } = useToast()
const { t, locale } = useI18n()
const router = useRouter()

const items = ref<RfxTemplateRecord[]>([])
const total = ref(0)
const loading = ref(true)
const loadFailed = ref(false)
const forbidden = ref(false)
const showCreateModal = ref(false)
const showCloneModal = ref(false)
const templateVersions = ref(new Map<string, never[]>())

const filters = reactive({
  search: '',
  status: '' as '' | (typeof RFX_TEMPLATE_AGGREGATE_STATUSES)[number],
})
const pagination = reactive({ limit: 20, offset: 0 })

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...RFX_TEMPLATE_AGGREGATE_STATUSES.map((v) => ({
    label: t(`rfx.templates.aggregateStatus.${v}`),
    value: v,
  })),
])

function templateName(tpl: RfxTemplateRecord) {
  return resolveI18nMapValue(tpl.name_i18n, locale.value) || tpl.template_code
}

async function loadLibrary() {
  loading.value = true
  loadFailed.value = false
  forbidden.value = false
  try {
    const data = await listTemplates({
      search: filters.search,
      status: filters.status || undefined,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items
    total.value = data.total ?? data.items.length
  } catch (e) {
    items.value = []
    if (e instanceof Error && 'status' in e && (e as { status: number }).status === 403) {
      forbidden.value = true
    } else {
      loadFailed.value = true
      pushToast('error', formatRfxApiError(e, t))
    }
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => filters.search,
  () => {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      pagination.offset = 0
      void loadLibrary()
    }, 300)
  },
)
watch(() => filters.status, () => {
  pagination.offset = 0
  void loadLibrary()
})

onMounted(() => void loadLibrary())

function openTemplate(id: string) {
  void router.push(`/rfx/templates/${id}`)
}
</script>

<template>
  <div class="page">
    <header class="page__header">
      <div>
        <h1>{{ $t('rfx.templates.libraryTitle') }}</h1>
        <p class="page__subtitle">{{ $t('rfx.templates.librarySubtitle') }}</p>
      </div>
      <div v-if="canManageRfxTemplates()" class="page__actions">
        <button type="button" class="btn btn--secondary" @click="showCloneModal = true">
          {{ $t('rfx.templates.clone.title') }}
        </button>
        <button type="button" class="btn btn--primary" @click="showCreateModal = true">
          {{ $t('rfx.templates.create') }}
        </button>
      </div>
    </header>

    <div class="filters">
      <input v-model="filters.search" type="search" :placeholder="$t('rfx.templates.search')" />
      <select v-model="filters.status">
        <option v-for="opt in statusOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
      </select>
    </div>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else-if="forbidden">{{ $t('rfx.errors.forbidden') }}</p>
    <p v-else-if="loadFailed">{{ $t('common.loadFailed') }}</p>
    <p v-else-if="items.length === 0">{{ $t('rfx.templates.empty') }}</p>

    <table v-else class="data-table">
      <thead>
        <tr>
          <th>{{ $t('rfx.templates.columns.name') }}</th>
          <th>{{ $t('rfx.templates.columns.code') }}</th>
          <th>{{ $t('rfx.templates.columns.status') }}</th>
          <th>{{ $t('rfx.templates.columns.updated') }}</th>
          <th>{{ $t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="tpl in items" :key="tpl.id">
          <td>{{ templateName(tpl) }}</td>
          <td><code>{{ tpl.template_code }}</code></td>
          <td><RfxTemplateStatusBadge :aggregate-status="tpl.status" /></td>
          <td>{{ tpl.updated_at }}</td>
          <td>
            <button type="button" class="btn btn--link" @click="openTemplate(tpl.id)">
              {{ $t('common.open') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>

    <RfxTemplateCreateModal
      :open="showCreateModal"
      @close="showCreateModal = false"
      @created="loadLibrary"
    />

    <RfxCreateFromTemplateModal
      :open="showCloneModal"
      :templates="items.filter((t) => t.status === 'ACTIVE')"
      :template-versions="templateVersions"
      @close="showCloneModal = false"
      @created="(id: string) => { showCloneModal = false; void router.push(`/rfx/${id}/studio`) }"
    />
  </div>
</template>

<style scoped>
.page { padding: 1.5rem; display: flex; flex-direction: column; gap: 1rem; }
.page__header { display: flex; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.page__subtitle { margin: 0.25rem 0 0; color: var(--color-text-muted); }
.page__actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
.filters { display: flex; gap: 0.75rem; flex-wrap: wrap; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { text-align: left; padding: 0.625rem; border-bottom: 1px solid var(--color-border); }
</style>
