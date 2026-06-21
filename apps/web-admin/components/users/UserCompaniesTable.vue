<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/user'

const props = defineProps<{ userId: string }>()

const { getUserCompanies, isApiUnavailableError } = useUsersApi()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<UserCompanyMembership[]>([])
const loading = ref(true)
const loadFailed = ref(false)

function formatRoles(company: UserCompanyMembership) {
  if (!company.roles?.length) return '—'
  return company.roles.map((role) => role.name || role.code).join(', ')
}

async function loadCompanies() {
  loading.value = true
  loadFailed.value = false
  try {
    items.value = await getUserCompanies(props.userId)
  } catch (error) {
    items.value = []
    loadFailed.value = true
    if (!isApiUnavailableError(error)) {
      pushToast('error', error instanceof Error ? error.message : t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

watch(
  () => props.userId,
  () => loadCompanies(),
  { immediate: true },
)
</script>

<template>
  <UiCard>
    <template #header>
      <h3>{{ $t('users.userCompanies') }}</h3>
    </template>

    <UiTable
      v-if="items.length || loading"
      :columns="[
        $t('users.company'),
        $t('companies.companyType'),
        $t('users.position'),
        $t('users.membershipStatus'),
        $t('users.role'),
      ]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.membership_id">
        <td>
          <NuxtLink :to="`/companies/${item.company_id}`" class="company-link">
            {{ item.legal_name }}
          </NuxtLink>
        </td>
        <td><CompaniesCompanyTypeBadge :type="item.company_type" /></td>
        <td>{{ item.position || '—' }}</td>
        <td><CompaniesCompanyMemberStatusBadge :status="item.membership_status" /></td>
        <td>{{ formatRoles(item) }}</td>
      </tr>
    </UiTable>

    <UiEmptyState
      v-else-if="loadFailed"
      :title="$t('users.loadCompaniesFailed')"
    />
    <UiEmptyState
      v-else
      :title="$t('users.noUserCompanies')"
    />
  </UiCard>
</template>

<style scoped>
h3 {
  margin: 0;
  font-size: 1rem;
}

.company-link {
  font-weight: 500;
  text-decoration: none;
}

.company-link:hover {
  text-decoration: underline;
}
</style>
