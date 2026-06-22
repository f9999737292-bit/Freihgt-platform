<script setup lang="ts">
import {
  LOW_CODE_ENTITY_TYPES,
  formatJsonValue,
  formatLowCodeDate,
  type AuditEventItem,
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
  action: 'CUSTOM_FIELD_VALUES_UPDATED',
  limit: 50,
})

const entityTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),
])

const actionOptions = computed(() => [
  { label: t('common.all'), value: '' },
  { label: 'CUSTOM_FIELD_VALUES_UPDATED', value: 'CUSTOM_FIELD_VALUES_UPDATED' },
])

const limitOptions = [
  { label: '10', value: 10 },
  { label: '25', value: 25 },
  { label: '50', value: 50 },
  { label: '100', value: 100 },
]

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
      action: filters.action.trim() || undefined,
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

function formatChangedFields(fields: string[]) {
  return fields?.length ? fields.join(', ') : '—'
}

onMounted(() => {
  const q = route.query
  if (typeof q.entity_type === 'string') filters.entity_type = q.entity_type as LowCodeEntityType | ''
  if (typeof q.entity_id === 'string') filters.entity_id = q.entity_id
  if (typeof q.action === 'string') filters.action = q.action
  if (typeof q.limit === 'string') {
    const parsed = Number.parseInt(q.limit, 10)
    if (!Number.isNaN(parsed)) filters.limit = parsed
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
      <div class="filters-actions">
        <UiButton @click="onFilterChange">{{ $t('common.search') }}</UiButton>
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

      <div v-else-if="items.length === 0" class="empty-state">
        {{ $t('lowCode.noAuditEventsFound') }}
      </div>

      <div v-else class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>{{ $t('lowCode.createdAt') }}</th>
              <th>{{ $t('lowCode.entityType') }}</th>
              <th>{{ $t('lowCode.entityId') }}</th>
              <th>{{ $t('lowCode.action') }}</th>
              <th>{{ $t('lowCode.actor') }}</th>
              <th>{{ $t('lowCode.changedFields') }}</th>
              <th>{{ $t('lowCode.oldValues') }}</th>
              <th>{{ $t('lowCode.newValues') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in items" :key="item.id">
              <td>{{ formatLowCodeDate(item.created_at) }}</td>
              <td>{{ item.entity_type }}</td>
              <td class="mono">{{ item.entity_id }}</td>
              <td>{{ item.action }}</td>
              <td>{{ item.actor || '—' }}</td>
              <td>{{ formatChangedFields(item.changed_fields) }}</td>
              <td>
                <details class="json-details">
                  <summary>{{ $t('common.details') }}</summary>
                  <pre>{{ formatJsonValue(item.old_values) }}</pre>
                </details>
              </td>
              <td>
                <details class="json-details">
                  <summary>{{ $t('common.details') }}</summary>
                  <pre>{{ formatJsonValue(item.new_values) }}</pre>
                </details>
              </td>
            </tr>
          </tbody>
        </table>
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

.filters-actions {
  margin-top: 1rem;
}

.empty-state {
  padding: 2rem 0;
  text-align: center;
  color: var(--color-text-muted);
}

.table-wrap {
  overflow-x: auto;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.data-table th,
.data-table td {
  padding: 0.625rem 0.75rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.data-table th {
  font-weight: 600;
  white-space: nowrap;
}

.mono {
  font-family: ui-monospace, monospace;
  font-size: 0.8125rem;
  word-break: break-all;
}

.json-details summary {
  cursor: pointer;
  color: var(--color-primary);
}

.json-details pre {
  margin: 0.5rem 0 0;
  max-width: 280px;
  max-height: 200px;
  overflow: auto;
  padding: 0.5rem;
  background: var(--color-surface-muted, #f8fafc);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
