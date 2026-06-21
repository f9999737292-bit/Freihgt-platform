<script setup lang="ts">
import { PARTICIPANT_TYPES } from '~/types/rfx'
import type { Company } from '~/types/company'

const props = defineProps<{ open: boolean; rfxEventId: string }>()
const emit = defineEmits<{ close: []; added: [] }>()

const { addRfxParticipant } = useRfxApi()
const { listCompanies } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const companies = ref<Company[]>([])
const form = reactive({ company_id: '', participant_type: 'CARRIER' })

const companyOptions = computed(() => {
  const carriers = companies.value.filter((c) => c.company_type === 'CARRIER')
  const list = carriers.length ? carriers : companies.value
  return list.map((c) => ({ label: `${c.legal_name} (${c.company_type})`, value: c.id }))
})
const typeOptions = computed(() => PARTICIPANT_TYPES.map((v) => ({ label: v, value: v })))

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    form.company_id = ''
    form.participant_type = 'CARRIER'
    errorMessage.value = ''
    try {
      const data = await listCompanies({ limit: 100 })
      companies.value = data.items
    } catch {
      companies.value = []
    }
  },
)

async function submit() {
  if (!form.company_id.trim()) {
    errorMessage.value = t('rfx.validation.required')
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    await addRfxParticipant(props.rfxEventId, {
      company_id: form.company_id,
      participant_type: form.participant_type,
    })
    pushToast('success', t('rfx.participantAdded'))
    emit('added')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('rfx.addParticipant')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid">
      <UiSelect
        v-model="form.company_id"
        :label="$t('rfx.company')"
        :options="companyOptions"
        required
      />
      <UiSelect
        v-model="form.participant_type"
        :label="$t('rfx.participantType')"
        :options="typeOptions"
      />
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
</style>
