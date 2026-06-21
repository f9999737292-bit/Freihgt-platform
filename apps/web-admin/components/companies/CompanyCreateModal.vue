<script setup lang="ts">
import {
  COMPANY_TYPES,
  PREFERRED_LOCALES,
  emptyCreateForm,
  hasFormErrors,
  validateCompanyForm,
  type CreateCompanyPayload,
  type CompanyFormErrors,
} from '~/types/company'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  close: []
  created: [companyId: string]
}>()

const { createCompany } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const form = reactive<CreateCompanyPayload>(emptyCreateForm())
const errors = reactive<CompanyFormErrors>({})

const companyTypeOptions = computed(() =>
  COMPANY_TYPES.map((value) => ({ label: value, value })),
)
const localeOptions = computed(() =>
  PREFERRED_LOCALES.map((value) => ({ label: value, value })),
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptyCreateForm())
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
  Object.assign(errors, validateCompanyForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  try {
    const company = await createCompany(form)
    pushToast('success', t('companies.createSuccess'))
    emit('created', company.id)
    emit('close')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('companies.create')" @close="$emit('close')">
    <div class="form-grid form-grid--2">
      <UiInput
        v-model="form.legal_name"
        :label="$t('companies.legalName')"
        required
        :placeholder="$t('companies.legalName')"
      />
      <p v-if="errors.legal_name" class="field-error">{{ fieldError('legal_name') }}</p>

      <UiInput v-model="form.short_name" :label="$t('companies.shortName')" />
      <UiInput v-model="form.legal_name_en" :label="$t('companies.legalNameEn')" />
      <UiInput v-model="form.legal_name_zh" :label="$t('companies.legalNameZh')" />

      <UiSelect
        v-model="form.company_type"
        :label="$t('companies.companyType')"
        :options="companyTypeOptions"
      />
      <p v-if="errors.company_type" class="field-error">{{ fieldError('company_type') }}</p>

      <UiInput v-model="form.tax_id" :label="$t('companies.taxId')" />
      <UiInput v-model="form.registration_number" :label="$t('companies.registrationNumber')" />

      <UiInput
        v-model="form.country_code"
        :label="$t('companies.country')"
        maxlength="2"
        placeholder="RU"
      />
      <p v-if="errors.country_code" class="field-error">{{ fieldError('country_code') }}</p>

      <UiSelect
        v-model="form.preferred_locale"
        :label="$t('companies.preferredLanguage')"
        :options="localeOptions"
      />
      <p v-if="errors.preferred_locale" class="field-error">{{ fieldError('preferred_locale') }}</p>
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
