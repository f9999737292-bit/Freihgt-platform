<script setup lang="ts">
import type { User } from '~/types/user'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getUser, isApiUnavailableError } = useUsersApi()
const { pushToast } = useToast()
const { t } = useI18n()

const user = ref<User | null>(null)
const loading = ref(true)
const apiUnavailable = ref(false)
const showEditModal = ref(false)

const userId = computed(() => String(route.params.id))

async function loadUser() {
  loading.value = true
  apiUnavailable.value = false
  try {
    user.value = await getUser(userId.value)
  } catch (error) {
    user.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

function onUpdated(updated: User) {
  user.value = updated
}

watch(userId, loadUser, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <nav class="user-breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/users">{{ $t('users.title') }}</NuxtLink>
      <span class="user-breadcrumbs__sep">/</span>
      <span class="user-breadcrumbs__current">{{ $t('users.details') }}</span>
    </nav>

    <UiPageHeader :title="user?.full_name || $t('users.details')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/users')">
          {{ $t('common.back') }}
        </UiButton>
        <UiButton v-if="user" @click="showEditModal = true">
          {{ $t('users.edit') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>

    <UiEmptyState
      v-else-if="apiUnavailable"
      :title="$t('users.loadFailed')"
    />

    <UiEmptyState
      v-else-if="!user"
      :title="$t('users.noUsersFound')"
    />

    <template v-else>
      <UsersUserDetailsCard :user="user" @edit="showEditModal = true" />
      <UsersUserCompaniesTable :user-id="user.id" />
    </template>

    <UsersUserEditModal
      :open="showEditModal"
      :user="user"
      @close="showEditModal = false"
      @updated="onUpdated"
    />
  </div>
</template>

<style scoped>
.user-breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.user-breadcrumbs__sep {
  opacity: 0.5;
}

.user-breadcrumbs__current {
  color: var(--color-text);
  font-weight: 500;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}
</style>
