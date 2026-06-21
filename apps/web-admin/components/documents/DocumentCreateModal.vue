<script setup lang="ts">
import {
  DOCUMENT_TYPES,
  LEGAL_LANGUAGES,
  RELATED_ENTITY_TYPES,
  emptyDocumentCreateForm,
  hasFormErrors,
  parseJsonObject,
  validateDocumentCreateForm,
  type DocumentFormErrors,
} from '~/types/document'
import type { Company } from '~/types/company'
import type { Shipment } from '~/types/shipment'

const props = defineProps<{
  open: boolean
  initialShipmentId?: string
  initialDocumentType?: string
}>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createDocument } = useDocumentsApi()
const { listCompanies } = useCompanies()
const { listShipments } = useShipmentsApi()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const saving = ref(false)
const errorMessage = ref('')
const companies = ref<Company[]>([])
const shipments = ref<Shipment[]>([])
const form = reactive(emptyDocumentCreateForm())
const errors = reactive<DocumentFormErrors>({})

const typeOptions = computed(() => DOCUMENT_TYPES.map((v) => ({ label: v, value: v })))
const entityTypeOptions = computed(() => RELATED_ENTITY_TYPES.map((v) => ({ label: v, value: v })))
const languageOptions = computed(() => LEGAL_LANGUAGES.map((v) => ({ label: v, value: v })))
const companyOptions = computed(() =>
  companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
)
const shipmentOptions = computed(() =>
  shipments.value.map((s) => ({
    label: `${s.shipment_number || s.id.slice(0, 8)} (${s.status})`,
    value: s.id,
  })),
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    Object.assign(
      form,
      emptyDocumentCreateForm(props.initialShipmentId || '', props.initialDocumentType || 'POD'),
    )
    Object.keys(errors).forEach((k) => delete errors[k as keyof DocumentFormErrors])
    errorMessage.value = ''
    try {
      const [companiesData, shipmentsData] = await Promise.all([
        listCompanies({ limit: 100 }),
        listShipments({ limit: 100 }),
      ])
      companies.value = companiesData.items
      shipments.value = shipmentsData.items
    } catch {
      companies.value = []
      shipments.value = []
    }
  },
)

watch(
  () => form.related_entity_type,
  (type) => {
    if (type !== 'SHIPMENT') return
    if (!form.related_entity_id && props.initialShipmentId) {
      form.related_entity_id = props.initialShipmentId
    }
  },
)

function fieldError(field: keyof DocumentFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'invalidJson') return t('documents.validation.invalidJson')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateDocumentCreateForm(form))
  if (hasFormErrors(errors)) return

  const payloadJson = parseJsonObject(form.payload_json)
  if (!payloadJson) {
    errors.payload_json = 'invalidJson'
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    const doc = await createDocument({
      document_number: form.document_number,
      document_type: form.document_type,
      owner_company_id: form.owner_company_id,
      related_entity_type: form.related_entity_type,
      related_entity_id: form.related_entity_id,
      legal_language: form.legal_language,
      payload_json: payloadJson,
    })
    pushToast('success', t('documents.documentCreated'))
    emit('created', doc.id)
    emit('close')
    await router.push(`/documents/${doc.id}`)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('documents.createFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('documents.createDocument')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.document_number" :label="$t('documents.documentNumber')" required />
      <p v-if="errors.document_number" class="field-error">{{ fieldError('document_number') }}</p>

      <UiSelect v-model="form.document_type" :label="$t('documents.documentType')" :options="typeOptions" />
      <UiSelect
        v-model="form.owner_company_id"
        :label="$t('documents.ownerCompany')"
        :options="companyOptions"
        required
      />
      <p v-if="errors.owner_company_id" class="field-error">{{ fieldError('owner_company_id') }}</p>

      <UiSelect
        v-model="form.related_entity_type"
        :label="$t('documents.relatedEntity')"
        :options="entityTypeOptions"
      />
      <UiSelect
        v-if="form.related_entity_type === 'SHIPMENT' && shipmentOptions.length"
        v-model="form.related_entity_id"
        :label="$t('documents.relatedEntityId')"
        :options="shipmentOptions"
        required
      />
      <UiInput
        v-else
        v-model="form.related_entity_id"
        :label="$t('documents.relatedEntityId')"
        required
      />
      <p v-if="errors.related_entity_id" class="field-error">{{ fieldError('related_entity_id') }}</p>

      <UiSelect
        v-model="form.legal_language"
        :label="$t('documents.legalLanguage')"
        :options="languageOptions"
      />

      <label class="json-field">
        <span class="json-field__label">{{ $t('documents.payloadJson') }}</span>
        <textarea v-model="form.payload_json" class="json-field__control" rows="6" />
      </label>
      <p v-if="errors.payload_json" class="field-error">{{ fieldError('payload_json') }}</p>
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
  grid-column: 1 / -1;
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
