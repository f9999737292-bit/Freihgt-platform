<script setup lang="ts">
import { LOW_CODE_ENTITY_TYPES, buildActivePublishedTemplateIdSet, formatLowCodeDate, isActivePublishedTemplate, type FormTemplateSummary } from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listFormTemplates, loadActivePublishedTemplateIds, isApiUnavailableError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<FormTemplateSummary[]>([])
const activeTemplateIds = ref<Set<string>>(new Set())
const loading = ref(true)
const loadFailed = ref(false)

const filters = reactive({ entity_type: '' })

const entityTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),
])

function onFilterChange() {
  load()
}

async function load() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listFormTemplates(filters.entity_type || undefined)
    items.value = data.items
    if (filters.entity_type) {
      activeTemplateIds.value = await loadActivePublishedTemplateIds([filters.entity_type])
    } else {
      const entityTypes = [...new Set(data.items.map((item) => item.entity_type))]
      activeTemplateIds.value = entityTypes.length
        ? await loadActivePublishedTemplateIds(entityTypes)
        : buildActivePublishedTemplateIdSet(data.items)
    }
  } catch (error) {
    items.value = []
    activeTemplateIds.value = new Set()
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('lowCode.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.formTemplates') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.formTemplates')">
      <template #actions>
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
        $t('lowCode.versionStatus'),
        $t('lowCode.version'),
        $t('lowCode.sectionsCount'),
        $t('lowCode.fieldsCount'),
        $t('lowCode.publishedAt'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.id">
        <td>{{ item.entity_type }}</td>
        <td><code>{{ item.code }}</code></td>
        <td>{{ item.name }}</td>
        <td><UiBadge :status="item.status" /></td>
        <td>
          <div class="badge-stack">
            <UiBadge
              v-if="isActivePublishedTemplate(item.id, activeTemplateIds)"
              :status="$t('lowCode.active')"
              tone="success"
            />
            <UiBadge
              v-else
              :status="$t('lowCode.olderPublishedVersion')"
              tone="warning"
            />
          </div>
        </td>
        <td>{{ item.version }}</td>
        <td>{{ item.sections_count }}</td>
        <td>{{ item.fields_count }}</td>
        <td>{{ formatLowCodeDate(item.published_at) }}</td>
        <td>
          <NuxtLink :to="`/low-code/form-templates/${item.id}`">{{ $t('lowCode.open') }}</NuxtLink>
        </td>
      </tr>
    </UiTable>

    <UiEmptyState v-else :title="$t('lowCode.noTemplatesFound')" />
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

.badge-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}
</style>
