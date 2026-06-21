<script setup lang="ts">
import { PREFERRED_LOCALES } from '~/types/company'
import { USER_STATUSES, type User } from '~/types/user'

const props = defineProps<{ open: boolean; user: User | null }>()
const emit = defineEmits<{
  close: []
  updated: [user: User]
}>()

const { updateUser } = useUsersApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive({
  full_name: '',
  phone: '',
  preferred_locale: 'ru-RU',
  status: 'ACTIVE',
})

const localeOptions = computed(() =>
  PREFERRED_LOCALES.map((value) => ({ label: value, value })),
)
const statusOptions = computed(() =>
  USER_STATUSES.map((value) => ({ label: value, value })),
)

watch(
  () => props.open,
  (open) => {
    if (!open || !props.user) return
    form.full_name = props.user.full_name
    form.phone = props.user.phone ?? ''
    form.preferred_locale = props.user.preferred_locale
    form.status = props.user.status || 'ACTIVE'
    errorMessage.value = ''
  },
)

async function submit() {
  if (!props.user) return
  if (!form.full_name?.trim()) {
    errorMessage.value = t('users.validation.required')
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    const updated = await updateUser(props.user.id, {
      full_name: form.full_name,
      phone: form.phone,
      preferred_locale: form.preferred_locale,
      status: form.status,
    })
    pushToast('success', t('users.updatedSuccess'))
    emit('updated', updated)
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('users.edit')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.full_name" :label="$t('users.fullName')" required />
      <UiInput v-model="form.phone" :label="$t('users.phone')" />
      <UiSelect
        v-model="form.preferred_locale"
        :label="$t('users.preferredLanguage')"
        :options="localeOptions"
      />
      <UiSelect v-model="form.status" :label="$t('common.status')" :options="statusOptions" />
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
