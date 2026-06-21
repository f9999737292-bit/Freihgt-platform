<script setup lang="ts">
import type { Company } from '~/types/company'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getCompany, isApiUnavailableError } = useCompanies()
const { setCompany } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const company = ref<Company | null>(null)
const loading = ref(true)
const apiUnavailable = ref(false)
const showEditModal = ref(false)

const companyId = computed(() => String(route.params.id))

async function loadCompany() {
  loading.value = true
  apiUnavailable.value = false
  try {
    company.value = await getCompany(companyId.value)
    setCompany(companyId.value)
  } catch (error) {
    company.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

function onUpdated(updated: Company) {
  company.value = updated
}

watch(companyId, loadCompany, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <nav class="company-breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/companies">{{ $t('companies.title') }}</NuxtLink>
      <span class="company-breadcrumbs__sep">/</span>
      <span class="company-breadcrumbs__current">{{ $t('companies.details') }}</span>
    </nav>

    <UiPageHeader :title="company?.legal_name || $t('companies.details')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/companies')">
          {{ $t('common.back') }}
        </UiButton>
        <UiButton v-if="company" @click="showEditModal = true">
          {{ $t('companies.edit') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>

    <UiEmptyState
      v-else-if="apiUnavailable"
      :title="$t('companies.apiUnavailable')"
    />

    <UiEmptyState
      v-else-if="!company"
      :title="$t('common.empty')"
    />

    <template v-else>
      <CompaniesCompanyDetailsCard :company="company" />
      <CompaniesCompanyMembersTable :company-id="company.id" />
    </template>

    <CompaniesCompanyEditModal
      :open="showEditModal"
      :company="company"
      @close="showEditModal = false"
      @updated="onUpdated"
    />
  </div>
</template>

<style scoped>
.company-breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.company-breadcrumbs__sep {
  opacity: 0.5;
}

.company-breadcrumbs__current {
  color: var(--color-text);
  font-weight: 500;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}
</style>
