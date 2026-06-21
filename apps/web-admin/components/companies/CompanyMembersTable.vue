<script setup lang="ts">
import type { CompanyMember } from '~/types/company'

const props = defineProps<{ companyId: string }>()

const { listCompanyMembers, isApiUnavailableError } = useCompanyMembersApi()
const { pushToast } = useToast()
const { t } = useI18n()

const members = ref<CompanyMember[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showAddModal = ref(false)

async function loadMembers() {
  loading.value = true
  loadFailed.value = false
  try {
    const data = await listCompanyMembers(props.companyId)
    members.value = data.items ?? []
  } catch (err) {
    members.value = []
    loadFailed.value = true
    if (!isApiUnavailableError(err)) {
      pushToast('error', err instanceof Error ? err.message : t('users.loadMembersFailed'))
    }
  } finally {
    loading.value = false
  }
}

function formatRoles(member: CompanyMember) {
  if (!member.roles?.length) return '—'
  return member.roles.map((role) => role.name || role.code).join(', ')
}

watch(
  () => props.companyId,
  () => loadMembers(),
  { immediate: true },
)
</script>

<template>
  <UiCard>
    <template #header>
      <div class="members-header">
        <h3>{{ $t('users.companyMembers') }}</h3>
        <UiButton size="sm" @click="showAddModal = true">{{ $t('users.addEmployee') }}</UiButton>
      </div>
    </template>

    <UiTable
      v-if="members.length || loading"
      :columns="[
        $t('users.fullName'),
        $t('users.email'),
        $t('users.phone'),
        $t('users.position'),
        $t('common.status'),
        $t('users.role'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="member in members" :key="member.membership_id">
        <td>{{ member.full_name || '—' }}</td>
        <td>{{ member.email || '—' }}</td>
        <td>{{ member.phone || '—' }}</td>
        <td>{{ member.position || '—' }}</td>
        <td><CompaniesCompanyMemberStatusBadge :status="member.status" /></td>
        <td>{{ formatRoles(member) }}</td>
        <td>
          <NuxtLink :to="`/users/${member.user_id}`">{{ $t('common.details') }}</NuxtLink>
        </td>
      </tr>
    </UiTable>

    <UiEmptyState
      v-else-if="loadFailed"
      :title="$t('users.loadMembersFailed')"
    />
    <UiEmptyState
      v-else
      :title="$t('users.noCompanyMembers')"
    />
  </UiCard>

  <CompaniesCompanyMemberCreateModal
    :open="showAddModal"
    :company-id="companyId"
    @close="showAddModal = false"
    @added="loadMembers"
  />
</template>

<style scoped>
.members-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.members-header h3 {
  margin: 0;
  font-size: 1rem;
}
</style>
