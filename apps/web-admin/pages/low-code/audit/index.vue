<script setup lang="ts">
import {
  LOW_CODE_AUDIT_ACTION_MIGRATED_TO_ACTIVE,
  LOW_CODE_AUDIT_ACTION_VALUES_UPDATED,
  LOW_CODE_ENTITY_TYPES,
  auditEventMatchesBatchId,
  isBatchMigrationAuditEvent,
  isTemplateAuditEvent,
  type AuditEventItem,
  type LowCodeAuditQuickFilter,
  type LowCodeEntityType,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listAuditEvents, isApiUnavailableError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const route = useRoute()

const items = ref<AuditEventItem[]>([])
const loading = ref(true)
const loadFailed = ref(false)

const filters = reactive({
  entity_type: '' as LowCodeEntityType | '',
  entity_id: '',
  action: '',
  batch_id: '',
  limit: 50,
})

const quickFilter = ref<LowCodeAuditQuickFilter>('all')

const entityTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),
])

const actionOptions = computed(() => [
  { label: t('lowCode.auditAllActions'), value: '' },
  { label: LOW_CODE_AUDIT_ACTION_VALUES_UPDATED, value: LOW_CODE_AUDIT_ACTION_VALUES_UPDATED },
  { label: LOW_CODE_AUDIT_ACTION_MIGRATED_TO_ACTIVE, value: LOW_CODE_AUDIT_ACTION_MIGRATED_TO_ACTIVE },
  { label: 'FORM_TEMPLATE_DRAFT_CREATED', value: 'FORM_TEMPLATE_DRAFT_CREATED' },
  { label: 'FORM_TEMPLATE_DRAFT_UPDATED', value: 'FORM_TEMPLATE_DRAFT_UPDATED' },
  { label: 'FORM_TEMPLATE_DRAFT_PUBLISHED', value: 'FORM_TEMPLATE_DRAFT_PUBLISHED' },
  { label: 'FORM_TEMPLATE_CLONED_TO_DRAFT', value: 'FORM_TEMPLATE_CLONED_TO_DRAFT' },
])

const quickFilterOptions = computed(() => [
  { id: 'all' as const, label: t('lowCode.auditAllActions') },
  { id: 'value_updates' as const, label: t('lowCode.auditValueUpdates') },
  { id: 'template_changes' as const, label: t('lowCode.auditTemplateChanges') },
  { id: 'migrations' as const, label: t('lowCode.auditMigrations') },
  { id: 'batch_migrations' as const, label: t('lowCode.auditBatchMigrations') },
])

const limitOptions = [
  { label: '10', value: 10 },
  { label: '25', value: 25 },
  { label: '50', value: 50 },
  { label: '100', value: 100 },
]

const displayedItems = computed(() => {
  let rows = items.value
  if (quickFilter.value === 'template_changes') {
    rows = rows.filter((item) => isTemplateAuditEvent(item))
  } else if (quickFilter.value === 'batch_migrations') {
    rows = rows.filter((item) => isBatchMigrationAuditEvent(item))
  }
  if (filters.batch_id.trim()) {
    rows = rows.filter((item) => auditEventMatchesBatchId(item, filters.batch_id))
  }
  return rows
})

const emptyMessage = computed(() => {
  if (quickFilter.value === 'batch_migrations') {
    return t('lowCode.auditNoBatchMigrationEventsFound')
  }
  if (quickFilter.value === 'migrations') {
    return t('lowCode.auditNoMigrationEventsFound')
  }
  return t('lowCode.noAuditEventsFound')
})

const hasActiveFilters = computed(() =>
  Boolean(
    filters.entity_type
    || filters.entity_id.trim()
    || filters.action.trim()
    || filters.batch_id.trim()
    || quickFilter.value !== 'all'
    || filters.limit !== 50,
  ),
)

function resolveActionForLoad(): string | undefined {
  if (filters.action.trim()) return filters.action.trim()
  if (quickFilter.value === 'migrations' || quickFilter.value === 'batch_migrations') {
    return LOW_CODE_AUDIT_ACTION_MIGRATED_TO_ACTIVE
  }
  if (quickFilter.value === 'value_updates') return LOW_CODE_AUDIT_ACTION_VALUES_UPDATED
  return undefined
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
    const data = await listAuditEvents({
      entity_type: filters.entity_type || undefined,
      entity_id: filters.entity_id.trim() || undefined,
      action: resolveActionForLoad(),
      limit: filters.limit,
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

function setQuickFilter(value: LowCodeAuditQuickFilter) {
  quickFilter.value = value
  if (value === 'migrations' || value === 'batch_migrations' || value === 'value_updates' || value === 'all' || value === 'template_changes') {
    filters.action = ''
  }
  load()
}

function clearFilters() {
  filters.entity_type = ''
  filters.entity_id = ''
  filters.action = ''
  filters.batch_id = ''
  filters.limit = 50
  quickFilter.value = 'all'
  load()
}

function parseCategory(value: string | undefined): LowCodeAuditQuickFilter {
  if (
    value === 'migrations'
    || value === 'value_updates'
    || value === 'template_changes'
    || value === 'batch_migrations'
  ) {
    return value
  }
  return 'all'
}

onMounted(() => {
  const q = route.query
  if (typeof q.entity_type === 'string') filters.entity_type = q.entity_type as LowCodeEntityType | ''
  if (typeof q.entity_id === 'string') filters.entity_id = q.entity_id
  if (typeof q.action === 'string') filters.action = q.action
  if (typeof q.batch_id === 'string') filters.batch_id = q.batch_id
  if (typeof q.category === 'string') quickFilter.value = parseCategory(q.category)
  if (typeof q.limit === 'string') {
    const parsed = Number.parseInt(q.limit, 10)
    if (!Number.isNaN(parsed)) filters.limit = parsed
  }
  if (filters.action === LOW_CODE_AUDIT_ACTION_MIGRATED_TO_ACTIVE) {
    quickFilter.value = filters.batch_id.trim() ? 'batch_migrations' : 'migrations'
    filters.action = ''
  } else if (filters.action === LOW_CODE_AUDIT_ACTION_VALUES_UPDATED) {
    quickFilter.value = 'value_updates'
    filters.action = ''
  }
  load()
})
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.auditLog') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.auditLog')">
      <template #actions>
        <UiButton variant="secondary" :disabled="loading" @click="load">
          {{ loading ? $t('common.loading') : $t('lowCode.reloadAuditEvents') }}
        </UiButton>
        <UiButton variant="secondary" @click="$router.push('/low-code')">{{ $t('common.back') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-grid">
        <UiSelect
          v-model="filters.entity_type"
          :label="$t('lowCode.entityType')"
          :options="entityTypeOptions"
          @update:model-value="onFilterChange"
        />
        <UiInput
          v-model="filters.entity_id"
          :label="$t('lowCode.entityId')"
          :placeholder="$t('lowCode.entityIdPlaceholder')"
          @keyup.enter="onFilterChange"
        />
        <UiInput
          v-model="filters.batch_id"
          class="audit-batch-id-input"
          :label="$t('lowCode.auditBatchId')"
          :placeholder="$t('lowCode.auditBatchIdPlaceholder')"
          @keyup.enter="onFilterChange"
        />
        <UiSelect
          v-model="filters.action"
          :label="$t('lowCode.action')"
          :options="actionOptions"
          @update:model-value="onFilterChange"
        />
        <UiSelect
          v-model="filters.limit"
          :label="$t('lowCode.limit')"
          :options="limitOptions"
          @update:model-value="onFilterChange"
        />
      </div>

      <div class="quick-filters">
        <span class="quick-filters__label">{{ $t('lowCode.auditMigrationHistory') }}</span>
        <div class="quick-filters__buttons">
          <UiButton
            v-for="option in quickFilterOptions"
            :key="option.id"
            size="sm"
            :variant="quickFilter === option.id ? 'primary' : 'secondary'"
            @click="setQuickFilter(option.id)"
          >
            {{ option.label }}
          </UiButton>
        </div>
      </div>

      <div class="filters-actions">
        <UiButton @click="onFilterChange">{{ $t('common.search') }}</UiButton>
        <UiButton v-if="hasActiveFilters" variant="secondary" @click="clearFilters">
          {{ $t('lowCode.auditClearFilters') }}
        </UiButton>
      </div>
    </UiCard>

    <CommonApiUnavailableState
      v-if="loadFailed"
      :title="$t('common.apiUnavailable')"
      :hint="$t('common.apiUnavailableHint')"
      @retry="load"
    />

    <UiCard v-else>
      <template #header>{{ $t('lowCode.auditEvents') }}</template>

      <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>

      <div v-else-if="displayedItems.length === 0" class="empty-state">
        <p class="empty-state__title">{{ emptyMessage }}</p>
        <p v-if="filters.batch_id.trim()" class="empty-state__hint">
          {{ $t('lowCode.auditEmptyBatchIdHint') }}
        </p>
        <UiButton
          v-if="hasActiveFilters"
          size="sm"
          variant="secondary"
          @click="clearFilters"
        >
          {{ $t('lowCode.auditClearFilters') }}
        </UiButton>
      </div>

      <div v-else class="audit-event-list">
        <LowCodeAuditEventCard
          v-for="item in displayedItems"
          :key="item.id"
          :event="item"
        />
      </div>
    </UiCard>
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

.filters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.quick-filters {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-top: 1rem;
}

.quick-filters__label {
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--color-text-muted);
}

.quick-filters__buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.filters-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 1rem;
}

.audit-batch-id-input :deep(input) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 2rem 1rem;
  text-align: center;
  color: var(--color-text-muted);
}

.empty-state__title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 500;
  color: var(--color-text);
}

.empty-state__hint {
  margin: 0;
  max-width: 36rem;
  font-size: 0.8125rem;
}

.audit-event-list {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}
</style>
