<script setup lang="ts">
import {
  FILE_TYPES,
  emptyDocumentFileForm,
  hasFormErrors,
  validateDocumentFileForm,
  type DocumentFileFormErrors,
  type DocumentVersion,
} from '~/types/document'

const props = defineProps<{
  open: boolean
  documentId: string
  versions: DocumentVersion[]
}>()
const emit = defineEmits<{ close: []; created: [] }>()

const { addDocumentFile } = useDocumentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive(emptyDocumentFileForm())
const errors = reactive<DocumentFileFormErrors>({})

const fileTypeOptions = computed(() => FILE_TYPES.map((v) => ({ label: v, value: v })))
const versionOptions = computed(() =>
  props.versions.map((v) => ({
    label: `#${v.version_number} (${v.id.slice(0, 8)}...)`,
    value: v.id,
  })),
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    const latest = props.versions[props.versions.length - 1]
    Object.assign(form, emptyDocumentFileForm(latest?.id || ''))
    Object.keys(errors).forEach((k) => delete errors[k as keyof DocumentFileFormErrors])
    errorMessage.value = ''
  },
)

function fieldError(field: keyof DocumentFileFormErrors) {
  const code = errors[field]
  if (!code) return ''
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateDocumentFileForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    await addDocumentFile(props.documentId, {
      document_version_id: form.document_version_id,
      file_type: form.file_type,
      storage_provider: form.storage_provider,
      bucket_name: form.bucket_name,
      object_key: form.object_key,
      file_name: form.file_name,
      mime_type: form.mime_type,
      file_size_bytes: Number(form.file_size_bytes) || undefined,
      checksum_sha256: form.checksum_sha256,
    })
    pushToast('success', t('documents.fileAdded'))
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
  <UiModal :open="open" :title="$t('documents.addFile')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiSelect
        v-model="form.document_version_id"
        :label="$t('documents.documentVersion')"
        :options="versionOptions"
        required
      />
      <p v-if="errors.document_version_id" class="field-error">{{ fieldError('document_version_id') }}</p>

      <UiSelect v-model="form.file_type" :label="$t('documents.fileType')" :options="fileTypeOptions" />
      <UiInput v-model="form.file_name" :label="$t('documents.fileName')" required />
      <p v-if="errors.file_name" class="field-error">{{ fieldError('file_name') }}</p>

      <UiInput v-model="form.mime_type" :label="$t('documents.mimeType')" required />
      <p v-if="errors.mime_type" class="field-error">{{ fieldError('mime_type') }}</p>

      <UiInput v-model="form.storage_provider" :label="$t('documents.storageProvider')" />
      <UiInput v-model="form.bucket_name" :label="$t('documents.bucketName')" />
      <UiInput v-model="form.object_key" :label="$t('documents.objectKey')" />
      <UiInput v-model="form.file_size_bytes" type="number" :label="$t('documents.fileSize')" />
      <UiInput v-model="form.checksum_sha256" :label="$t('documents.checksum')" />
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
