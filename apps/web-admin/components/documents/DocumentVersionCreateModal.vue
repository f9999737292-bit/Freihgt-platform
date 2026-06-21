<script setup lang="ts">
import {
  emptyDocumentVersionForm,
  hasFormErrors,
  parseJsonObject,
  validateDocumentVersionForm,
  type DocumentVersionFormErrors,
} from '~/types/document'

const props = defineProps<{ open: boolean; documentId: string }>()
const emit = defineEmits<{ close: []; created: [] }>()

const { createDocumentVersion } = useDocumentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive(emptyDocumentVersionForm())
const errors = reactive<DocumentVersionFormErrors>({})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptyDocumentVersionForm())
    Object.keys(errors).forEach((k) => delete errors[k as keyof DocumentVersionFormErrors])
    errorMessage.value = ''
  },
)

function fieldError(field: keyof DocumentVersionFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'invalidJson') return t('documents.validation.invalidJson')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateDocumentVersionForm(form))
  if (hasFormErrors(errors)) return

  const payloadJson = parseJsonObject(form.payload_json)
  if (!payloadJson) {
    errors.payload_json = 'invalidJson'
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    await createDocumentVersion(props.documentId, {
      payload_json: payloadJson,
      payload_xml_path: form.payload_xml_path || null,
      pdf_file_path: form.pdf_file_path || null,
    })
    pushToast('success', t('documents.versionCreated'))
    emit('created')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('documents.createVersion')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid">
      <label class="json-field">
        <span class="json-field__label">{{ $t('documents.payloadJson') }}</span>
        <textarea v-model="form.payload_json" class="json-field__control" rows="6" />
      </label>
      <p v-if="errors.payload_json" class="field-error">{{ fieldError('payload_json') }}</p>
      <UiInput v-model="form.payload_xml_path" :label="$t('documents.payloadXmlPath')" />
      <UiInput v-model="form.pdf_file_path" :label="$t('documents.pdfFilePath')" />
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

.json-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.json-field__label {
  font-size: 0.875rem;
  font-weight: 500;
}

.json-field__control {
  min-height: 120px;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  font-family: ui-monospace, monospace;
  font-size: 0.8125rem;
}
</style>
