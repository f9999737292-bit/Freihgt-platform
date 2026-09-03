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

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createRfxEvent } = useRfxApi()
const { loadAuthorizedOwnerCompanies, isApiUnavailableError } = useRfxOwnerCompanies()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const saving = ref(false)
const errorMessage = ref('')
const ownerOptions = ref<Array<{ label: string; value: string }>>([])
const ownerLoadState = ref<'idle' | 'loading' | 'ready' | 'empty' | 'error'>('idle')
const form = reactive(emptyCreateRfxForm())
const errors = reactive<RfxFormErrors>({})

const typeOptions = computed(() => RFX_TYPES.map((v) => ({ label: v, value: v })))
const categoryOptions = computed(() => RFX_CATEGORIES.map((v) => ({ label: v, value: v })))

const ownerPlaceholder = computed(() => {
  if (ownerLoadState.value === 'loading') return t('rfx.ownerLoading')
  if (ownerLoadState.value === 'error') return t('rfx.ownerLoadFailed')
  if (ownerLoadState.value === 'empty') return t('rfx.noOwnerCompanies')
  return t('rfx.selectOwner')
})

const ownerSelectDisabled = computed(
  () => ownerLoadState.value === 'loading' || ownerLoadState.value === 'error' || ownerLoadState.value === 'empty',
)

watch(
  () => form.title,
  (value) => {
    if (value.trim()) delete errors.title
  },
)

watch(
  () => form.owner_company_id,
  (value) => {
    if (value.trim()) delete errors.owner_company_id
  },
)

watch(
  () => form.rfx_number,
  (value) => {
    if (value.trim()) delete errors.rfx_number
  },
)

watch(
  () => form.response_deadline,
  (value) => {
    if (value?.trim()) delete errors.response_deadline
  },
)

watch(
  () => form.valid_to,
  () => {
    if (!errors.valid_to) return
    if (!form.valid_from || !form.valid_to || form.valid_to >= form.valid_from) {
      delete errors.valid_to
    }
  },
)

function resetFormState() {
  Object.assign(form, emptyCreateRfxForm())
  replaceRfxFormErrors(errors, {})
  errorMessage.value = ''
  ownerOptions.value = []
  ownerLoadState.value = 'idle'
}

async function loadOwnerCompanies() {
  ownerLoadState.value = 'loading'
  ownerOptions.value = []
  form.owner_company_id = ''

  try {
    const result = await loadAuthorizedOwnerCompanies()
    ownerOptions.value = result.options
    ownerLoadState.value = result.state

    if (result.memberships.length === 1) {
      form.owner_company_id = result.memberships[0].company_id
    }
  } catch (error) {
    ownerLoadState.value = 'error'
    ownerOptions.value = []
    if (!isApiUnavailableError(error)) {
      pushToast('error', error instanceof Error ? error.message : t('rfx.ownerLoadFailed'))
    }
  }
}

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    resetFormState()
    await loadOwnerCompanies()
  },
)

function fieldError(field: keyof RfxFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'range') return t('rfx.validation.validToRange')
  return t('rfx.validation.required')
}

function applyValidation() {
  replaceRfxFormErrors(errors, validateCreateRfxForm(form))
}

async function submit() {
  applyValidation()
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const event = await createRfxEvent({
      ...form,
      response_deadline: toRFC3339(form.response_deadline || ''),
    })
    pushToast('success', t('rfx.createdSuccess'))
    emit('created', event.id)
    emit('close')
    await router.push(`/rfx/${event.id}`)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('rfx.create')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.rfx_number" :label="$t('rfx.rfxNumber')" required />
      <p v-if="errors.rfx_number" class="field-error">{{ fieldError('rfx_number') }}</p>

      <UiSelect v-model="form.rfx_type" :label="$t('rfx.rfxType')" :options="typeOptions" />
      <UiSelect v-model="form.category" :label="$t('rfx.category')" :options="categoryOptions" />

      <UiInput v-model="form.title" :label="$t('rfx.titleLabel')" required />
      <p v-if="errors.title" class="field-error">{{ fieldError('title') }}</p>

      <UiSelect
        v-model="form.owner_company_id"
        :label="$t('rfx.owner')"
        :options="ownerOptions"
        :placeholder="ownerPlaceholder"
        :disabled="ownerSelectDisabled"
        :loading="ownerLoadState === 'loading'"
        required
      />
      <p v-if="ownerLoadState === 'error'" class="field-hint field-hint--error">
        {{ $t('rfx.ownerLoadFailed') }}
      </p>
      <p v-else-if="ownerLoadState === 'empty'" class="field-hint">
        {{ $t('rfx.noOwnerCompanies') }}
      </p>
      <p v-if="errors.owner_company_id" class="field-error">{{ fieldError('owner_company_id') }}</p>

      <UiInput v-model="form.currency_code" :label="$t('rfx.currency')" />
      <UiInput v-model="form.valid_from" type="date" :label="$t('rfx.validFrom')" />
      <UiInput v-model="form.valid_to" type="date" :label="$t('rfx.validTo')" />
      <p v-if="errors.valid_to" class="field-error">{{ fieldError('valid_to') }}</p>

      <UiInput
        v-model="form.response_deadline"
        type="datetime-local"
        :label="$t('rfx.responseDeadline')"
        required
      />
      <p v-if="errors.response_deadline" class="field-error">{{ fieldError('response_deadline') }}</p>

      <UiInput v-model="form.description" :label="$t('rfx.description')" />
    </div>
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" :disabled="saving" @click="submit">{{ $t('common.save') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.modal-error {
  margin: 0 0 1rem;
  padding: 0.75rem;
  border-radius: var(--radius-sm);
  background: #fee2e2;
  color: #991b1b;
  font-size: 0.875rem;
}

.field-error {
  margin: -0.5rem 0 0;
  font-size: 0.8125rem;
  color: #b91c1c;
}

.field-hint {
  margin: -0.5rem 0 0;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.field-hint--error {
  color: #b45309;
}
</style>
