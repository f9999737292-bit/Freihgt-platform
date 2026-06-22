<script setup lang="ts">
import {
  adminDetailToDraft,
  draftToPayload,
  formatLowCodeDate,
  type AdminFormTemplateDetail,
  type DraftFormTemplateDraft,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const templateId = computed(() => String(route.params.id ?? ''))

const {
  getAdminFormTemplate,
  updateDraftFormTemplate,
  publishDraftFormTemplate,
  getAdminFormTemplateErrorMessage,
  isApiUnavailableError,
} = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const template = ref<AdminFormTemplateDetail | null>(null)
const draft = ref<DraftFormTemplateDraft | null>(null)
const editorRef = ref<{ validate: () => unknown[] } | null>(null)
const loading = ref(true)
const loadFailed = ref(false)
const saving = ref(false)
const publishing = ref(false)
const publishModalOpen = ref(false)

const isDraft = computed(() => template.value?.status === 'DRAFT')
const isReadOnly = computed(() => !isDraft.value)

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
  } catch (error) {
    template.value = null
    draft.value = null
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
    publishModalOpen.value = false
    pushToast('success', t('lowCode.draftPublished'))
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    pushToast('error', getAdminFormTemplateErrorMessage(error))
  } finally {
    publishing.value = false
  }
}

onMounted(load)
watch(templateId, load)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink to="/low-code/admin/form-templates">{{ $t('lowCode.formTemplateAdmin') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ template?.code || templateId }}</span>
    </nav>

    <UiPageHeader :title="template?.name || $t('lowCode.templateDetails')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/low-code/admin/form-templates')">
          {{ $t('common.back') }}
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
          <div><dt>{{ $t('lowCode.version') }}</dt><dd>{{ template.version }}</dd></div>
          <div><dt>{{ $t('lowCode.publishedAt') }}</dt><dd>{{ formatLowCodeDate(template.published_at) }}</dd></div>
        </dl>
      </UiCard>

      <div
        v-if="template.status === 'PUBLISHED'"
        class="notice notice--warn"
      >
        <strong>{{ $t('lowCode.publishedTemplatesCannotBeEdited') }}</strong>
      </div>

      <div
        v-else-if="template.status === 'ARCHIVED'"
        class="notice notice--info"
      >
        <strong>{{ $t('lowCode.archivedTemplatesReadOnly') }}</strong>
      </div>

      <LowCodeFormTemplateEditor
        ref="editorRef"
        v-model="draft"
        :readonly="isReadOnly"
        lock-identity
      />

      <div v-if="isDraft" class="actions-row">
        <UiButton :loading="saving" @click="saveDraft">{{ $t('lowCode.saveDraft') }}</UiButton>
        <UiButton variant="secondary" :loading="publishing" @click="publishModalOpen = true">
          {{ $t('lowCode.publish') }}
        </UiButton>
      </div>
    </template>
  </div>

  <UiModal
    :open="publishModalOpen"
    :title="$t('lowCode.publish')"
    @close="publishModalOpen = false"
  >
    <p>{{ $t('lowCode.publishConfirmMessage') }}</p>
    <template #footer>
      <UiButton variant="secondary" @click="publishModalOpen = false">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="publishing" @click="confirmPublish">{{ $t('lowCode.publish') }}</UiButton>
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

.notice--info {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #1e3a8a;
}

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
</style>
