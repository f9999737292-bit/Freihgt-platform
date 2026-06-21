<script setup lang="ts">
import { USER_STATUSES, formatUserDate, type User } from '~/types/user'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listUsers, isApiUnavailableError } = useUsersApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const items = ref<User[]>([])
const total = ref(0)
const loading = ref(true)
const loadFailed = ref(false)
const showCreateModal = ref(false)

const filters = reactive({
  search: '',
  status: '',
})

const pagination = reactive({
  limit: 20,
  offset: 0,
})

const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...USER_STATUSES.map((value) => ({ label: value, value })),
])

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

let searchTimer: ReturnType<typeof setTimeout> | undefined

async function loadUsers() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    total.value = 0
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listUsers({
      search: filters.search,
      status: filters.status,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items ?? []
    total.value = data.total ?? items.value.length
  } catch (error) {
    items.value = []
    total.value = 0
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('users.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  loadUsers()
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(onFiltersChange, 350)
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadUsers()
}

function goNext() {
  pagination.offset += pagination.limit
  loadUsers()
}

function onCreated(userId: string) {
  loadUsers()
  router.push(`/users/${userId}`)
}

onMounted(loadUsers)
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('users.title')">
      <template #actions>
        <UiButton @click="showCreateModal = true">{{ $t('users.create') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-row">
        <UiInput
          v-model="filters.search"
          :label="$t('common.search')"
          :placeholder="$t('common.search')"
          @update:model-value="onSearchInput"
        />
        <UiSelect
          v-model="filters.status"
          :label="$t('common.status')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
      </div>
    </UiCard>

    <UiEmptyState
      v-if="loadFailed && !loading"
      :title="$t('users.loadFailed')"
    />

    <UiEmptyState
      v-else-if="!loading && !hasItems"
      :title="$t('users.noUsersFound')"
    />

    <UiCard v-else>
      <UiTable
        :columns="[
          $t('users.fullName'),
          $t('users.email'),
          $t('users.phone'),
          $t('users.preferredLanguage'),
          $t('common.status'),
          $t('users.createdAt'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/users/${item.id}`" class="user-link">{{ item.full_name }}</NuxtLink>
          </td>
          <td>{{ item.email }}</td>
          <td>{{ item.phone || '—' }}</td>
          <td>{{ item.preferred_locale }}</td>
          <td><UsersUserStatusBadge :status="item.status" /></td>
          <td>{{ formatUserDate(item.created_at) }}</td>
          <td>
            <NuxtLink :to="`/users/${item.id}`">{{ $t('common.details') }}</NuxtLink>
          </td>
        </tr>
      </UiTable>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }} {{ $t('users.title').toLowerCase() }}</span>
        <div class="pagination__actions">
          <UiButton size="sm" variant="secondary" :disabled="!canGoPrev" @click="goPrev">←</UiButton>
          <UiButton size="sm" variant="secondary" :disabled="!canGoNext" @click="goNext">→</UiButton>
        </div>
      </div>
    </UiCard>

    <UsersUserCreateModal
      :open="showCreateModal"
      @close="showCreateModal = false"
      @created="onCreated"
    />
  </div>
</template>

<style scoped>
.user-link {
  font-weight: 500;
  text-decoration: none;
}

.user-link:hover {
  text-decoration: underline;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--color-border);
}

.pagination__actions {
  display: flex;
  gap: 0.5rem;
}
</style>
