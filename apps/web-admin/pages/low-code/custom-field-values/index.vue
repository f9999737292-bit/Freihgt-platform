<script setup lang="ts">

import {

  DEMO_EMPTY_ENTITY_REFS,

  DEMO_ENTITY_REFS,

  LOW_CODE_ENTITY_TYPES,

  PREVIEW_ENTITY_STATUS_PRESETS,

  isMigrationAuditEvent,

  buildLowCodeAuditLink,

  type AuditEventItem,

  type LowCodeEntityType,

} from '~/types/lowCode'



definePageMeta({ middleware: 'auth', layout: 'default' })



const { resolveDemoEntityId, resolveDemoEmptyEntityId, resolveEntityStatus, resolveActiveTemplateCode, listAuditEvents } = useLowCodeApi()

const { hasTenant } = useTenantContext()

const { pushToast } = useToast()

const { t } = useI18n()



const form = reactive({

  entity_type: 'TRANSPORT_ORDER' as LowCodeEntityType,

  entity_id: '',

  entity_status: '',

})



const loadedEntity = reactive({

  entity_type: '' as LowCodeEntityType | '',

  entity_id: '',

})



const loaded = ref(false)

const resolvingDemo = ref(false)

const fetchingStatus = ref(false)

const panelKey = ref(0)

const recentAuditEvents = ref<AuditEventItem[]>([])

const loadingRecentAudit = ref(false)

const showAuditLink = ref(false)

const migrationModalOpen = ref(false)
const batchWizardOpen = ref(false)
const activeTemplateCode = ref<string | null>(null)
const resolvingTemplateCode = ref(false)



const entityTypeOptions = computed(() =>

  LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),

)



const demoRefHint = computed(() => DEMO_ENTITY_REFS[form.entity_type])

const demoEmptyRefHint = computed(() => DEMO_EMPTY_ENTITY_REFS[form.entity_type] ?? null)

const entityStatusPresetOptions = computed(() => {
  const presets = PREVIEW_ENTITY_STATUS_PRESETS[form.entity_type] ?? []
  return [
    { label: t('lowCode.previewEntityStatusAuto'), value: '' },
    ...presets.map((value) => ({ label: value, value })),
  ]
})

const auditLogLink = computed(() => {

  if (!loadedEntity.entity_id) return '/low-code/audit'

  return buildLowCodeAuditLink({

    entity_type: loadedEntity.entity_type,

    entity_id: loadedEntity.entity_id,

  })

})

const migrationHistoryLink = computed(() => {

  if (!loadedEntity.entity_id) return '/low-code/audit?category=migrations'

  return buildLowCodeAuditLink({

    entity_type: loadedEntity.entity_type,

    entity_id: loadedEntity.entity_id,

    category: 'migrations',

  })

})

const latestMigrationEvent = computed(() =>
  recentAuditEvents.value.find((item) => isMigrationAuditEvent(item)) ?? null,
)

const canOpenMigration = computed(
  () => loaded.value && !!loadedEntity.entity_id && !!activeTemplateCode.value,
)

const canOpenBatchMigration = computed(
  () => hasTenant.value && !!form.entity_type && !!activeTemplateCode.value,
)

async function resolveTemplateCodeForType(entityType: LowCodeEntityType) {
  if (!hasTenant.value) {
    activeTemplateCode.value = null
    return
  }
  resolvingTemplateCode.value = true
  try {
    activeTemplateCode.value = await resolveActiveTemplateCode(entityType)
  } catch {
    activeTemplateCode.value = null
  } finally {
    resolvingTemplateCode.value = false
  }
}

async function resolveTemplateCodeForEntity() {
  if (!loadedEntity.entity_id) {
    await resolveTemplateCodeForType(form.entity_type)
    return
  }
  await resolveTemplateCodeForType(loadedEntity.entity_type)
}

function openMigrationModal() {
  if (!canOpenMigration.value) return
  migrationModalOpen.value = true
}

function openBatchWizard() {
  if (!canOpenBatchMigration.value || !activeTemplateCode.value) return
  batchWizardOpen.value = true
}

async function onBatchMigrationExecuted(migratedEntityIds: string[]) {
  showAuditLink.value = true
  if (
    loaded.value
    && loadedEntity.entity_id
    && migratedEntityIds.some((id) => id.toLowerCase() === loadedEntity.entity_id.toLowerCase())
  ) {
    panelKey.value += 1
  }
  await loadRecentAuditEvents(undefined)
}

async function onMigrationCompleted() {
  showAuditLink.value = true
  panelKey.value += 1
  await loadRecentAuditEvents(undefined)
}



function markLoaded() {

  loadedEntity.entity_type = form.entity_type

  loadedEntity.entity_id = form.entity_id.trim()

  loaded.value = true

  panelKey.value += 1

  showAuditLink.value = false

  recentAuditEvents.value = []

  void resolveTemplateCodeForEntity()

}

async function refreshEntityStatus(showToastOnMissing = false) {
  if (!hasTenant.value || !form.entity_id.trim()) return false
  fetchingStatus.value = true
  try {
    const status = await resolveEntityStatus(form.entity_type, form.entity_id.trim())
    if (status) {
      form.entity_status = status
      if (loaded.value && loadedEntity.entity_id === form.entity_id.trim()) {
        panelKey.value += 1
      }
      return true
    }
    if (showToastOnMissing) {
      pushToast('error', t('lowCode.previewEntityStatusFetchFailed'))
    }
    return false
  } catch (error) {
    if (showToastOnMissing) {
      pushToast('error', error instanceof Error ? error.message : t('lowCode.previewEntityStatusFetchFailed'))
    }
    return false
  } finally {
    fetchingStatus.value = false
  }
}

function applyPreviewEntityStatus() {
  if (loaded.value) panelKey.value += 1
}

async function loadRecentAuditEvents(action?: string) {

  if (!hasTenant.value || !loadedEntity.entity_id) return

  loadingRecentAudit.value = true

  try {

    const data = await listAuditEvents({

      entity_type: loadedEntity.entity_type,

      entity_id: loadedEntity.entity_id,

      action,

      limit: 10,

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

  await refreshEntityStatus()
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
    await refreshEntityStatus()
    markLoaded()
    await loadRecentAuditEvents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('lowCode.demoResolveFailed'))
  } finally {
    resolvingDemo.value = false
  }
}


async function useDemoEmptyEntity() {
  if (!hasTenant.value || !demoEmptyRefHint.value) return
  resolvingDemo.value = true
  try {
    const id = await resolveDemoEmptyEntityId(form.entity_type)
    if (!id) {
      pushToast('error', t('lowCode.demoEntityNotFound', { ref: demoEmptyRefHint.value }))
      return
    }

    form.entity_id = id
    await refreshEntityStatus()
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

const route = useRoute()

onMounted(async () => {
  const q = route.query
  if (typeof q.entity_type === 'string' && q.entity_type) {
    form.entity_type = q.entity_type as LowCodeEntityType
  }
  if (typeof q.entity_id === 'string') form.entity_id = q.entity_id
  if (typeof q.entity_status === 'string') form.entity_status = q.entity_status
  await resolveTemplateCodeForType(form.entity_type)
  if (hasTenant.value && form.entity_id.trim()) {
    await loadValues()
  }
})

watch(
  () => form.entity_type,
  (entityType) => {
    void resolveTemplateCodeForType(entityType)
  },
)

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
        <UiSelect
          v-model="form.entity_status"
          :label="$t('lowCode.previewEntityStatusLabel')"
          :options="entityStatusPresetOptions"
          @update:model-value="applyPreviewEntityStatus"
        />
        <UiInput
          v-model="form.entity_status"
          :label="$t('lowCode.previewEntityStatusCustom')"
          :placeholder="$t('lowCode.previewEntityStatusPlaceholder')"
          @change="applyPreviewEntityStatus"
        />
        <div class="lookup-form__actions">
          <UiButton @click="loadValues">{{ $t('lowCode.loadValues') }}</UiButton>
          <UiButton variant="secondary" :loading="resolvingDemo" @click="useDemoEntity">
            {{ $t('lowCode.useDemoEntity') }}
          </UiButton>
          <UiButton
            v-if="demoEmptyRefHint"
            variant="secondary"
            :loading="resolvingDemo"
            @click="useDemoEmptyEntity"
          >
            {{ $t('lowCode.useDemoEmptyEntity') }}
          </UiButton>
          <UiButton
            variant="secondary"
            :loading="fetchingStatus"
            :disabled="!form.entity_id.trim()"
            @click="refreshEntityStatus(true)"
          >
            {{ $t('lowCode.fetchEntityStatus') }}
          </UiButton>
          <UiButton

            v-if="loaded && loadedEntity.entity_id"

            variant="secondary"

            @click="reloadPanel"

          >

            {{ $t('lowCode.reloadValues') }}

          </UiButton>

          <span
            v-if="loaded && loadedEntity.entity_id"
            class="migration-entry"
            :title="canOpenMigration ? '' : $t('lowCode.migrationSelectEntityFirst')"
          >
            <UiButton
              variant="secondary"
              :disabled="!canOpenMigration"
              :loading="resolvingTemplateCode"
              @click="openMigrationModal"
            >
              {{ $t('lowCode.migrateToActiveTemplate') }}
            </UiButton>
            <span v-if="!canOpenMigration" class="migration-entry__hint">
              {{ $t('lowCode.migrationSelectEntityFirst') }}
            </span>
          </span>

          <span
            class="migration-entry"
            :title="canOpenBatchMigration ? '' : $t('lowCode.batchMigrationSelectContextFirst')"
          >
            <UiButton
              variant="secondary"
              :disabled="!canOpenBatchMigration"
              :loading="resolvingTemplateCode"
              @click="openBatchWizard"
            >
              {{ $t('lowCode.batchMigration') }}
            </UiButton>
            <span v-if="!canOpenBatchMigration" class="migration-entry__hint">
              {{ $t('lowCode.batchMigrationSelectContextFirst') }}
            </span>
          </span>

        </div>

      </div>

      <p class="demo-hint">
        {{ $t('lowCode.demoHint', { entityType: form.entity_type, demoRef: demoRefHint }) }}
      </p>
      <p v-if="demoEmptyRefHint" class="demo-hint">
        {{ $t('lowCode.demoEmptyHint', { demoRef: demoEmptyRefHint }) }}
      </p>
      <p class="demo-hint">{{ $t('lowCode.previewEntityStatusHint') }}</p>
    </UiCard>

    <LowCodeCustomFieldsPanel
      v-if="loaded && loadedEntity.entity_id"
      :key="panelKey"
      :entity-type="loadedEntity.entity_type"
      :entity-id="loadedEntity.entity_id"
      :entity-status="form.entity_status || null"
      editable
      @saved="onPanelSaved"
    />

    <LowCodeMigrationPreviewModal
      v-if="loaded && loadedEntity.entity_id && activeTemplateCode"
      :open="migrationModalOpen"
      :entity-type="loadedEntity.entity_type"
      :entity-id="loadedEntity.entity_id"
      :template-code="activeTemplateCode"
      @close="migrationModalOpen = false"
      @migrated="onMigrationCompleted"
    />

    <LowCodeBatchMigrationWizard
      v-if="activeTemplateCode"
      :open="batchWizardOpen"
      :entity-type="form.entity_type"
      :template-code="activeTemplateCode"
      :initial-entity-id="loadedEntity.entity_id || form.entity_id.trim() || undefined"
      @close="batchWizardOpen = false"
      @executed="onBatchMigrationExecuted"
    />

    <UiCard v-if="loaded && loadedEntity.entity_id && latestMigrationEvent">
      <template #header>{{ $t('lowCode.auditLatestMigration') }}</template>
      <LowCodeMigrationAuditCard :event="latestMigrationEvent" compact />
      <div class="audit-actions">
        <NuxtLink :to="migrationHistoryLink" class="audit-link">
          {{ $t('lowCode.auditViewMigrationHistory') }}
        </NuxtLink>
      </div>
    </UiCard>

    <UiCard v-if="loaded && loadedEntity.entity_id && (showAuditLink || recentAuditEvents.length)">

      <template #header>{{ $t('lowCode.auditEvents') }}</template>

      <div class="audit-actions">

        <NuxtLink :to="auditLogLink" class="audit-link">{{ $t('lowCode.viewAuditLog') }}</NuxtLink>

        <NuxtLink :to="migrationHistoryLink" class="audit-link">{{ $t('lowCode.auditViewMigrationHistory') }}</NuxtLink>

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

      <div v-else class="audit-event-list">
        <LowCodeAuditEventCard
          v-for="item in recentAuditEvents"
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

  align-items: center;

}

.migration-entry {
  display: inline-flex;
  flex-direction: column;
  gap: 0.25rem;
}

.migration-entry__hint {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  max-width: 220px;
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

.audit-event-list {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
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

