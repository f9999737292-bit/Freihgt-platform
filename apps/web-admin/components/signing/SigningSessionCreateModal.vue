<script setup lang="ts">
import {
  emptySigningSessionForm,
  hasFormErrors,
  toRFC3339,
  validateSigningSessionForm,
  type SigningSessionFormErrors,
} from '~/types/document'

const props = defineProps<{ open: boolean; documentId: string }>()
const emit = defineEmits<{ close: []; created: [sessionId: string] }>()

const { createSigningSession } = useDocumentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive(emptySigningSessionForm())
const errors = reactive<SigningSessionFormErrors>({})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptySigningSessionForm())
    Object.keys(errors).forEach((k) => delete errors[k as keyof SigningSessionFormErrors])
    errorMessage.value = ''
  },
)

function fieldError(field: keyof SigningSessionFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'positive') return t('documents.validation.positive')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateSigningSessionForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const session = await createSigningSession(props.documentId, {
      required_signers_count: Number(form.required_signers_count),
      expires_at: toRFC3339(form.expires_at),
    })
    pushToast('success', t('documents.signingSessionCreated'))
    emit('created', session.id)
    emit('close')
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : t('documents.signingSessionCreateFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('documents.createSigningSession')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput
        v-model="form.required_signers_count"
        type="number"
        :label="$t('documents.requiredSignersCount')"
        required
      />
      <p v-if="errors.required_signers_count" class="field-error">
        {{ fieldError('required_signers_count') }}
      </p>

      <UiInput
        v-model="form.expires_at"
        type="datetime-local"
        :label="$t('freightRequests.validUntil')"
        required
      />
      <p v-if="errors.expires_at" class="field-error">{{ fieldError('expires_at') }}</p>
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
</style>
