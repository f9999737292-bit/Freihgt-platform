<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { Company } from '~/types/company'
import type { RfxLane, RfxLot } from '~/types/rfx'
import type { CarrierRfxResponse } from '~/types/carrierRfx'
import {
  canCreateResponse,
  canSubmitResponse,
  canEditCommercial,
  formatDeadlineRemaining,
  isDeadlineExpired,
} from '~/types/carrierRfx'
import { formatRfxDate } from '~/types/rfx'
import { formatMoney } from '~/types/evaluation'
import {
  filterCarrierMemberships,
  membershipSelectOptions,
  selectDefaultCarrierCompany,
} from '~/utils/companyMembership'
import { shouldShowNotFound, isApiUnavailableError } from '~/utils/apiError'
import { ApiError } from '~/utils/apiClient'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const {
  getTender,
  getOwnParticipant,
  getOwnResponse,
  createResponse,
  submitResponse,
  updateResponseCommercial,
  getOwnAward,
  listLots,
  listLanes,
  isApiUnavailableError: isCarrierApiUnavailable,
} = useCarrierRfxApi()
const { getUserCompanies } = useCompanies()
const { listCompanies } = useCompanies()
const { setCompany } = useTenantContext()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const eventId = computed(() => String(route.params.id))
const loading = ref(true)
const notFound = ref(false)
const permissionDenied = ref(false)
const apiUnavailable = ref(false)
const acting = ref(false)

const event = ref<Awaited<ReturnType<typeof getTender>> | null>(null)
const participant = ref<Awaited<ReturnType<typeof getOwnParticipant>> | null>(null)
const response = ref<CarrierRfxResponse | null>(null)
const ownAward = ref<Awaited<ReturnType<typeof getOwnAward>> | null>(null)
const offerAmount = ref<number | null>(null)
const offerAmountByLot = ref<Record<string, number | null>>({})
const lots = ref<RfxLot[]>([])
const lanesByLot = ref<Record<string, RfxLane[]>>({})
const companies = ref<Company[]>([])
const memberships = ref<UserCompanyMembership[]>([])
const selectedCarrierCompanyId = ref('')

const carrierMemberships = computed(() => filterCarrierMemberships(memberships.value))
const carrierOptions = computed(() => membershipSelectOptions(carrierMemberships.value))
const companyName = computed(() => {
  if (!event.value) return '—'
  return companies.value.find((c) => c.id === event.value!.owner_company_id)?.legal_name || event.value.owner_company_id
})
const ownResponseStatus = computed(() => response.value?.status ?? 'NOT_STARTED')
const deadlineExpired = computed(() => isDeadlineExpired(event.value?.response_deadline))
const remaining = computed(() => formatDeadlineRemaining(event.value?.response_deadline))

const showCreate = computed(() =>
  event.value
    ? canCreateResponse(event.value.status, ownResponseStatus.value, event.value.response_deadline)
    : false,
)
const showSubmit = computed(() =>
  event.value && response.value
    ? canSubmitResponse(response.value.status, event.value.status, event.value.response_deadline)
    : false,
)
const showOfferEdit = computed(() => response.value ? canEditCommercial(response.value.status) : false)
const defaultCurrency = computed(() => event.value?.currency_code || 'RUB')

function statusLabel(status: string) {
  const key = `carrierTenders.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

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

async function loadResponse() {
  if (!selectedCarrierCompanyId.value) {
    response.value = null
    return
  }
  try {
    response.value = await getOwnResponse(eventId.value, selectedCarrierCompanyId.value)
    offerAmountByLot.value = {}
    if (lots.value.length > 0) {
      for (const lot of lots.value) {
        const line = response.value.offer_lines?.find((item) => item.rfx_lot_id === lot.id)
        offerAmountByLot.value[lot.id] = line?.amount ?? null
      }
    } else {
      const line = response.value.offer_lines?.[0]
      offerAmount.value = line?.amount ?? null
    }
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      response.value = null
      return
    }
    throw error
  }
}

async function loadWorkspace() {
  loading.value = true
  notFound.value = false
  permissionDenied.value = false
  apiUnavailable.value = false
  try {
    if (selectedCarrierCompanyId.value) {
      setCompany(selectedCarrierCompanyId.value)
    }
    event.value = await getTender(eventId.value)
    participant.value = await getOwnParticipant(eventId.value, selectedCarrierCompanyId.value || undefined)
    lots.value = await listLots(eventId.value)
    const laneEntries: Record<string, RfxLane[]> = {}
    for (const lot of lots.value) {
      laneEntries[lot.id] = await listLanes(lot.id, selectedCarrierCompanyId.value || undefined)
    }
    lanesByLot.value = laneEntries
    await loadResponse()
    try {
      ownAward.value = await getOwnAward(eventId.value, selectedCarrierCompanyId.value || undefined)
    } catch {
      ownAward.value = null
    }
  } catch (error) {
    event.value = null
    lots.value = []
    lanesByLot.value = {}
    response.value = null
    participant.value = null
    if (shouldShowNotFound(error)) {
      notFound.value = true
    } else if (error instanceof ApiError && error.status === 403) {
      permissionDenied.value = true
    } else {
      apiUnavailable.value = isCarrierApiUnavailable(error)
      if (!apiUnavailable.value) {
        pushToast('error', error instanceof Error ? error.message : t('carrierTenders.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

async function handleCreateResponse() {
  if (!selectedCarrierCompanyId.value) return
  acting.value = true
  try {
    response.value = await createResponse(eventId.value, selectedCarrierCompanyId.value)
    pushToast('success', t('carrierTenders.detail.responseCreated'))
    await loadWorkspace()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

async function handleSaveOffer() {
  if (!response.value?.id) return
  acting.value = true
  try {
    if (lots.value.length > 0) {
      const missing = lots.value.some((lot) => {
        const amount = offerAmountByLot.value[lot.id]
        return amount == null || Number.isNaN(amount)
      })
      if (missing) {
        pushToast('error', t('carrierTenders.detail.offerLotRequired'))
        return
      }
    }
    const lines = lots.value.length
      ? lots.value.map((lot) => ({
          rfx_lot_id: lot.id,
          amount: offerAmountByLot.value[lot.id] as number,
          currency_code: defaultCurrency.value,
        }))
      : [{ amount: offerAmount.value ?? 0, currency_code: defaultCurrency.value }]
    if (!lots.value.length && offerAmount.value == null) return
    response.value = await updateResponseCommercial(response.value.id, lines)
    pushToast('success', t('carrierTenders.detail.offerSaved'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

function offerLineForLot(lotId: string) {
  return response.value?.offer_lines?.find((line) => line.rfx_lot_id === lotId)
}

async function handleSubmitResponse() {
  if (!response.value?.id) return
  if (!window.confirm(t('carrierTenders.detail.submitConfirm'))) return
  acting.value = true
  try {
    response.value = await submitResponse(response.value.id)
    pushToast('success', t('carrierTenders.detail.responseSubmitted'))
    await loadWorkspace()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

watch(selectedCarrierCompanyId, () => loadWorkspace())

onMounted(async () => {
  await Promise.all([loadMemberships(), loadCompanies()])
  await loadWorkspace()
})

let deadlineTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  deadlineTimer = setInterval(() => {
    if (deadlineExpired.value && event.value) {
      loadWorkspace()
    }
  }, 60000)
})
onUnmounted(() => {
  if (deadlineTimer) clearInterval(deadlineTimer)
})
</script>

<template>
  <div>
    <nav class="breadcrumb" aria-label="Breadcrumb">
      <NuxtLink to="/carrier/tenders">{{ t('carrierTenders.title') }}</NuxtLink>
      <span aria-hidden="true"> / </span>
      <span>{{ event?.rfx_number || eventId }}</span>
    </nav>

    <PageHeader
      :title="event?.title || t('carrierTenders.detail.title')"
      :subtitle="event?.rfx_number"
    >
      <template #actions>
        <Button variant="secondary" @click="$router.push('/carrier/tenders')">{{ t('common.back') }}</Button>
      </template>
    </PageHeader>

    <Select
      v-if="carrierOptions.length > 1"
      v-model="selectedCarrierCompanyId"
      class="mb-4"
      :label="t('carrierTenders.companyLabel')"
      :options="carrierOptions"
    />

    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('carrierTenders.notAvailable')" />
    <EmptyState v-else-if="permissionDenied" :title="t('carrierTenders.permissionDenied')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('common.error')" />

    <template v-else-if="event">
      <Card class="mb-4">
        <h2>{{ t('carrierTenders.detail.overview') }}</h2>
        <dl class="detail-grid">
          <dt>{{ t('carrierTenders.detail.buyer') }}</dt>
          <dd>{{ companyName }}</dd>
          <dt>{{ t('carrierTenders.columns.status') }}</dt>
          <dd><Badge :status="event.status" /></dd>
          <dt>{{ t('carrierTenders.detail.deadline') }}</dt>
          <dd>
            <span :class="{ 'text-danger': deadlineExpired }">{{ formatRfxDate(event.response_deadline) }}</span>
            <span v-if="deadlineExpired" class="muted"> — {{ t('carrierTenders.detail.deadlineExpired') }}</span>
            <span v-else-if="remaining" class="muted"> — {{ t('carrierTenders.detail.timeRemaining', { value: remaining }) }}</span>
          </dd>
          <dt>{{ t('carrierTenders.detail.participantStatus') }}</dt>
          <dd>{{ statusLabel(participant?.status || 'INVITED') }}</dd>
          <dt>{{ t('carrierTenders.columns.ownResponse') }}</dt>
          <dd>{{ statusLabel(ownResponseStatus) }}</dd>
        </dl>
        <p v-if="event.description" class="description">{{ event.description }}</p>
      </Card>

      <Card class="mb-4">
        <h2>{{ t('carrierTenders.detail.lots') }}</h2>
        <EmptyState v-if="lots.length === 0" :title="t('carrierTenders.detail.noLots')" />
        <div v-for="lot in lots" :key="lot.id" class="lot-block">
          <h3>{{ lot.lot_number }} — {{ lot.name }}</h3>
          <p v-if="lot.description" class="muted">{{ lot.description }}</p>
          <h4>{{ t('carrierTenders.detail.lanes') }}</h4>
          <EmptyState
            v-if="!(lanesByLot[lot.id]?.length)"
            :title="t('carrierTenders.detail.noLanes')"
          />
          <Table
            v-else
            :columns="[
              t('carrierTenders.detail.origin'),
              t('carrierTenders.detail.destination'),
              t('carrierTenders.detail.transportMode'),
              t('carrierTenders.detail.equipment'),
              t('carrierTenders.detail.volume'),
              t('carrierTenders.detail.serviceLevel'),
            ]"
          >
            <tr v-for="lane in lanesByLot[lot.id]" :key="lane.id">
              <td>{{ lane.origin_location_id || '—' }}</td>
              <td>{{ lane.destination_location_id || '—' }}</td>
              <td>{{ lane.transport_mode }}</td>
              <td>{{ lane.equipment_type || '—' }}</td>
              <td>{{ lane.estimated_volume != null ? `${lane.estimated_volume} ${lane.volume_unit || ''}`.trim() : '—' }}</td>
              <td>{{ lane.required_service_level || '—' }}</td>
            </tr>
          </Table>
          <div v-if="response && lots.length > 0" class="lot-offer">
            <h4>{{ t('carrierTenders.detail.lotOffer') }}</h4>
            <p v-if="!showOfferEdit" class="muted">
              {{ formatMoney(offerLineForLot(lot.id)?.amount, offerLineForLot(lot.id)?.currency_code || defaultCurrency) }}
            </p>
            <Input
              v-else
              v-model.number="offerAmountByLot[lot.id]"
              type="number"
              min="0"
              step="0.01"
              :label="t('carrierTenders.detail.offerAmountForLot', { lot: lot.lot_number })"
            />
          </div>
        </div>
      </Card>

      <Card>
        <h2>{{ t('carrierTenders.detail.response') }}</h2>
        <p v-if="!response" class="muted">{{ t('carrierTenders.detail.noResponse') }}</p>
        <p v-else-if="response.status === 'SUBMITTED'" class="muted">{{ t('carrierTenders.detail.responseReadOnly') }}</p>
        <dl v-if="response" class="detail-grid">
          <dt>ID</dt>
          <dd>{{ response.id }}</dd>
          <dt>{{ t('carrierTenders.columns.ownResponse') }}</dt>
          <dd>{{ statusLabel(response.status) }}</dd>
          <dt>{{ t('carrierTenders.status.SUBMITTED') }}</dt>
          <dd>{{ formatRfxDate(response.submitted_at) }}</dd>
          <dt>{{ t('carrierTenders.detail.offerAmount') }}</dt>
          <dd v-if="!showOfferEdit && lots.length === 0">{{ formatMoney(response.offer_lines?.[0]?.amount, response.offer_lines?.[0]?.currency_code || defaultCurrency) }}</dd>
          <dd v-else-if="showOfferEdit && lots.length === 0">
            <Input v-model.number="offerAmount" type="number" min="0" step="0.01" :label="t('carrierTenders.detail.offerAmount')" />
            <span class="muted">{{ defaultCurrency }}</span>
          </dd>
          <dd v-else-if="lots.length > 0" class="muted">{{ t('carrierTenders.detail.lotOfferHint') }}</dd>
        </dl>
        <div v-if="ownAward" class="award-result">
          <Badge status="AWARDED" />
          <span>{{ t('carrierTenders.detail.awarded') }}</span>
        </div>
        <div class="actions">
          <Button v-if="showOfferEdit" :disabled="acting" variant="secondary" @click="handleSaveOffer">
            {{ t('carrierTenders.detail.saveOffer') }}
          </Button>
          <Button v-if="showCreate" :disabled="acting" @click="handleCreateResponse">
            {{ t('carrierTenders.detail.createResponse') }}
          </Button>
          <Button v-if="showSubmit" :disabled="acting" @click="handleSubmitResponse">
            {{ t('carrierTenders.detail.submitResponse') }}
          </Button>
        </div>
        <p class="muted decline-note">{{ t('carrierTenders.detail.declineNotSupported') }}</p>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.breadcrumb {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
}

.breadcrumb a {
  color: var(--color-primary);
  text-decoration: none;
}

.detail-grid {
  display: grid;
  grid-template-columns: minmax(140px, 220px) 1fr;
  gap: 0.5rem 1rem;
  margin: 1rem 0;
}

.detail-grid dt {
  color: var(--color-text-muted);
}

.lot-block + .lot-block {
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid var(--color-border);
}

.lot-offer {
  margin-top: 1rem;
}

.actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1rem;
}

.muted {
  color: var(--color-text-muted);
}

.text-danger {
  color: var(--color-danger);
}

.decline-note {
  margin-top: 1rem;
  font-size: 0.875rem;
}

.description {
  margin-top: 1rem;
}

.mb-4 {
  margin-bottom: 1rem;
}
</style>
