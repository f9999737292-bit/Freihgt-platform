<script setup lang="ts">
import {
  COMPANY_STATUSES,
  PREFERRED_LOCALES,
  hasFormErrors,
  validateCompanyForm,
  type Company,
  type CompanyFormErrors,
  type UpdateCompanyPayload,
} from '~/types/company'

const props = defineProps<{ open: boolean; company: Company | null }>()
const emit = defineEmits<{
  close: []
  updated: [company: Company]
}>()

const { updateCompany } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const form = reactive<UpdateCompanyPayload>({})
const errors = reactive<CompanyFormErrors>({})

const localeOptions = computed(() =>
  PREFERRED_LOCALES.map((value) => ({ label: value, value })),
)
const statusOptions = computed(() =>
  COMPANY_STATUSES.map((value) => ({ label: value, value })),
)

watch(
  () => props.open,
  (open) => {
    if (!open || !props.company) return
    form.legal_name = props.company.legal_name
    form.short_name = props.company.short_name ?? ''
    form.legal_name_en = props.company.legal_name_en ?? ''
    form.legal_name_zh = props.company.legal_name_zh ?? ''
    form.tax_id = props.company.tax_id ?? ''
    form.registration_number = props.company.registration_number ?? ''
    form.country_code = props.company.country_code
    form.preferred_locale = props.company.preferred_locale
    form.status = props.company.status
    Object.keys(errors).forEach((key) => delete errors[key as keyof CompanyFormErrors])
  },
)

function fieldError(field: keyof CompanyFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (field === 'country_code' && code === 'invalid') return t('companies.validation.countryCode')
  return t('companies.validation.required')
}

async function submit() {
  if (!props.company) return

  Object.assign(
    errors,
    validateCompanyForm({
      legal_name: form.legal_name ?? '',
      company_type: props.company.company_type,
      country_code: form.country_code ?? '',
      preferred_locale: form.preferred_locale ?? '',
    }),
  )
  if (hasFormErrors(errors)) return

  saving.value = true
  try {
    const company = await updateCompany(props.company.id, form)
    pushToast('success', t('companies.updateSuccess'))
    emit('updated', company)
    emit('close')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('companies.edit')" @close="$emit('close')">
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.legal_name" :label="$t('companies.legalName')" required />
      <p v-if="errors.legal_name" class="field-error">{{ fieldError('legal_name') }}</p>

      <UiInput v-model="form.short_name" :label="$t('companies.shortName')" />
      <UiInput v-model="form.legal_name_en" :label="$t('companies.legalNameEn')" />
      <UiInput v-model="form.legal_name_zh" :label="$t('companies.legalNameZh')" />
      <UiInput v-model="form.tax_id" :label="$t('companies.taxId')" />
      <UiInput v-model="form.registration_number" :label="$t('companies.registrationNumber')" />

      <UiInput v-model="form.country_code" :label="$t('companies.country')" maxlength="2" />
      <p v-if="errors.country_code" class="field-error">{{ fieldError('country_code') }}</p>

      <UiSelect
        v-model="form.preferred_locale"
        :label="$t('companies.preferredLanguage')"
        :options="localeOptions"
      />

      <UiSelect v-model="form.status" :label="$t('common.status')" :options="statusOptions" />
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" @click="submit">{{ $t('common.save') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.field-error {
  margin: -0.5rem 0 0;
  color: var(--color-danger);
  font-size: 0.8125rem;
  grid-column: 1 / -1;
}
</style>
