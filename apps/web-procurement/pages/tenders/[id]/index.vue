<script setup lang="ts">
import {
  canCancelStatus,
  canPublishStatus,
  formatRfxDate,
  isEditableStatus,
  toDatetimeLocal,
  toRFC3339,
  type RfxEvent,
  type RfxLot,
  type RfxParticipant,
} from '~/types/rfx'
import type { Company } from '~/types/company'
import { checkPublishReadiness } from '~/utils/publishReadiness'
import { shouldShowNotFound, isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getRfxEvent, updateRfxEvent, publishRfxEvent, cancelRfxEvent, listRfxParticipants, addRfxParticipant } =
  useRfxApi()
const { listLots, createLot, createLane } = useRfxLotsApi()
const { listCompanies } = useCompanies()
const { setCompany } = useTenantContext()
const { canManageTenders, canPublishTenders } = usePermissions()
const { pushToast } = useToast()
const { t } = useI18n()

const event = ref<RfxEvent | null>(null)
const lots = ref<RfxLot[]>([])
const participants = ref<RfxParticipant[]>([])
const companies = ref<Company[]>([])
const loading = ref(true)
const notFound = ref(false)
const apiUnavailable = ref(false)
const saving = ref(false)
const showEditModal = ref(false)
const showParticipantModal = ref(false)

const editForm = reactive({ title: '', description: '', response_deadline: '' })
const lotForm = reactive({ lot_number: 'LOT-1', name: '', description: '' })
const laneForm = reactive({
  lot_id: '',
  origin_location_id: '',
  destination_location_id: '',
  transport_mode: 'ROAD',
})
const participantForm = reactive({ company_id: '', participant_type: 'CARRIER' })

const eventId = computed(() => String(route.params.id))

const companyName = computed(() => {
  if (!event.value) return '—'
  return companies.value.find((company) => company.id === event.value!.owner_company_id)?.legal_name || event.value.owner_company_id
})

const carrierOptions = computed(() =>
  companies.value
    .filter((company) => company.company_type === 'CARRIER')
    .map((company) => ({ label: company.legal_name, value: company.id })),
)

const publishReadiness = computed(() =>
  checkPublishReadiness({
    title: event.value?.title ?? '',
    lotCount: lots.value.length,
    responseDeadline: event.value?.response_deadline,
    participantCount: participants.value.length,
    rfxType: event.value?.rfx_type ?? 'LANE_TENDER',
  }),
)

async function loadCompanies() {
  try {
    const data = await listCompanies({ limit: 200, status: 'ACTIVE' })
    companies.value = data.items
  } catch {
    companies.value = []
  }
}

async function loadWorkspace() {
  loading.value = true
  notFound.value = false
  apiUnavailable.value = false
  try {
    event.value = await getRfxEvent(eventId.value)
    setCompany(event.value.owner_company_id)
    lots.value = await listLots(eventId.value)
    participants.value = await listRfxParticipants(eventId.value)
  } catch (error) {
    event.value = null
    lots.value = []
    participants.value = []
    if (shouldShowNotFound(error)) {
      notFound.value = true
    } else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) {
        pushToast('error', error instanceof Error ? error.message : t('tenders.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

function openEdit() {
  if (!event.value) return
  editForm.title = event.value.title
  editForm.description = event.value.description || ''
  editForm.response_deadline = toDatetimeLocal(event.value.response_deadline)
  showEditModal.value = true
}

async function saveEdit() {
  if (!event.value) return
  saving.value = true
  try {
    event.value = await updateRfxEvent(event.value.id, {
      title: editForm.title,
      description: editForm.description,
      response_deadline: toRFC3339(editForm.response_deadline),
    })
    pushToast('success', t('tenders.updated'))
    showEditModal.value = false
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function addLot() {
  if (!event.value || !lotForm.name.trim()) return
  saving.value = true
  try {
    const lot = await createLot(event.value.id, {
      lot_number: lotForm.lot_number,
      name: lotForm.name,
      description: lotForm.description,
      category: event.value.category,
      currency_code: event.value.currency_code || undefined,
    })
    lots.value = [...lots.value, lot]
    lotForm.lot_number = `LOT-${lots.value.length + 1}`
    lotForm.name = ''
    lotForm.description = ''
    if (!laneForm.lot_id) laneForm.lot_id = lot.id
    pushToast('success', t('tenders.lotAdded'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function addLane() {
  if (!laneForm.lot_id || !laneForm.origin_location_id || !laneForm.destination_location_id) return
  saving.value = true
  try {
    await createLane(laneForm.lot_id, {
      origin_location_id: laneForm.origin_location_id,
      destination_location_id: laneForm.destination_location_id,
      transport_mode: laneForm.transport_mode,
    })
    pushToast('success', t('tenders.laneAdded'))
    laneForm.origin_location_id = ''
    laneForm.destination_location_id = ''
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function submitParticipant() {
  if (!event.value || !participantForm.company_id) return
  saving.value = true
  try {
    const participant = await addRfxParticipant(event.value.id, {
      company_id: participantForm.company_id,
      participant_type: participantForm.participant_type,
    })
    participants.value = [...participants.value, participant]
    participantForm.company_id = ''
    showParticipantModal.value = false
    pushToast('success', t('tenders.participantAdded'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function publishTender() {
  if (!event.value || !canPublishTenders()) return
  if (!publishReadiness.value.ready) {
    pushToast('error', t('tenders.validation.publishNotReady'))
    return
  }
  if (publishReadiness.value.warnings.includes('participants')) {
    pushToast('info', t('tenders.warnings.noParticipants'))
  }
  saving.value = true
  try {
    await publishRfxEvent(event.value.id)
    pushToast('success', t('tenders.publishSuccess'))
    await loadWorkspace()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function cancelTender() {
  if (!event.value) return
  saving.value = true
  try {
    await cancelRfxEvent(event.value.id)
    pushToast('success', t('tenders.cancelSuccess'))
    await loadWorkspace()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

watch(eventId, loadWorkspace, { immediate: true })
onMounted(loadCompanies)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/tenders">{{ $t('tenders.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('tenders.details') }}</span>
    </nav>

    <PageHeader :title="event?.title || $t('tenders.details')">
      <template #actions>
        <Button variant="secondary" @click="$router.push('/tenders')">{{ $t('common.back') }}</Button>
        <Button
          v-if="event && isEditableStatus(event.status) && canManageTenders()"
          variant="secondary"
          @click="openEdit"
        >
          {{ $t('common.edit') }}
        </Button>
        <Button
          v-if="event && canManageTenders()"
          variant="secondary"
          @click="$router.push(`/tenders/${eventId}/evaluation`)"
        >
          {{ $t('tenders.evaluation.title') }}
        </Button>
        <Button
          v-if="event && canPublishStatus(event.status) && canPublishTenders()"
          :disabled="!publishReadiness.ready"
          :loading="saving"
          @click="publishTender"
        >
          {{ $t('tenders.publish') }}
        </Button>
        <Button
          v-if="event && canCancelStatus(event.status) && canManageTenders()"
          variant="danger"
          :loading="saving"
          @click="cancelTender"
        >
          {{ $t('tenders.cancel') }}
        </Button>
      </template>
    </PageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="$t('tenders.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="$t('tenders.loadFailed')" />
    <EmptyState v-else-if="!event" :title="$t('tenders.empty')" />

    <template v-else>
      <Card>
        <template #header>
          <h3>{{ $t('tenders.overview') }}</h3>
        </template>
        <dl class="detail-grid">
          <dt>{{ $t('tenders.number') }}</dt>
          <dd>{{ event.rfx_number }}</dd>
          <dt>{{ $t('tenders.type') }}</dt>
          <dd>{{ event.rfx_type }}</dd>
          <dt>{{ $t('tenders.ownerCompany') }}</dt>
          <dd>{{ companyName }}</dd>
          <dt>{{ $t('common.status') }}</dt>
          <dd><Badge :status="event.status" /></dd>
          <dt>{{ $t('tenders.deadline') }}</dt>
          <dd>{{ formatRfxDate(event.response_deadline) }}</dd>
          <dt>{{ $t('tenders.description') }}</dt>
          <dd>{{ event.description || '—' }}</dd>
        </dl>
      </Card>

      <Card>
        <template #header>
          <div class="section-header">
            <h3>{{ $t('tenders.lotsTitle') }}</h3>
            <Button
              v-if="isEditableStatus(event.status) && canManageTenders()"
              size="sm"
              @click="addLot"
            >
              {{ $t('tenders.addLot') }}
            </Button>
          </div>
        </template>
        <div v-if="isEditableStatus(event.status)" class="form-grid form-grid--2">
          <Input v-model="lotForm.lot_number" :label="$t('tenders.lotNumber')" />
          <Input v-model="lotForm.name" :label="$t('tenders.lotName')" />
          <Input v-model="lotForm.description" :label="$t('tenders.lotDescription')" />
        </div>
        <ul v-if="lots.length" class="item-list">
          <li v-for="lot in lots" :key="lot.id">{{ lot.lot_number }} — {{ lot.name }}</li>
        </ul>
        <EmptyState v-else :title="$t('tenders.noLots')" />

        <div v-if="isEditableStatus(event.status) && lots.length" class="form-grid form-grid--2">
          <Select
            v-model="laneForm.lot_id"
            :label="$t('tenders.lot')"
            :options="lots.map((lot) => ({ label: lot.name, value: lot.id }))"
          />
          <Input v-model="laneForm.origin_location_id" :label="$t('tenders.originLocationId')" />
          <Input v-model="laneForm.destination_location_id" :label="$t('tenders.destinationLocationId')" />
          <Button variant="secondary" :loading="saving" @click="addLane">{{ $t('tenders.addLane') }}</Button>
        </div>
      </Card>

      <Card>
        <template #header>
          <div class="section-header">
            <h3>{{ $t('tenders.participantsTitle') }}</h3>
            <Button
              v-if="isEditableStatus(event.status) && canManageTenders()"
              size="sm"
              @click="showParticipantModal = true"
            >
              {{ $t('tenders.addParticipant') }}
            </Button>
          </div>
        </template>
        <Table
          v-if="participants.length"
          :columns="[$t('tenders.participantCompany'), $t('tenders.participantType'), $t('common.status')]"
        >
          <tr v-for="participant in participants" :key="participant.id">
            <td>{{ participant.company_id }}</td>
            <td>{{ participant.participant_type }}</td>
            <td>{{ participant.status }}</td>
          </tr>
        </Table>
        <EmptyState v-else :title="$t('tenders.noParticipants')" />
      </Card>

      <Card>
        <template #header>
          <h3>{{ $t('tenders.responseProgress') }}</h3>
        </template>
        <p class="text-muted">{{ $t('tenders.responseProgressHint') }}</p>
        <dl class="detail-grid">
          <dt>{{ $t('tenders.participantsTitle') }}</dt>
          <dd>{{ participants.length }}</dd>
          <dt>{{ $t('tenders.lotsTitle') }}</dt>
          <dd>{{ lots.length }}</dd>
          <dt>{{ $t('tenders.publishReady') }}</dt>
          <dd>{{ publishReadiness.ready ? $t('common.yes') : $t('common.no') }}</dd>
        </dl>
      </Card>
    </template>

    <Modal :open="showEditModal" :title="$t('common.edit')" @close="showEditModal = false">
      <div class="form-grid">
        <Input v-model="editForm.title" :label="$t('tenders.titleLabel')" required />
        <Input v-model="editForm.description" :label="$t('tenders.description')" />
        <Input
          v-model="editForm.response_deadline"
          type="datetime-local"
          :label="$t('tenders.deadline')"
        />
      </div>
      <template #footer>
        <Button variant="secondary" @click="showEditModal = false">{{ $t('common.cancel') }}</Button>
        <Button :loading="saving" @click="saveEdit">{{ $t('common.save') }}</Button>
      </template>
    </Modal>

    <Modal :open="showParticipantModal" :title="$t('tenders.addParticipant')" @close="showParticipantModal = false">
      <div class="form-grid">
        <Select
          v-model="participantForm.company_id"
          :label="$t('tenders.participantCompany')"
          :options="carrierOptions"
        />
        <Select
          v-model="participantForm.participant_type"
          :label="$t('tenders.participantType')"
          :options="[
            { label: 'CARRIER', value: 'CARRIER' },
            { label: 'FORWARDER', value: 'FORWARDER' },
          ]"
        />
      </div>
      <template #footer>
        <Button variant="secondary" @click="showParticipantModal = false">{{ $t('common.cancel') }}</Button>
        <Button :loading="saving" @click="submitParticipant">{{ $t('common.save') }}</Button>
      </template>
    </Modal>
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

.detail-grid {
  display: grid;
  grid-template-columns: 12rem 1fr;
  gap: 0.75rem 1rem;
  margin: 0;
}

.detail-grid dt {
  color: var(--color-text-muted);
  font-weight: 500;
}

.detail-grid dd {
  margin: 0;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.item-list {
  margin: 0;
  padding-left: 1.25rem;
}
</style>
