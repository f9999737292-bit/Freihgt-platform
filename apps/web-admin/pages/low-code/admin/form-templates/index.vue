<script setup lang="ts">
import {
  LOW_CODE_ADMIN_ENTITY_TYPES,
  LOW_CODE_TEMPLATE_STATUSES,
  formatLowCodeDate,
  type FormTemplateSummary,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listAdminFormTemplates, isApiUnavailableError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<FormTemplateSummary[]>([])
const loading = ref(true)
const loadFailed = ref(false)

const filters = reactive({
  entity_type: '',
  status: 'DRAFT',
})

const entityTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...LOW_CODE_ADMIN_ENTITY_TYPES.map((value) => ({ label: value, value })),
])

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...LOW_CODE_TEMPLATE_STATUSES.map((value) => ({ label: value, value })),
])

async function load() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listAdminFormTemplates({
      entity_type: filters.entity_type || undefined,
      status: filters.status || undefined,
      limit: 100,
    })
    items.value = data.items
  } catch (error) {
    items.value = []
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('lowCode.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFilterChange() {
  load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.formTemplateAdmin') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.formTemplateDrafts')">
      <template #actions>
        <UiButton @click="$router.push('/low-code/admin/form-templates/new')">
          {{ $t('lowCode.newDraft') }}
        </UiButton>
        <UiButton variant="secondary" @click="$router.push('/low-code')">{{ $t('common.back') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-row">
        <UiSelect
          v-model="filters.entity_type"
          :label="$t('lowCode.entityType')"
          :options="entityTypeOptions"
          @update:model-value="onFilterChange"
        />
        <UiSelect
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFilterChange"
        />
      </div>
    </UiCard>

    <CommonApiUnavailableState
      v-if="loadFailed"
      :message="$t('lowCode.serviceUnavailable')"
      @retry="load"
    />

    <UiTable
      v-else-if="items.length || loading"
      :columns="[
        $t('lowCode.entityType'),
        $t('lowCode.code'),
        $t('common.name'),
        $t('common.status'),
        $t('lowCode.version'),
        $t('lowCode.updatedAt'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.id">
        <td>{{ item.entity_type }}</td>
        <td><code>{{ item.code }}</code></td>
        <td>{{ item.name }}</td>
        <td><UiBadge :status="item.status" /></td>
        <td>{{ item.version }}</td>
        <td>{{ formatLowCodeDate(item.published_at) }}</td>
        <td>
          <div class="actions-cell">
            <NuxtLink :to="`/low-code/admin/form-templates/${item.id}`">{{ $t('lowCode.open') }}</NuxtLink>
            <NuxtLink
              v-if="item.status === 'PUBLISHED'"
              :to="`/low-code/form-templates/${item.id}`"
            >
              {{ $t('lowCode.viewFormTemplate') }}
            </NuxtLink>
          </div>
        </td>
      </tr>
    </UiTable>

    <UiEmptyState v-else :title="$t('lowCode.noDraftTemplatesFound')" />
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.filters-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
}

.actions-cell {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
</style>
