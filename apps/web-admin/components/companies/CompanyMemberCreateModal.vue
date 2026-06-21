<script setup lang="ts">
import { PREFERRED_LOCALES } from '~/types/company'
import type { User } from '~/types/user'
import {
  emptyCreateUserForm,
  hasUserFormErrors,
  validateAddMemberForm,
  validateCreateUserForm,
  type AddMemberFormErrors,
  type UserFormErrors,
} from '~/types/user'

const props = defineProps<{ open: boolean; companyId: string }>()
const emit = defineEmits<{
  close: []
  added: []
}>()

const { createUser, listUsers } = useUsersApi()
const { addCompanyMember } = useCompanyMembersApi()
const { pushToast } = useToast()
const { t } = useI18n()

type Mode = 'create' | 'existing'

const mode = ref<Mode>('create')
const saving = ref(false)
const errorMessage = ref('')

const newUserForm = reactive(emptyCreateUserForm())
const newUserErrors = reactive<UserFormErrors>({})

const memberFields = reactive({
  user_id: '',
  position: '',
  role_id: '',
})
const memberErrors = reactive<AddMemberFormErrors>({})

const searchQuery = ref('')
const searchResults = ref<User[]>([])
const searchLoading = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | undefined

const localeOptions = computed(() =>
  PREFERRED_LOCALES.map((value) => ({ label: value, value })),
)

function resetForm() {
  mode.value = 'create'
  Object.assign(newUserForm, emptyCreateUserForm())
  Object.keys(newUserErrors).forEach((key) => delete newUserErrors[key as keyof UserFormErrors])
  memberFields.user_id = ''
  memberFields.position = ''
  memberFields.role_id = ''
  Object.keys(memberErrors).forEach((key) => delete memberErrors[key as keyof AddMemberFormErrors])
  searchQuery.value = ''
  searchResults.value = []
  errorMessage.value = ''
}

watch(
  () => props.open,
  (open) => {
    if (open) resetForm()
  },
)

watch(mode, () => {
  errorMessage.value = ''
})

function userFieldError(field: keyof UserFormErrors) {
  const code = newUserErrors[field]
  if (!code) return ''
  if (code === 'invalid' && field === 'email') return t('users.validation.invalidEmail')
  if (code === 'minLength' && field === 'password') return t('users.validation.passwordMinLength')
  if (code === 'invalid' && field === 'preferred_locale') return t('users.validation.invalidLocale')
  return t('users.validation.required')
}

function memberFieldError(field: keyof AddMemberFormErrors) {
  return memberErrors[field] ? t('users.validation.required') : ''
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(searchUsers, 350)
}

async function searchUsers() {
  const query = searchQuery.value.trim()
  if (!query) {
    searchResults.value = []
    return
  }

  searchLoading.value = true
  try {
    const data = await listUsers({ search: query, limit: 10, offset: 0 })
    searchResults.value = data.items
  } catch {
    // TODO: fallback to manual user_id if search endpoint is unavailable
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

function selectUser(user: User) {
  memberFields.user_id = user.id
  searchQuery.value = user.full_name
  searchResults.value = []
}

async function addMember(userId: string) {
  await addCompanyMember(props.companyId, {
    user_id: userId,
    position: memberFields.position.trim(),
    role_id: memberFields.role_id.trim() || undefined,
  })
}

async function submitCreateMode() {
  Object.assign(newUserErrors, validateCreateUserForm(newUserForm))
  Object.assign(memberErrors, validateAddMemberForm({ user_id: 'placeholder', position: memberFields.position }))
  delete memberErrors.user_id

  if (hasUserFormErrors(newUserErrors) || hasUserFormErrors(memberErrors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const user = await createUser(newUserForm)
    await addMember(user.id)
    pushToast('success', t('users.employeeAdded'))
    emit('added')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('users.addEmployeeFailed')
  } finally {
    saving.value = false
  }
}

async function submitExistingMode() {
  Object.assign(memberErrors, validateAddMemberForm(memberFields))
  if (hasUserFormErrors(memberErrors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    await addMember(memberFields.user_id.trim())
    pushToast('success', t('users.employeeAdded'))
    emit('added')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('users.addEmployeeFailed')
  } finally {
    saving.value = false
  }
}

async function submit() {
  if (mode.value === 'create') {
    await submitCreateMode()
  } else {
    await submitExistingMode()
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('users.addEmployee')" @close="$emit('close')">
    <div class="mode-switch">
      <button
        type="button"
        class="mode-switch__btn"
        :class="{ 'mode-switch__btn--active': mode === 'create' }"
        @click="mode = 'create'"
      >
        {{ $t('users.createNewUser') }}
      </button>
      <button
        type="button"
        class="mode-switch__btn"
        :class="{ 'mode-switch__btn--active': mode === 'existing' }"
        @click="mode = 'existing'"
      >
        {{ $t('users.addExistingUser') }}
      </button>
    </div>

    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>

    <div v-if="mode === 'create'" class="form-grid form-grid--2">
      <UiInput v-model="newUserForm.full_name" :label="$t('users.fullName')" required />
      <p v-if="newUserErrors.full_name" class="field-error">{{ userFieldError('full_name') }}</p>

      <UiInput v-model="newUserForm.email" type="email" :label="$t('users.email')" required />
      <p v-if="newUserErrors.email" class="field-error">{{ userFieldError('email') }}</p>

      <UiInput v-model="newUserForm.phone" :label="$t('users.phone')" />
      <UiInput v-model="newUserForm.password" type="password" :label="$t('users.password')" required />
      <p v-if="newUserErrors.password" class="field-error">{{ userFieldError('password') }}</p>

      <UiSelect
        v-model="newUserForm.preferred_locale"
        :label="$t('users.preferredLanguage')"
        :options="localeOptions"
        required
      />
      <p v-if="newUserErrors.preferred_locale" class="field-error">{{ userFieldError('preferred_locale') }}</p>

      <UiInput v-model="memberFields.position" :label="$t('users.position')" required />
      <p v-if="memberErrors.position" class="field-error">{{ memberFieldError('position') }}</p>

      <UiInput v-model="memberFields.role_id" :label="$t('users.role')" />
    </div>

    <div v-else class="form-grid">
      <UiInput
        v-model="searchQuery"
        :label="$t('users.searchUser')"
        @update:model-value="onSearchInput"
      />

      <div v-if="searchLoading" class="text-sm text-muted">{{ $t('common.loading') }}</div>
      <ul v-else-if="searchResults.length" class="search-results">
        <li v-for="user in searchResults" :key="user.id">
          <button type="button" class="search-results__item" @click="selectUser(user)">
            <strong>{{ user.full_name }}</strong>
            <span>{{ user.email }}</span>
            <span>{{ user.phone || '—' }}</span>
            <UsersUserStatusBadge :status="user.status" />
          </button>
        </li>
      </ul>

      <UiInput v-model="memberFields.user_id" :label="$t('users.userId')" required />
      <p v-if="memberErrors.user_id" class="field-error">{{ memberFieldError('user_id') }}</p>

      <UiInput v-model="memberFields.position" :label="$t('users.position')" required />
      <p v-if="memberErrors.position" class="field-error">{{ memberFieldError('position') }}</p>

      <UiInput v-model="memberFields.role_id" :label="$t('users.role')" />
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" :disabled="saving" @click="submit">{{ $t('common.save') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.mode-switch {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.mode-switch__btn {
  flex: 1;
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  border-radius: var(--radius-sm);
  padding: 0.5rem 0.75rem;
  font-size: 0.8125rem;
  cursor: pointer;
}

.mode-switch__btn--active {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

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

.search-results {
  list-style: none;
  margin: 0;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  max-height: 12rem;
  overflow: auto;
}

.search-results__item {
  width: 100%;
  display: grid;
  grid-template-columns: 1fr 1fr 1fr auto;
  gap: 0.5rem;
  align-items: center;
  padding: 0.625rem 0.75rem;
  border: none;
  border-bottom: 1px solid var(--color-border);
  background: transparent;
  text-align: left;
  cursor: pointer;
  font-size: 0.8125rem;
}

.search-results__item:last-child {
  border-bottom: none;
}

.search-results__item:hover {
  background: var(--color-bg);
}
</style>
