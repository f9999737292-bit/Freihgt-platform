<script setup lang="ts">
import {
  SIGNATURE_TYPES,
  emptySignatureForm,
  hasFormErrors,
  validateSignatureForm,
  type SignatureFormErrors,
} from '~/types/document'
import type { Company } from '~/types/company'
import type { User } from '~/types/user'

const props = defineProps<{ open: boolean; signingSessionId: string }>()
const emit = defineEmits<{ close: []; created: [] }>()

const { addSignature } = useSigningApi()
const { listCompanies } = useCompanies()
const { listUsers } = useUsersApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const companies = ref<Company[]>([])
const users = ref<User[]>([])
const form = reactive(emptySignatureForm())
const errors = reactive<SignatureFormErrors>({})

const companyOptions = computed(() =>
  companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
)
const userOptions = computed(() =>
  users.value.map((u) => ({ label: `${u.full_name} (${u.email})`, value: u.id })),
)
const signatureTypeOptions = computed(() =>
  SIGNATURE_TYPES.map((v) => ({ label: v, value: v })),
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    Object.assign(form, emptySignatureForm())
    Object.keys(errors).forEach((k) => delete errors[k as keyof SignatureFormErrors])
    errorMessage.value = ''
    try {
      const [companiesData, usersData] = await Promise.all([
        listCompanies({ limit: 100 }),
        listUsers({ limit: 100 }),
      ])
      companies.value = companiesData.items
      users.value = usersData.items
    } catch {
      companies.value = []
      users.value = []
    }
  },
)

function fieldError(field: keyof SignatureFormErrors) {
  const code = errors[field]
  if (!code) return ''
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateSignatureForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    await addSignature(props.signingSessionId, {
      signer_user_id: form.signer_user_id,
      signer_company_id: form.signer_company_id,
      signature_type: form.signature_type,
      certificate_fingerprint: form.certificate_fingerprint,
    })
    pushToast('success', t('documents.signatureAdded'))
    emit('created')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('documents.addSignatureFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('documents.mockSignature')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiSelect
        v-model="form.signer_user_id"
        :label="$t('users.title')"
        :options="userOptions"
        required
      />
      <p v-if="errors.signer_user_id" class="field-error">{{ fieldError('signer_user_id') }}</p>

      <UiSelect
        v-model="form.signer_company_id"
        :label="$t('documents.ownerCompany')"
        :options="companyOptions"
        required
      />
      <p v-if="errors.signer_company_id" class="field-error">{{ fieldError('signer_company_id') }}</p>

      <UiSelect
        v-model="form.signature_type"
        :label="$t('documents.signatureType')"
        :options="signatureTypeOptions"
      />
      <UiInput
        v-model="form.certificate_fingerprint"
        :label="$t('documents.certificateFingerprint')"
      />
    </div>
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" :disabled="saving" @click="submit">{{ $t('documents.addSignature') }}</UiButton>
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
</style>
