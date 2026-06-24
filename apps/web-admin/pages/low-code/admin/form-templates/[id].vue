<script setup lang="ts">
import {
  adminDetailToDraft,
  compareFormTemplates,
  adminDetailToCompareInput,
  draftToCompareInput,
  draftToPayload,
  draftToPreviewModel,
  formatLowCodeDate,
  formTemplateDetailToPreview,
  hasFormTemplateCompareChanges,
  type AdminFormTemplateDetail,
  type DraftFormTemplateDraft,
  type TemplateExportEnvelope,
} from '~/types/lowCode'
import {
  buildTemplateExportFilename,
  downloadJsonFile,
  formatTemplateExportJson,
} from '~/utils/lowCodeTemplateImportExport'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: ['auth', 'low-code-admin'], layout: 'default' })

const route = useRoute()
const templateId = computed(() => String(route.params.id ?? ''))

const {
  getAdminFormTemplate,
  listActiveFormTemplates,
  updateDraftFormTemplate,
  publishDraftFormTemplate,
  clonePublishedTemplateToDraft,
  exportFormTemplate,
  getAdminFormTemplateErrorMessage,
  isApiUnavailableError,
} = useLowCodeApi()
const router = useRouter()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()
const { canExportTemplates } = useLowCodePermissions()

const template = ref<AdminFormTemplateDetail | null>(null)
const draft = ref<DraftFormTemplateDraft | null>(null)
const basePublished = ref<AdminFormTemplateDetail | null>(null)
const isActivePublished = ref(false)
const editorRef = ref<{ validate: () => unknown[] } | null>(null)
const loading = ref(true)
const basePublishedLoading = ref(false)
const loadFailed = ref(false)
const saving = ref(false)
const publishing = ref(false)
const cloning = ref(false)
const publishModalOpen = ref(false)
const builderTab = ref<'editor' | 'preview' | 'compare'>('editor')
const exporting = ref(false)
const exportError = ref('')
const exportEnvelope = ref<TemplateExportEnvelope | null>(null)
const exportCopied = ref(false)

const isDraft = computed(() => template.value?.status === 'DRAFT')
const isReadOnly = computed(() => !isDraft.value)

const previewModel = computed(() => {
  if (isDraft.value && draft.value) {
    return draftToPreviewModel(draft.value)
  }
  if (template.value) {
    return formTemplateDetailToPreview(template.value)
  }
  return null
})

const compareResult = computed(() => {
  if (!basePublished.value || !draft.value) return null
  return compareFormTemplates(
    adminDetailToCompareInput(basePublished.value),
    draftToCompareInput(draft.value, template.value?.version ?? 0),
  )
})

const hasCompareChanges = computed(() =>
  compareResult.value ? hasFormTemplateCompareChanges(compareResult.value) : false,
)

async function loadBasePublished() {
  if (!template.value || template.value.status !== 'DRAFT') {
    basePublished.value = null
    return
  }

  basePublishedLoading.value = true
  try {
    const { items } = await listActiveFormTemplates({
      entity_type: template.value.entity_type,
      code: template.value.code,
    })
    const active = items[0]
    if (!active) {
      basePublished.value = null
      return
    }
    basePublished.value = await getAdminFormTemplate(active.id)
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    basePublished.value = null
  } finally {
    basePublishedLoading.value = false
  }
}

async function loadActiveStatus() {
  if (!template.value || template.value.status !== 'PUBLISHED') {
    isActivePublished.value = false
    return
  }

  try {
    const { items } = await listActiveFormTemplates({
      entity_type: template.value.entity_type,
      code: template.value.code,
    })
    isActivePublished.value = items.some((item) => item.id === template.value!.id)
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    isActivePublished.value = false
  }
}

async function load() {
  if (!hasTenant.value || !templateId.value) {
    loading.value = false
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const detail = await getAdminFormTemplate(templateId.value)
    template.value = detail
    draft.value = adminDetailToDraft(detail)
    await Promise.all([loadBasePublished(), loadActiveStatus()])
  } catch (error) {
    template.value = null
    draft.value = null
    basePublished.value = null
    isActivePublished.value = false
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', getAdminFormTemplateErrorMessage(error))
    }
  } finally {
    loading.value = false
  }
}

async function saveDraft() {
  if (!draft.value || !isDraft.value) return
  const issues = editorRef.value?.validate() ?? []
  if (issues.length) {
    pushToast('error', t('lowCode.fixValidationIssues'))
    return
  }

  saving.value = true
  try {
    const updated = await updateDraftFormTemplate(templateId.value, draftToPayload(draft.value))
    template.value = updated
    draft.value = adminDetailToDraft(updated)
    await loadBasePublished()
    pushToast('success', t('lowCode.draftSaved'))
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    pushToast('error', getAdminFormTemplateErrorMessage(error))
  } finally {
    saving.value = false
  }
}

async function confirmPublish() {
  publishing.value = true
  try {
    const updated = await publishDraftFormTemplate(templateId.value)
    template.value = updated
    draft.value = adminDetailToDraft(updated)
    basePublished.value = null
    publishModalOpen.value = false
    builderTab.value = 'editor'
    pushToast('success', t('lowCode.draftPublished'))
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    pushToast('error', getAdminFormTemplateErrorMessage(error))
  } finally {
    publishing.value = false
  }
}

async function cloneToDraft() {
  if (!template.value || template.value.status !== 'PUBLISHED') return
  cloning.value = true
  try {
    const result = await clonePublishedTemplateToDraft(templateId.value)
    pushToast('success', t('lowCode.draftCreatedFromPublished'))
    await router.push(`/low-code/admin/form-templates/${result.id}`)
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    pushToast('error', getAdminFormTemplateErrorMessage(error))
  } finally {
    cloning.value = false
  }
}

const exportJsonText = computed(() =>
  exportEnvelope.value ? formatTemplateExportJson(exportEnvelope.value) : '',
)

async function runExport() {
  if (!canExportTemplates() || exporting.value) return
  exporting.value = true
  exportError.value = ''
  try {
    exportEnvelope.value = await exportFormTemplate(templateId.value)
    pushToast('success', t('lowCode.templateExportSuccess'))
  } catch (error) {
    exportEnvelope.value = null
    exportError.value = getAdminFormTemplateErrorMessage(error)
    pushToast('error', exportError.value)
  } finally {
    exporting.value = false
  }
}

function downloadExport() {
  if (!exportEnvelope.value) return
  downloadJsonFile(buildTemplateExportFilename(exportEnvelope.value), exportJsonText.value)
}

async function copyExportJson() {
  if (!exportJsonText.value) return
  try {
    await navigator.clipboard.writeText(exportJsonText.value)
    exportCopied.value = true
    pushToast('success', t('lowCode.templateExportJsonCopied'))
    window.setTimeout(() => {
      exportCopied.value = false
    }, 2000)
  } catch {
    pushToast('error', t('common.error'))
  }
}

onMounted(load)
watch(templateId, () => {
  exportEnvelope.value = null
  exportError.value = ''
  load()
})
</script>

<template>
  <div class="page-stack form-template-builder-page">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink to="/low-code/admin/form-templates">{{ $t('lowCode.formTemplateAdmin') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ template?.code || templateId }}</span>
    </nav>

    <UiPageHeader :title="template?.name || $t('lowCode.templateDetails')">
      <template #actions>
        <UiButton
          v-if="canExportTemplates()"
          variant="secondary"
          :loading="exporting"
          :disabled="exporting"
          @click="runExport"
        >
          {{ $t('lowCode.templateExportJson') }}
        </UiButton>
        <UiButton variant="secondary" @click="$router.push('/low-code/admin/form-templates')">
          {{ $t('lowCode.backToTemplates') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>

    <CommonApiUnavailableState
      v-else-if="loadFailed"
      :message="$t('lowCode.serviceUnavailable')"
      @retry="load"
    />

    <template v-else-if="template && draft">
      <UiCard>
        <template #header>{{ $t('lowCode.templateMetadata') }}</template>
        <dl class="metadata-grid">
          <div><dt>{{ $t('lowCode.entityType') }}</dt><dd>{{ template.entity_type }}</dd></div>
          <div><dt>{{ $t('lowCode.code') }}</dt><dd><code>{{ template.code }}</code></dd></div>
          <div><dt>{{ $t('common.status') }}</dt><dd><UiBadge :status="template.status" /></dd></div>
          <div v-if="template.status === 'PUBLISHED'">
            <dt>{{ $t('lowCode.versionStatus') }}</dt>
            <dd>
              <UiBadge
                v-if="isActivePublished"
                :status="$t('lowCode.activePublishedVersion')"
                tone="success"
              />
              <UiBadge
                v-else
                :status="$t('lowCode.olderPublishedVersion')"
                tone="warning"
              />
            </dd>
          </div>
          <div><dt>{{ $t('lowCode.version') }}</dt><dd>{{ template.version }}</dd></div>
          <div><dt>{{ $t('lowCode.publishedAt') }}</dt><dd>{{ formatLowCodeDate(template.published_at) }}</dd></div>
        </dl>
      </UiCard>

      <UiCard v-if="exportEnvelope || exportError">
        <template #header>{{ $t('lowCode.templateExportJson') }}</template>
        <p v-if="exportError" class="export-panel__error" role="alert">{{ exportError }}</p>
        <template v-else-if="exportEnvelope">
          <div class="export-panel__actions">
            <UiButton size="sm" variant="secondary" @click="downloadExport">
              {{ $t('lowCode.templateExportDownloadJson') }}
            </UiButton>
            <UiButton size="sm" variant="secondary" @click="copyExportJson">
              {{ exportCopied ? $t('lowCode.templateExportJsonCopied') : $t('lowCode.templateExportCopyJson') }}
            </UiButton>
          </div>
          <details class="export-panel__preview" open>
            <summary>{{ $t('lowCode.templateExportPreview') }}</summary>
            <pre class="export-panel__pre">{{ exportJsonText }}</pre>
          </details>
        </template>
      </UiCard>

      <div
        v-if="template.status === 'PUBLISHED' && !isActivePublished"
        class="notice notice--warn"
      >
        <strong>{{ $t('lowCode.olderPublishedVersionNotice') }}</strong>
        <p class="notice__text">{{ $t('lowCode.newFormsShouldUseActiveVersion') }}</p>
      </div>

      <div
        v-if="template.status === 'PUBLISHED'"
        class="notice notice--warn"
      >
        <strong>{{ $t('lowCode.publishedTemplatesCannotBeEditedDirectly') }}</strong>
        <p class="notice__text">{{ $t('lowCode.publishedTemplatesCannotBeEdited') }}</p>
        <UiButton class="notice__action" :loading="cloning" @click="cloneToDraft">
          {{ $t('lowCode.cloneToDraft') }}
        </UiButton>
      </div>

      <div
        v-else-if="template.status === 'ARCHIVED'"
        class="notice notice--info"
      >
        <strong>{{ $t('lowCode.archivedTemplatesReadOnly') }}</strong>
      </div>

      <div
        v-else-if="isDraft && !basePublishedLoading && !basePublished"
        class="notice notice--info"
      >
        {{ $t('lowCode.noPublishedBaseTemplateFound') }}
      </div>

      <div class="form-builder">
        <div class="form-builder__tabs" role="tablist">
          <button
            type="button"
            role="tab"
            class="form-builder__tab"
            :class="{ 'form-builder__tab--active': builderTab === 'editor' }"
            :aria-selected="builderTab === 'editor'"
            @click="builderTab = 'editor'"
          >
            {{ $t('lowCode.editor') }}
          </button>
          <button
            type="button"
            role="tab"
            class="form-builder__tab"
            :class="{ 'form-builder__tab--active': builderTab === 'preview' }"
            :aria-selected="builderTab === 'preview'"
            @click="builderTab = 'preview'"
          >
            {{ $t('lowCode.preview') }}
          </button>
          <button
            v-if="isDraft"
            type="button"
            role="tab"
            class="form-builder__tab"
            :class="{ 'form-builder__tab--active': builderTab === 'compare' }"
            :aria-selected="builderTab === 'compare'"
            @click="builderTab = 'compare'"
          >
            {{ $t('lowCode.compare') }}
          </button>
        </div>

        <div
          v-if="builderTab === 'compare'"
          class="form-builder__compare"
          role="tabpanel"
        >
          <UiCard>
            <template #header>{{ $t('lowCode.versionCompare') }}</template>
            <div v-if="basePublishedLoading" class="text-muted">{{ $t('common.loading') }}</div>
            <LowCodeFormTemplateDiff
              v-else
              :base-template="basePublished"
              :draft-template="draft"
              :draft-version="template.version"
            />
          </UiCard>
        </div>

        <div
          v-else
          class="form-builder__layout"
          :class="{ 'form-builder__layout--split': builderTab === 'editor' }"
        >
          <div
            class="form-builder__editor"
            :class="{ 'form-builder__panel--hidden': builderTab !== 'editor' }"
            role="tabpanel"
          >
            <LowCodeFormTemplateEditor
              ref="editorRef"
              v-model="draft"
              :readonly="isReadOnly"
              lock-identity
            />
          </div>

          <aside
            class="form-builder__preview"
            :class="{ 'form-builder__panel--hidden': builderTab !== 'preview' && builderTab !== 'editor' }"
            role="tabpanel"
          >
            <LowCodeFormTemplatePreview
              :template="previewModel"
              :title="$t('lowCode.formPreview')"
            />
          </aside>
        </div>
      </div>

      <div v-if="isDraft" class="form-builder__sticky-actions">
        <UiButton :loading="saving" @click="saveDraft">{{ $t('lowCode.saveDraft') }}</UiButton>
        <UiButton variant="secondary" :loading="publishing" @click="publishModalOpen = true">
          {{ $t('lowCode.publish') }}
        </UiButton>
        <UiButton variant="secondary" @click="$router.push('/low-code/admin/form-templates')">
          {{ $t('lowCode.backToTemplates') }}
        </UiButton>
      </div>
    </template>
  </div>

  <UiModal
    :open="publishModalOpen"
    :title="$t('lowCode.publishReview')"
    @close="publishModalOpen = false"
  >
    <div class="publish-review">
      <dl class="publish-review__meta">
        <div><dt>{{ $t('lowCode.code') }}</dt><dd><code>{{ template?.code }}</code></dd></div>
        <div><dt>{{ $t('lowCode.draftVersion') }}</dt><dd>{{ template?.version ?? '—' }}</dd></div>
        <div>
          <dt>{{ $t('lowCode.basePublishedVersion') }}</dt>
          <dd>{{ basePublished?.version ?? '—' }}</dd>
        </div>
      </dl>

      <LowCodeFormTemplateDiff
        :base-template="basePublished"
        :draft-template="draft"
        :draft-version="template?.version ?? 0"
        compact
        :show-empty-state="!!basePublished"
      />

      <p v-if="basePublished && !hasCompareChanges" class="publish-review__warn">
        {{ $t('lowCode.noChangesDetected') }}
      </p>

      <p class="publish-review__warn publish-review__warn--primary">
        {{ $t('lowCode.publishPublicApiWarning') }}
      </p>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="publishModalOpen = false">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="publishing" @click="confirmPublish">{{ $t('lowCode.publishThisDraft') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  flex-wrap: wrap;
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.metadata-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1rem;
  margin: 0;
}

.metadata-grid div {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.metadata-grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.metadata-grid dd {
  margin: 0;
}

.notice {
  padding: 1rem 1.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid transparent;
}

.notice--warn {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.notice__text {
  margin: 0.5rem 0 0;
  font-size: 0.875rem;
}

.notice__action {
  margin-top: 0.75rem;
}

.notice--info {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #1e3a8a;
}

.form-builder__tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.form-builder__tab {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  border-radius: var(--radius-md);
  padding: 0.375rem 0.875rem;
  font-size: 0.875rem;
  cursor: pointer;
}

.form-builder__tab--active {
  border-color: var(--color-primary, #2563eb);
  background: #eff6ff;
  color: #1d4ed8;
  font-weight: 600;
}

.form-builder__layout {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-builder__preview {
  min-width: 0;
}

.form-builder__compare {
  min-width: 0;
}

.form-builder__sticky-actions {
  position: sticky;
  bottom: 0;
  z-index: 10;
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  padding: 0.875rem 0;
  margin-top: 0.5rem;
  background: linear-gradient(to top, var(--color-bg, #fff) 70%, transparent);
}

.publish-review {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.publish-review__meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 0.75rem;
  margin: 0;
}

.publish-review__meta div {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.publish-review__meta dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.publish-review__meta dd {
  margin: 0;
}

.publish-review__warn {
  margin: 0;
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  background: #fffbeb;
  color: #92400e;
  font-size: 0.875rem;
}

.publish-review__warn--primary {
  background: #eff6ff;
  color: #1e3a8a;
}

@media (min-width: 1100px) {
  .form-builder__layout--split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 400px);
    gap: 1.5rem;
    align-items: start;
  }

  .form-builder__layout--split .form-builder__preview {
    position: sticky;
    top: 1rem;
  }

  .form-builder__layout--split .form-builder__panel--hidden {
    display: block !important;
  }
}

@media (max-width: 1099px) {
  .form-builder__panel--hidden {
    display: none;
  }
}

.export-panel__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.export-panel__preview summary {
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.export-panel__pre {
  margin: 0.5rem 0 0;
  padding: 0.75rem;
  max-height: 320px;
  overflow: auto;
  font-size: 0.75rem;
  border-radius: 0.375rem;
  background: var(--color-bg-muted, #f4f4f5);
  white-space: pre-wrap;
  word-break: break-word;
}

.export-panel__error {
  margin: 0;
  color: var(--color-danger, #dc2626);
  font-size: 0.875rem;
}
</style>
