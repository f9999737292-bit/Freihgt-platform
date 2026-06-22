<script setup lang="ts">
import { createEmptyDraftTemplate, draftToPayload, type DraftFormTemplateDraft } from '~/types/lowCode'
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
  <div class="page-stack">
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
          {{ $t('common.cancel') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <LowCodeFormTemplateEditor ref="editorRef" v-model="draft" />

    <div class="actions-row">
      <UiButton :loading="saving" @click="saveDraft">{{ $t('lowCode.saveDraft') }}</UiButton>
      <UiButton variant="secondary" @click="$router.push('/low-code/admin/form-templates')">
        {{ $t('common.cancel') }}
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

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
</style>
