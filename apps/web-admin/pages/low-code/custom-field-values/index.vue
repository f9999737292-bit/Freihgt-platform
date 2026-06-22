<script setup lang="ts">
import {
  DEMO_ENTITY_REFS,
  LOW_CODE_ENTITY_TYPES,
  formatJsonValue,
  formatLowCodeDate,
  type AuditEventItem,
  type LowCodeEntityType,
} from '~/types/lowCode'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { resolveDemoEntityId, listAuditEvents } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const form = reactive({
  entity_type: 'TRANSPORT_ORDER' as LowCodeEntityType,
  entity_id: '',
})

const loadedEntity = reactive({
  entity_type: '' as LowCodeEntityType | '',
  entity_id: '',
})

const loaded = ref(false)
const resolvingDemo = ref(false)
const panelKey = ref(0)
const recentAuditEvents = ref<AuditEventItem[]>([])
const loadingRecentAudit = ref(false)
const showAuditLink = ref(false)

const entityTypeOptions = computed(() =>
  LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),
)

const demoRefHint = computed(() => DEMO_ENTITY_REFS[form.entity_type])

const auditLogLink = computed(() => {
  if (!loadedEntity.entity_id) return '/low-code/audit'
  const query = new URLSearchParams({
    entity_type: loadedEntity.entity_type,
    entity_id: loadedEntity.entity_id,
    action: 'CUSTOM_FIELD_VALUES_UPDATED',
  })
  return `/low-code/audit?${query.toString()}`
})

function markLoaded() {
  loadedEntity.entity_type = form.entity_type
  loadedEntity.entity_id = form.entity_id.trim()
  loaded.value = true
  panelKey.value += 1
  showAuditLink.value = false
  recentAuditEvents.value = []
}

async function loadRecentAuditEvents() {
  if (!hasTenant.value || !loadedEntity.entity_id) return
  loadingRecentAudit.value = true
  try {
    const data = await listAuditEvents({
      entity_type: loadedEntity.entity_type,
      entity_id: loadedEntity.entity_id,
      action: 'CUSTOM_FIELD_VALUES_UPDATED',
      limit: 5,
    })
    recentAuditEvents.value = data.items
  } catch {
    recentAuditEvents.value = []
  } finally {
    loadingRecentAudit.value = false
  }
}

async function onPanelSaved() {
  showAuditLink.value = true
  await loadRecentAuditEvents()
}

async function loadValues() {
  if (!hasTenant.value) return
  if (!form.entity_id.trim()) {
    pushToast('error', t('lowCode.entityIdRequired'))
    return
  }
  markLoaded()
  await loadRecentAuditEvents()
}

async function useDemoEntity() {
  if (!hasTenant.value) return
  resolvingDemo.value = true
  try {
    const id = await resolveDemoEntityId(form.entity_type)
    if (!id) {
      pushToast('error', t('lowCode.demoEntityNotFound', { ref: demoRefHint.value }))
      return
    }
    form.entity_id = id
    markLoaded()
    await loadRecentAuditEvents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('lowCode.demoResolveFailed'))
  } finally {
    resolvingDemo.value = false
  }
}

function reloadPanel() {
  if (!loadedEntity.entity_id) return
  panelKey.value += 1
}

function formatChangedFields(fields: string[]) {
  return fields?.length ? fields.join(', ') : '—'
}
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.customFieldValues') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.customFieldValues')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/low-code')">{{ $t('common.back') }}</UiButton>
      </template>
    </UiPageHeader>

    <div class="low-code-hub__notice low-code-hub__notice--warn">
      <strong>{{ $t('lowCode.editCustomFields') }}</strong>
      <p>{{ $t('lowCode.coreEntityNotChanged') }}</p>
    </div>

    <UiCard>
      <div class="lookup-form">
        <UiSelect
          v-model="form.entity_type"
          :label="$t('lowCode.entityType')"
          :options="entityTypeOptions"
        />
        <UiInput
          v-model="form.entity_id"
          :label="$t('lowCode.entityId')"
          :placeholder="$t('lowCode.entityIdPlaceholder')"
        />
        <div class="lookup-form__actions">
          <UiButton @click="loadValues">{{ $t('lowCode.loadValues') }}</UiButton>
          <UiButton variant="secondary" :loading="resolvingDemo" @click="useDemoEntity">
            {{ $t('lowCode.useDemoEntity') }}
          </UiButton>
          <UiButton
            v-if="loaded && loadedEntity.entity_id"
            variant="secondary"
            @click="reloadPanel"
          >
            {{ $t('lowCode.reloadValues') }}
          </UiButton>
        </div>
      </div>
      <p class="demo-hint">
        {{ $t('lowCode.demoHint', { entityType: form.entity_type, demoRef: demoRefHint }) }}
      </p>
    </UiCard>

    <LowCodeCustomFieldsPanel
      v-if="loaded && loadedEntity.entity_id"
      :key="panelKey"
      :entity-type="loadedEntity.entity_type"
      :entity-id="loadedEntity.entity_id"
      editable
      @saved="onPanelSaved"
    />

    <UiCard v-if="loaded && loadedEntity.entity_id && (showAuditLink || recentAuditEvents.length)">
      <template #header>{{ $t('lowCode.auditEvents') }}</template>
      <div class="audit-actions">
        <NuxtLink :to="auditLogLink" class="audit-link">{{ $t('lowCode.viewAuditLog') }}</NuxtLink>
        <UiButton
          v-if="showAuditLink"
          size="sm"
          variant="secondary"
          :loading="loadingRecentAudit"
          @click="loadRecentAuditEvents"
        >
          {{ $t('lowCode.reloadAuditEvents') }}
        </UiButton>
      </div>

      <div v-if="loadingRecentAudit" class="text-muted">{{ $t('common.loading') }}</div>
      <div v-else-if="recentAuditEvents.length === 0" class="text-muted">
        {{ $t('lowCode.noAuditEventsFound') }}
      </div>
      <div v-else class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>{{ $t('lowCode.createdAt') }}</th>
              <th>{{ $t('lowCode.action') }}</th>
              <th>{{ $t('lowCode.actor') }}</th>
              <th>{{ $t('lowCode.changedFields') }}</th>
              <th>{{ $t('lowCode.newValues') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in recentAuditEvents" :key="item.id">
              <td>{{ formatLowCodeDate(item.created_at) }}</td>
              <td>{{ item.action }}</td>
              <td>{{ item.actor || '—' }}</td>
              <td>{{ formatChangedFields(item.changed_fields) }}</td>
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

.low-code-hub__notice {
  padding: 1rem 1.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid transparent;
}

.low-code-hub__notice p {
  margin: 0.375rem 0 0;
  font-size: 0.875rem;
}

.low-code-hub__notice--warn {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.lookup-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
  align-items: end;
}

.lookup-form__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.demo-hint {
  margin: 1rem 0 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.audit-actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.audit-link {
  color: var(--color-primary);
  font-weight: 500;
  text-decoration: none;
}

.audit-link:hover {
  text-decoration: underline;
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
