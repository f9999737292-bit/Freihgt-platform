<script setup lang="ts">
import type { Company } from '~/types/company'
import type { UserCompanyMembership } from '~/types/company'
import {
  filterCarrierMemberships,
  membershipSelectOptions,
  selectDefaultCarrierCompany,
} from '~/utils/companyMembership'
import { shouldShowNotFound, isApiUnavailableError } from '~/utils/apiError'
import { ApiError } from '~/utils/apiClient'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getTender } = useCarrierRfxApi()
const { getUserCompanies, listCompanies } = useCompanies()
const { setCompany } = useTenantContext()
const authStore = useAuthStore()
const { t } = useI18n()

const eventId = computed(() => String(route.params.id))
const loading = ref(true)
const notFound = ref(false)
const permissionDenied = ref(false)
const apiUnavailable = ref(false)

const event = ref<Awaited<ReturnType<typeof getTender>> | null>(null)
const companies = ref<Company[]>([])
const memberships = ref<UserCompanyMembership[]>([])
const selectedCarrierCompanyId = ref('')

const carrierMemberships = computed(() => filterCarrierMemberships(memberships.value))
const carrierOptions = computed(() => membershipSelectOptions(carrierMemberships.value))
const hasMembership = computed(() => carrierMemberships.value.length > 0)
const buyerName = computed(() => {
  if (!event.value) return '—'
  return companies.value.find((c) => c.id === event.value!.owner_company_id)?.legal_name
    || event.value.owner_company_id
})

async function loadMemberships() {
  const userId = authStore.user?.id
  if (!userId) return
  try {
    memberships.value = await getUserCompanies(userId)
    if (!selectedCarrierCompanyId.value) {
      selectedCarrierCompanyId.value = selectDefaultCarrierCompany(filterCarrierMemberships(memberships.value))
    }
  } catch {
    memberships.value = []
  }
}

async function loadCompanies() {
  try {
    companies.value = (await listCompanies({ limit: 200, status: 'ACTIVE' })).items
  } catch {
    companies.value = []
  }
}

async function loadEvent() {
  loading.value = true
  notFound.value = false
  permissionDenied.value = false
  apiUnavailable.value = false
  try {
    event.value = await getTender(eventId.value)
  } catch (err) {
    if (shouldShowNotFound(err)) notFound.value = true
    else if (err instanceof ApiError && err.status === 403) permissionDenied.value = true
    else if (isApiUnavailableError(err)) apiUnavailable.value = true
    event.value = null
  } finally {
    loading.value = false
  }
}

watch(selectedCarrierCompanyId, (id) => {
  if (id) setCompany(id)
})

onMounted(async () => {
  await Promise.all([loadMemberships(), loadCompanies(), loadEvent()])
})
</script>

<template>
  <div class="page">
    <p>
      <NuxtLink :to="`/carrier/tenders/${eventId}`">{{ t('carrierResponse.backToTender') }}</NuxtLink>
    </p>

    <div v-if="loading">{{ t('common.loading') }}</div>
    <div v-else-if="notFound">{{ t('carrierTenders.notAvailable') }}</div>
    <div v-else-if="permissionDenied">{{ t('carrierTenders.permissionDenied') }}</div>
    <div v-else-if="apiUnavailable">{{ t('common.backendUnavailable') }}</div>
    <div v-else-if="!hasMembership">{{ t('carrierTenders.noMembership') }}</div>

    <template v-else-if="event">
      <div class="company-select">
        <label>{{ t('carrierTenders.companyLabel') }}</label>
        <select v-model="selectedCarrierCompanyId">
          <option v-for="opt in carrierOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
      </div>

      <CarrierResponseWorkspace
        v-if="selectedCarrierCompanyId"
        :event="event"
        :buyer-name="buyerName"
        :carrier-company-id="selectedCarrierCompanyId"
      />
    </template>
  </div>
</template>

<style scoped>
.page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 1.5rem 1rem 3rem;
}
.company-select {
  margin-bottom: 1rem;
  display: flex;
  gap: 0.75rem;
  align-items: center;
}
</style>
