<script setup lang="ts">
import {
  createEmptyDraftTemplate,
  draftToPayload,
  draftToPreviewModel,
  type DraftFormTemplateDraft,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const router = useRouter()
const { createDraftFormTemplate, getAdminFormTemplateErrorMessage, isApiUnavailableError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const draft = ref<DraftFormTemplateDraft>(createEmptyDraftTemplate())
const editorRef = ref<{ validate: () => unknown[] } | null>(null)
const saving = ref(false)
const builderTab = ref<'editor' | 'preview'>('editor')

const previewModel = computed(() => draftToPreviewModel(draft.value))

async function saveDraft() {
  if (!hasTenant.value) return
  const issues = editorRef.value?.validate() ?? []
  if (issues.length) {
    pushToast('error', t('lowCode.fixValidationIssues'))
    return
  }

  saving.value = true
  try {
    const result = await createDraftFormTemplate(draftToPayload(draft.value))
    pushToast('success', t('lowCode.draftSaved'))
    await router.push(`/low-code/admin/form-templates/${result.id}`)
  } catch (error) {
    if (error instanceof TenantRequiredError) return
    if (!isApiUnavailableError(error)) {
      pushToast('error', getAdminFormTemplateErrorMessage(error))
    } else {
      pushToast('error', t('lowCode.serviceUnavailable'))
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="page-stack form-template-builder-page">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink to="/low-code/admin/form-templates">{{ $t('lowCode.formTemplateAdmin') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.createDraft') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.createDraft')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/low-code/admin/form-templates')">
          {{ $t('lowCode.backToTemplates') }}
        </UiButton>
      </template>
    </UiPageHeader>

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
      </div>

      <div class="form-builder__layout">
        <div
          class="form-builder__editor"
          :class="{ 'form-builder__panel--hidden': builderTab !== 'editor' }"
          role="tabpanel"
        >
          <LowCodeFormTemplateEditor ref="editorRef" v-model="draft" />
        </div>

        <aside
          class="form-builder__preview"
          :class="{ 'form-builder__panel--hidden': builderTab !== 'preview' }"
          role="tabpanel"
        >
          <LowCodeFormTemplatePreview
            :template="previewModel"
            :title="$t('lowCode.formPreview')"
          />
        </aside>
      </div>
    </div>

    <div class="form-builder__sticky-actions">
      <UiButton :loading="saving" @click="saveDraft">{{ $t('lowCode.saveDraft') }}</UiButton>
      <UiButton variant="secondary" @click="$router.push('/low-code/admin/form-templates')">
        {{ $t('lowCode.backToTemplates') }}
      </UiButton>
    </div>
  </div>
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

.form-builder__tabs {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
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

@media (min-width: 1100px) {
  .form-builder__tabs {
    display: none;
  }

  .form-builder__layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(320px, 400px);
    gap: 1.5rem;
    align-items: start;
  }

  .form-builder__panel--hidden {
    display: block !important;
  }

  .form-builder__preview {
    position: sticky;
    top: 1rem;
  }
}

@media (max-width: 1099px) {
  .form-builder__panel--hidden {
    display: none;
  }
}
</style>
