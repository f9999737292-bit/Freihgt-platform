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
import type { RfxTemplateRecord, RfxTemplateVersionRecord } from '~/types/rfx-template'
import { canCloneFromTemplateVersion } from '~/types/rfx-template'
import { resolveI18nMapValue } from '~/utils/rfxTemplateI18n'
import { createIdempotencyKey } from '~/utils/idempotencyKey'
import { formatRfxApiError } from '~/utils/rfxApiError'

const props = defineProps<{
  open: boolean
  templates: RfxTemplateRecord[]
  templateVersions: Map<string, RfxTemplateVersionRecord[]>
}>()

const emit = defineEmits<{ close: []; created: [eventId: string] }>()

const lifecycleApi = useRfxVersionLifecycleApi(ref('clone-from-template'))
const { loadAuthorizedOwnerCompanies } = useRfxOwnerCompanies()
const { pushToast } = useToast()
const { t, locale } = useI18n()

const saving = ref(false)
const selectedTemplateId = ref('')
const selectedVersionId = ref('')
const idempotencyKey = ref('')
const replayEventId = ref<string | null>(null)
const form = reactive(emptyCreateRfxForm())
const errors = reactive<RfxFormErrors>({})
const ownerOptions = ref<Array<{ label: string; value: string }>>([])

const selectedTemplate = computed(() => props.templates.find((tpl) => tpl.id === selectedTemplateId.value))
const cloneableVersions = computed(() => {
  const template = selectedTemplate.value
  if (!template) return []
  const versions = props.templateVersions.get(template.id) ?? []
  return versions.filter((v) => canCloneFromTemplateVersion(template.status, v.status))
})

const supersededWarning = computed(() => {
  const version = cloneableVersions.value.find((v) => v.id === selectedVersionId.value)
  return version?.status === 'SUPERSEDED'
})

watch(
  () => props.open,
  async (isOpen) => {
    if (!isOpen) return
    Object.assign(form, emptyCreateRfxForm())
    replaceRfxFormErrors(errors, {})
    selectedTemplateId.value = ''
    selectedVersionId.value = ''
    idempotencyKey.value = createIdempotencyKey('clone-tpl')
    replayEventId.value = null
    const owners = await loadAuthorizedOwnerCompanies()
    ownerOptions.value = owners.map((o) => ({ label: o.legal_name, value: o.id }))
  },
)

watch(selectedTemplateId, () => {
  selectedVersionId.value = cloneableVersions.value[0]?.id ?? ''
})

async function handleSubmit() {
  replaceRfxFormErrors(errors, validateCreateRfxForm(form))
  if (hasFormErrors(errors)) return
  if (!selectedVersionId.value) {
    pushToast('error', t('rfx.templates.clone.selectVersion'))
    return
  }

  saving.value = true
  try {
    const result = await lifecycleApi.cloneFromTemplate(
      {
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
      },
      idempotencyKey.value,
    )
    replayEventId.value = result.id
    pushToast('success', t('rfx.templates.clone.success'))
    emit('created', result.id)
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    saving.value = false
  }
}

function templateLabel(tpl: RfxTemplateRecord) {
  return resolveI18nMapValue(tpl.name_i18n, locale.value) || tpl.template_code
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" role="dialog" aria-modal="true" @click.self="emit('close')">
    <div class="modal">
      <h2>{{ $t('rfx.templates.clone.title') }}</h2>

      <label class="modal__label">
        {{ $t('rfx.templates.clone.selectTemplate') }}
        <select v-model="selectedTemplateId">
          <option value="">{{ $t('common.select') }}</option>
          <option v-for="tpl in templates" :key="tpl.id" :value="tpl.id">
            {{ templateLabel(tpl) }}
          </option>
        </select>
      </label>

      <label v-if="selectedTemplateId" class="modal__label">
        {{ $t('rfx.templates.clone.selectVersion') }}
        <select v-model="selectedVersionId">
          <option v-for="ver in cloneableVersions" :key="ver.id" :value="ver.id">
            v{{ ver.version_number }} — {{ $t(`rfx.templates.versionStatus.${ver.status}`) }}
          </option>
        </select>
      </label>

      <p v-if="supersededWarning" class="modal__warning">{{ $t('rfx.templates.clone.supersededWarning') }}</p>

      <label class="modal__label">{{ $t('rfx.rfxNumber') }}<input v-model="form.rfx_number" /></label>
      <label class="modal__label">{{ $t('rfx.title') }}<input v-model="form.title" /></label>
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
        <select v-model="form.owner_company_id">
          <option v-for="opt in ownerOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
      </label>

      <p v-if="replayEventId" class="modal__info">{{ $t('rfx.templates.clone.replay', { id: replayEventId }) }}</p>

      <div class="modal__actions">
        <button type="button" class="btn btn--secondary" :disabled="saving" @click="emit('close')">
          {{ $t('common.cancel') }}
        </button>
        <button type="button" class="btn btn--primary" :disabled="saving" @click="handleSubmit">
          {{ $t('rfx.templates.clone.submit') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: grid; place-items: center; z-index: 1000; padding: 1rem; }
.modal { background: #fff; border-radius: var(--radius-lg); padding: 1.5rem; width: min(100%, 36rem); max-height: 90vh; overflow: auto; display: flex; flex-direction: column; gap: 0.75rem; }
.modal__label { display: flex; flex-direction: column; gap: 0.375rem; font-size: 0.875rem; }
.modal__warning { color: #b45309; margin: 0; }
.modal__info { font-size: 0.875rem; color: var(--color-text-muted); margin: 0; }
.modal__actions { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 0.5rem; }
</style>
