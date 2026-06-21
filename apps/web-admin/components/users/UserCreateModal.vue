<script setup lang="ts">
import { PREFERRED_LOCALES } from '~/types/company'
import {
  emptyCreateUserForm,
  hasUserFormErrors,
  validateCreateUserForm,
  type UserFormErrors,
} from '~/types/user'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  close: []
  created: [userId: string]
}>()

const { createUser } = useUsersApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive(emptyCreateUserForm())
const errors = reactive<UserFormErrors>({})

const localeOptions = computed(() =>
  PREFERRED_LOCALES.map((value) => ({ label: value, value })),
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptyCreateUserForm())
    Object.keys(errors).forEach((key) => delete errors[key as keyof UserFormErrors])
    errorMessage.value = ''
  },
)

function fieldError(field: keyof UserFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'invalid' && field === 'email') return t('users.validation.invalidEmail')
  if (code === 'minLength' && field === 'password') return t('users.validation.passwordMinLength')
  if (code === 'invalid' && field === 'preferred_locale') return t('users.validation.invalidLocale')
  return t('users.validation.required')
}

async function submit() {
  Object.assign(errors, validateCreateUserForm(form))
  if (hasUserFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const user = await createUser(form)
    pushToast('success', t('users.createdSuccess'))
    emit('created', user.id)
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('users.createFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('users.create')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.full_name" :label="$t('users.fullName')" required />
      <p v-if="errors.full_name" class="field-error">{{ fieldError('full_name') }}</p>

      <UiInput v-model="form.email" type="email" :label="$t('users.email')" required />
      <p v-if="errors.email" class="field-error">{{ fieldError('email') }}</p>

      <UiInput v-model="form.phone" :label="$t('users.phone')" />
      <UiInput v-model="form.password" type="password" :label="$t('users.password')" required />
      <p v-if="errors.password" class="field-error">{{ fieldError('password') }}</p>

      <UiSelect
        v-model="form.preferred_locale"
        :label="$t('users.preferredLanguage')"
        :options="localeOptions"
        required
      />
      <p v-if="errors.preferred_locale" class="field-error">{{ fieldError('preferred_locale') }}</p>
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
