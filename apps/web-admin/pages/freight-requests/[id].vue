<script setup lang="ts">
import type { Company } from '~/types/company'
import type { FreightRequest } from '~/types/rfx'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getFreightRequest, isApiUnavailableError } = useFreightRequestsApi()
const { listCompanies } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()

const request = ref<FreightRequest | null>(null)
const companies = ref<Company[]>([])
const loading = ref(true)
const apiUnavailable = ref(false)

const requestId = computed(() => String(route.params.id))

const companyName = computed(() => {
  if (!request.value) return ''
  return companies.value.find((c) => c.id === request.value!.shipper_company_id)?.legal_name
})

function companyNameById(id: string) {
  return companies.value.find((c) => c.id === id)?.legal_name || id
}

async function loadRequest() {
  loading.value = true
  apiUnavailable.value = false
  try {
    request.value = await getFreightRequest(requestId.value)
  } catch (error) {
    request.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('freightRequests.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

watch(requestId, loadRequest, { immediate: true })
onMounted(async () => {
  try {
    companies.value = (await listCompanies({ limit: 100 })).items
  } catch {
    companies.value = []
  }
})
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/freight-requests">{{ $t('freightRequests.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('freightRequests.details') }}</span>
    </nav>

    <UiPageHeader :title="request?.freight_request_number || $t('freightRequests.details')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/freight-requests')">
          {{ $t('common.back') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <UiEmptyState v-else-if="apiUnavailable" :title="$t('freightRequests.loadFailed')" />
    <UiEmptyState v-else-if="!request" :title="$t('freightRequests.noRequestsFound')" />

    <template v-else>
      <FreightRequestsFreightRequestActions :request="request" @updated="loadRequest" />
      <FreightRequestsFreightRequestDetailsCard :request="request" :company-name="companyName" />
      <FreightRequestsFreightRequestBidsTable
        :freight-request-id="request.id"
        :request="request"
        :company-name="companyNameById"
        @updated="loadRequest"
      />
      <LowCodeCustomFieldsPanel
        entity-type="FREIGHT_REQUEST"
        :entity-id="request.id"
        :entity-status="request.status"
      />
    </template>
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}
</style>
