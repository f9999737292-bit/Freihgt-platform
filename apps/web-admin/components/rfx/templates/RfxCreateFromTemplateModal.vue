<script setup lang="ts">
import {
  RFX_CATEGORIES,
  RFX_TYPES,
  emptyCreateRfxForm,
  hasFormErrors,
  replaceRfxFormErrors,
  toRFC3339,
  validateCreateRfxForm,
  type RfxFormErrors,
} from '~/types/rfx'
import type { RfxTemplateRecord } from '~/types/rfx-template'
import type { RfxCloneEventFromTemplateResponse } from '~/types/rfx-version-lifecycle'
import { resolveI18nMapValue } from '~/utils/rfxTemplateI18n'
import { IdempotentOperation } from '~/utils/idempotentOperation'
import { formatRfxApiError } from '~/utils/rfxApiError'
import {
  filterCloneableTemplateVersions,
  selectDefaultCloneVersionId,
} from '~/utils/rfxTemplateLifecycle'

const props = defineProps<{
  open: boolean
  templates: RfxTemplateRecord[]
}>()

const emit = defineEmits<{ close: []; created: [eventId: string] }>()

const lifecycleApi = useRfxVersionLifecycleApi(ref('clone-from-template'))
const { getTemplate } = useRfxTemplateApi()
const { loadAuthorizedOwnerCompanies } = useRfxOwnerCompanies()
const { canManageRfxTemplates } = useRfxBuyerPermissions()
const { pushToast } = useToast()
const { t, locale } = useI18n()

const saving = ref(false)
const selectedTemplateId = ref('')
const selectedVersionId = ref('')
const cloneResult = ref<RfxCloneEventFromTemplateResponse | null>(null)
const form = reactive(emptyCreateRfxForm())
const errors = reactive<RfxFormErrors>({})
const ownerOptions = ref<Array<{ label: string; value: string }>>([])

const versionsLoading = ref(false)
const versionsLoadFailed = ref(false)
const versionCache = ref(new Map<string, ReturnType<typeof filterCloneableTemplateVersions>>())

const cloneOp = new IdempotentOperation('clone-tpl')
const previousFocus = ref<HTMLElement | null>(null)

const cloneableVersions = computed(() => {
  const templateId = selectedTemplateId.value
  if (!templateId) return []
  return versionCache.value.get(templateId) ?? []
})

const supersededWarning = computed(() => {
  const version = cloneableVersions.value.find((v) => v.id === selectedVersionId.value)
  return version?.status === 'SUPERSEDED'
})

const noCloneableVersions = computed(() =>
  Boolean(selectedTemplateId.value && !versionsLoading.value && cloneableVersions.value.length === 0),
)

watch(
  () => props.open,
  async (isOpen) => {
    if (!isOpen) return
    previousFocus.value = document.activeElement as HTMLElement | null
    Object.assign(form, emptyCreateRfxForm())
    replaceRfxFormErrors(errors, {})
    selectedTemplateId.value = ''
    selectedVersionId.value = ''
    cloneResult.value = null
    versionsLoadFailed.value = false
    cloneOp.reset()
    const owners = await loadAuthorizedOwnerCompanies()
    ownerOptions.value = owners.map((o) => ({ label: o.legal_name, value: o.id }))
  },
)

watch(selectedTemplateId, async (templateId) => {
  selectedVersionId.value = ''
  if (!templateId) return
  if (versionCache.value.has(templateId)) {
    selectedVersionId.value = selectDefaultCloneVersionId(versionCache.value.get(templateId)!) ?? ''
    return
  }
  versionsLoading.value = true
  versionsLoadFailed.value = false
  try {
    const detail = await getTemplate(templateId)
    const cloneable = filterCloneableTemplateVersions(detail)
    versionCache.value.set(templateId, cloneable)
    selectedVersionId.value = selectDefaultCloneVersionId(cloneable) ?? ''
  } catch {
    versionsLoadFailed.value = true
  } finally {
    versionsLoading.value = false
  }
})

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape' && !saving.value) closeModal()
}

function closeModal() {
  emit('close')
  previousFocus.value?.focus()
}

async function retryVersionLoad() {
  if (!selectedTemplateId.value) return
  versionCache.value.delete(selectedTemplateId.value)
  const id = selectedTemplateId.value
  selectedTemplateId.value = ''
  await nextTick()
  selectedTemplateId.value = id
}

function buildCloneBody() {
  return {
    template_version_id: selectedVersionId.value,
    rfx_number: form.rfx_number.trim(),
    rfx_type: form.rfx_type,
    category: form.category,
    title: form.title.trim(),
    description: form.description?.trim() || undefined,
    owner_company_id: form.owner_company_id,
    currency_code: form.currency_code?.trim() || undefined,
    valid_from: form.valid_from || undefined,
    valid_to: form.valid_to || undefined,
    response_deadline: form.response_deadline ? toRFC3339(form.response_deadline) : undefined,
  }
}

async function handleSubmit() {
  if (cloneResult.value) {
    emit('created', cloneResult.value.id)
    return
  }
  replaceRfxFormErrors(errors, validateCreateRfxForm(form))
  if (hasFormErrors(errors)) return
  if (!selectedVersionId.value) {
    pushToast('error', t('rfx.templates.clone.selectVersion'))
    return
  }
  if (cloneOp.isSubmitting()) return

  saving.value = true
  try {
    cloneResult.value = await cloneOp.execute(buildCloneBody(), (key, payload) =>
      lifecycleApi.cloneFromTemplate(payload, key),
    )
    pushToast('success', t('rfx.templates.clone.success'))
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    saving.value = false
  }
}

function templateLabel(tpl: RfxTemplateRecord) {
  return resolveI18nMapValue(tpl.name_i18n, locale.value) || tpl.template_code
}

const provenanceTemplateName = computed(() => {
  if (!cloneResult.value) return null
  const tpl = props.templates.find((item) => item.id === cloneResult.value!.source_template_id)
  return tpl ? templateLabel(tpl) : null
})
</script>

<template>
  <div
    v-if="open"
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    aria-labelledby="clone-modal-title"
    @keydown="handleEscape"
    @click.self="closeModal"
  >
    <form class="modal" @submit.prevent="handleSubmit">
      <h2 id="clone-modal-title">{{ $t('rfx.templates.clone.title') }}</h2>

      <template v-if="!cloneResult">
        <label class="modal__label">
          {{ $t('rfx.templates.clone.selectTemplate') }}
          <select v-model="selectedTemplateId" required>
            <option value="">{{ $t('common.select') }}</option>
            <option v-for="tpl in templates" :key="tpl.id" :value="tpl.id">
              {{ templateLabel(tpl) }}
            </option>
          </select>
        </label>

        <p v-if="selectedTemplateId && versionsLoading">{{ $t('rfx.templates.clone.versionsLoading') }}</p>
        <p v-else-if="versionsLoadFailed" class="modal__error">
          {{ $t('rfx.templates.clone.versionsLoadFailed') }}
          <button type="button" class="btn btn--link" @click="retryVersionLoad">{{ $t('common.retry') }}</button>
        </p>
        <p v-else-if="noCloneableVersions" class="modal__warning">{{ $t('rfx.templates.clone.noCloneableVersions') }}</p>

        <label v-else-if="selectedTemplateId && cloneableVersions.length" class="modal__label">
          {{ $t('rfx.templates.clone.selectVersion') }}
          <select v-model="selectedVersionId" required>
            <option v-for="ver in cloneableVersions" :key="ver.id" :value="ver.id">
              v{{ ver.version_number }} — {{ $t(`rfx.templates.versionStatus.${ver.status}`) }}
            </option>
          </select>
        </label>

        <p v-if="supersededWarning" class="modal__warning">{{ $t('rfx.templates.clone.supersededWarning') }}</p>

        <label class="modal__label">{{ $t('rfx.rfxNumber') }}<input v-model="form.rfx_number" required /></label>
        <p v-if="errors.rfx_number" class="modal__field-error">{{ errors.rfx_number }}</p>
        <label class="modal__label">{{ $t('rfx.title') }}<input v-model="form.title" required /></label>
        <p v-if="errors.title" class="modal__field-error">{{ errors.title }}</p>
        <label class="modal__label">
          {{ $t('rfx.type') }}
          <select v-model="form.rfx_type">
            <option v-for="opt in RFX_TYPES" :key="opt" :value="opt">{{ opt }}</option>
          </select>
        </label>
        <label class="modal__label">
          {{ $t('rfx.category') }}
          <select v-model="form.category">
            <option v-for="opt in RFX_CATEGORIES" :key="opt" :value="opt">{{ opt }}</option>
          </select>
        </label>
        <label class="modal__label">
          {{ $t('rfx.owner') }}
          <select v-model="form.owner_company_id" required>
            <option v-for="opt in ownerOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </label>
      </template>

      <template v-else>
        <p>{{ $t('rfx.templates.clone.confirmIntro') }}</p>
        <RfxProvenanceBanner
          :provenance="cloneResult"
          :template-name="provenanceTemplateName"
          :template-id="cloneResult.source_template_id"
          :can-link-template="canManageRfxTemplates()"
        />
      </template>

      <div class="modal__actions">
        <button type="button" class="btn btn--secondary" :disabled="saving" @click="closeModal">
          {{ $t('common.cancel') }}
        </button>
        <button
          type="submit"
          class="btn btn--primary"
          :disabled="saving || (!cloneResult && (versionsLoading || noCloneableVersions || !selectedVersionId))"
        >
          {{ cloneResult ? $t('rfx.templates.clone.openStudio') : $t('rfx.templates.clone.submit') }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: grid; place-items: center; z-index: 1000; padding: 1rem; }
.modal { background: #fff; border-radius: var(--radius-lg); padding: 1.5rem; width: min(100%, 36rem); max-height: 90vh; overflow: auto; display: flex; flex-direction: column; gap: 0.75rem; }
.modal__label { display: flex; flex-direction: column; gap: 0.375rem; font-size: 0.875rem; }
.modal__warning { color: #b45309; margin: 0; }
.modal__error { color: var(--color-danger, #b91c1c); margin: 0; }
.modal__field-error { color: var(--color-danger, #b91c1c); margin: -0.25rem 0 0; font-size: 0.8125rem; }
.modal__actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 0.5rem; }
</style>
