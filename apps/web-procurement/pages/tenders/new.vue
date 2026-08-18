<script setup lang="ts">
import {
  RFX_CATEGORIES,
  RFX_TYPES,
  emptyTenderWizardForm,
  toRFC3339,
  type RfxEvent,
  type RfxLot,
  type RfxParticipant,
} from '~/types/rfx'
import {
  filterBuyerMemberships,
  membershipSelectOptions,
  selectDefaultOwnerCompany,
} from '~/utils/companyMembership'
import { checkPublishReadiness } from '~/utils/publishReadiness'

definePageMeta({ middleware: 'auth', layout: 'default' })

const STEPS = ['general', 'lots', 'participants', 'response', 'review'] as const
type WizardStep = (typeof STEPS)[number]

const { user } = useAuth()
const { getUserCompanies } = useCompanies()
const { listCompanies } = useCompanies()
const { createRfxEvent, updateRfxEvent, publishRfxEvent, addRfxParticipant } = useRfxApi()
const { listLots, createLot, createLane } = useRfxLotsApi()
const { setCompany } = useTenantContext()
const { canPublishTenders } = usePermissions()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const currentStep = ref<WizardStep>('general')
const saving = ref(false)
const draftEvent = ref<RfxEvent | null>(null)
const lots = ref<RfxLot[]>([])
const participants = ref<RfxParticipant[]>([])
const carrierOptions = ref<Array<{ label: string; value: string }>>([])

const form = reactive(emptyTenderWizardForm())
const ownerOptions = ref<Array<{ label: string; value: string }>>([])

const lotForm = reactive({
  lot_number: 'LOT-1',
  name: '',
  description: '',
})

const laneForm = reactive({
  lot_id: '',
  origin_location_id: '',
  destination_location_id: '',
  transport_mode: 'ROAD',
})

const participantForm = reactive({
  company_id: '',
  participant_type: 'CARRIER',
})

const stepIndex = computed(() => STEPS.indexOf(currentStep.value))

const publishReadiness = computed(() =>
  checkPublishReadiness({
    title: form.title,
    lotCount: lots.value.length,
    responseDeadline: form.response_deadline,
    participantCount: participants.value.length,
    rfxType: form.rfx_type,
  }),
)

const typeOptions = computed(() => RFX_TYPES.map((value) => ({ label: value, value })))
const categoryOptions = computed(() => RFX_CATEGORIES.map((value) => ({ label: value, value })))

function stepLabel(step: WizardStep) {
  return t(`tenders.wizard.${step}`)
}

async function loadOwnerCompanies() {
  if (!user.value?.id) return
  try {
    const memberships = filterBuyerMemberships(await getUserCompanies(user.value.id))
    ownerOptions.value = membershipSelectOptions(memberships)
    if (!form.owner_company_id) {
      form.owner_company_id = selectDefaultOwnerCompany(memberships)
    }
  } catch {
    ownerOptions.value = []
  }
}

async function loadCarriers() {
  try {
    const data = await listCompanies({ limit: 100, company_type: 'CARRIER', status: 'ACTIVE' })
    carrierOptions.value = data.items.map((company) => ({
      label: company.legal_name,
      value: company.id,
    }))
  } catch {
    carrierOptions.value = []
  }
}

async function ensureDraft() {
  if (draftEvent.value) return draftEvent.value

  saving.value = true
  try {
    const event = await createRfxEvent({
      rfx_number: form.rfx_number,
      rfx_type: form.rfx_type,
      category: form.category,
      title: form.title,
      description: form.description,
      owner_company_id: form.owner_company_id,
      currency_code: form.currency_code,
      valid_from: form.valid_from,
      valid_to: form.valid_to,
      response_deadline: toRFC3339(form.response_deadline),
    })
    draftEvent.value = event
    setCompany(form.owner_company_id)
    return event
  } finally {
    saving.value = false
  }
}

async function saveGeneral() {
  if (!form.title.trim() || !form.owner_company_id) {
    pushToast('error', t('tenders.validation.requiredFields'))
    return
  }

  saving.value = true
  try {
    if (draftEvent.value) {
      draftEvent.value = await updateRfxEvent(draftEvent.value.id, {
        title: form.title,
        description: form.description,
        currency_code: form.currency_code,
      })
    } else {
      await ensureDraft()
    }
    setCompany(form.owner_company_id)
    currentStep.value = 'lots'
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function addLot() {
  if (!lotForm.name.trim()) {
    pushToast('error', t('tenders.validation.lotNameRequired'))
    return
  }

  saving.value = true
  try {
    const event = await ensureDraft()
    const lot = await createLot(event.id, {
      lot_number: lotForm.lot_number,
      name: lotForm.name,
      description: lotForm.description,
      category: form.category,
      currency_code: form.currency_code,
    })
    lots.value = [...lots.value, lot]
    lotForm.lot_number = `LOT-${lots.value.length + 1}`
    lotForm.name = ''
    lotForm.description = ''
    if (!laneForm.lot_id) {
      laneForm.lot_id = lot.id
    }
    pushToast('success', t('tenders.lotAdded'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function addLane() {
  if (!laneForm.lot_id || !laneForm.origin_location_id || !laneForm.destination_location_id) {
    pushToast('error', t('tenders.validation.laneFieldsRequired'))
    return
  }

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

async function addParticipant() {
  if (!participantForm.company_id) {
    pushToast('error', t('tenders.validation.participantRequired'))
    return
  }

  saving.value = true
  try {
    const event = await ensureDraft()
    const participant = await addRfxParticipant(event.id, {
      company_id: participantForm.company_id,
      participant_type: participantForm.participant_type,
    })
    participants.value = [...participants.value, participant]
    participantForm.company_id = ''
    pushToast('success', t('tenders.participantAdded'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function saveResponseSettings() {
  if (!draftEvent.value) {
    await ensureDraft()
  }
  if (!draftEvent.value) return

  saving.value = true
  try {
    draftEvent.value = await updateRfxEvent(draftEvent.value.id, {
      response_deadline: toRFC3339(form.response_deadline),
      currency_code: form.currency_code,
    })
    currentStep.value = 'review'
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function publishTender() {
  if (!canPublishTenders()) {
    pushToast('error', t('tenders.validation.noPermission'))
    return
  }

  const readiness = publishReadiness.value
  if (!readiness.ready) {
    pushToast('error', t('tenders.validation.publishNotReady'))
    return
  }

  if (readiness.warnings.includes('participants')) {
    pushToast('info', t('tenders.warnings.noParticipants'))
  }

  saving.value = true
  try {
    const event = await ensureDraft()
    await publishRfxEvent(event.id)
    pushToast('success', t('tenders.publishSuccess'))
    await router.push(`/tenders/${event.id}`)
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

async function refreshLots() {
  if (!draftEvent.value) return
  lots.value = await listLots(draftEvent.value.id)
}

watch(currentStep, async (step) => {
  if (step === 'lots' && draftEvent.value) {
    await refreshLots()
  }
})

onMounted(async () => {
  await Promise.all([loadOwnerCompanies(), loadCarriers()])
})
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="$t('tenders.create')" :subtitle="$t('tenders.createHint')" />

    <div class="wizard-steps">
      <span
        v-for="(step, index) in STEPS"
        :key="step"
        class="wizard-step"
        :class="{
          'wizard-step--active': step === currentStep,
          'wizard-step--done': index < stepIndex,
        }"
      >
        {{ index + 1 }}. {{ stepLabel(step) }}
      </span>
    </div>

    <Card v-if="currentStep === 'general'">
      <div class="form-grid form-grid--2">
        <Input v-model="form.rfx_number" :label="$t('tenders.number')" required />
        <Select v-model="form.rfx_type" :label="$t('tenders.type')" :options="typeOptions" />
        <Select v-model="form.category" :label="$t('tenders.category')" :options="categoryOptions" />
        <Select
          v-model="form.owner_company_id"
          :label="$t('tenders.ownerCompany')"
          :options="ownerOptions"
        />
        <Input v-model="form.title" :label="$t('tenders.titleLabel')" required />
        <Input v-model="form.description" :label="$t('tenders.description')" />
      </div>
      <div class="wizard-actions">
        <Button variant="secondary" @click="$router.push('/tenders')">{{ $t('common.cancel') }}</Button>
        <Button :loading="saving" @click="saveGeneral">{{ $t('common.next') }}</Button>
      </div>
    </Card>

    <Card v-else-if="currentStep === 'lots'">
      <h3>{{ $t('tenders.lotsTitle') }}</h3>
      <div class="form-grid form-grid--2">
        <Input v-model="lotForm.lot_number" :label="$t('tenders.lotNumber')" />
        <Input v-model="lotForm.name" :label="$t('tenders.lotName')" required />
        <Input v-model="lotForm.description" :label="$t('tenders.lotDescription')" />
      </div>
      <div class="wizard-actions">
        <Button variant="secondary" :loading="saving" @click="addLot">{{ $t('tenders.addLot') }}</Button>
      </div>

      <ul v-if="lots.length" class="item-list">
        <li v-for="lot in lots" :key="lot.id">{{ lot.lot_number }} — {{ lot.name }}</li>
      </ul>
      <EmptyState v-else :title="$t('tenders.noLots')" />

      <h3>{{ $t('tenders.lanesTitle') }}</h3>
      <div class="form-grid form-grid--2">
        <Select
          v-model="laneForm.lot_id"
          :label="$t('tenders.lot')"
          :options="lots.map((lot) => ({ label: lot.name, value: lot.id }))"
        />
        <Select
          v-model="laneForm.transport_mode"
          :label="$t('tenders.transportMode')"
          :options="[
            { label: 'ROAD', value: 'ROAD' },
            { label: 'RAIL', value: 'RAIL' },
            { label: 'SEA', value: 'SEA' },
            { label: 'AIR', value: 'AIR' },
          ]"
        />
        <Input v-model="laneForm.origin_location_id" :label="$t('tenders.originLocationId')" />
        <Input v-model="laneForm.destination_location_id" :label="$t('tenders.destinationLocationId')" />
      </div>
      <div class="wizard-actions">
        <Button variant="secondary" :loading="saving" @click="addLane">{{ $t('tenders.addLane') }}</Button>
      </div>

      <div class="wizard-actions">
        <Button variant="secondary" @click="currentStep = 'general'">{{ $t('common.back') }}</Button>
        <Button @click="currentStep = 'participants'">{{ $t('common.next') }}</Button>
      </div>
    </Card>

    <Card v-else-if="currentStep === 'participants'">
      <div class="form-grid form-grid--2">
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
            { label: 'LSP', value: 'LSP' },
          ]"
        />
      </div>
      <div class="wizard-actions">
        <Button variant="secondary" :loading="saving" @click="addParticipant">
          {{ $t('tenders.addParticipant') }}
        </Button>
      </div>

      <ul v-if="participants.length" class="item-list">
        <li v-for="participant in participants" :key="participant.id">
          {{ participant.company_id }} ({{ participant.participant_type }})
        </li>
      </ul>
      <EmptyState v-else :title="$t('tenders.noParticipants')" />

      <div class="wizard-actions">
        <Button variant="secondary" @click="currentStep = 'lots'">{{ $t('common.back') }}</Button>
        <Button @click="currentStep = 'response'">{{ $t('common.next') }}</Button>
      </div>
    </Card>

    <Card v-else-if="currentStep === 'response'">
      <div class="form-grid form-grid--2">
        <Input v-model="form.currency_code" :label="$t('tenders.currency')" />
        <Input
          v-model="form.response_deadline"
          type="datetime-local"
          :label="$t('tenders.deadline')"
          required
        />
        <Input v-model="form.valid_from" type="date" :label="$t('tenders.validFrom')" />
        <Input v-model="form.valid_to" type="date" :label="$t('tenders.validTo')" />
      </div>
      <div class="wizard-actions">
        <Button variant="secondary" @click="currentStep = 'participants'">{{ $t('common.back') }}</Button>
        <Button :loading="saving" @click="saveResponseSettings">{{ $t('common.next') }}</Button>
      </div>
    </Card>

    <Card v-else>
      <h3>{{ $t('tenders.reviewTitle') }}</h3>
      <dl class="review-list">
        <dt>{{ $t('tenders.titleLabel') }}</dt>
        <dd>{{ form.title || '—' }}</dd>
        <dt>{{ $t('tenders.type') }}</dt>
        <dd>{{ form.rfx_type }}</dd>
        <dt>{{ $t('tenders.lotsTitle') }}</dt>
        <dd>{{ lots.length }}</dd>
        <dt>{{ $t('tenders.participantsTitle') }}</dt>
        <dd>{{ participants.length }}</dd>
        <dt>{{ $t('tenders.deadline') }}</dt>
        <dd>{{ form.response_deadline || '—' }}</dd>
      </dl>

      <ul v-if="publishReadiness.errors.length" class="readiness-errors">
        <li v-for="error in publishReadiness.errors" :key="error">
          {{ $t(`tenders.readiness.${error}`) }}
        </li>
      </ul>
      <p v-if="publishReadiness.warnings.includes('participants')" class="readiness-warning">
        {{ $t('tenders.warnings.noParticipants') }}
      </p>

      <div class="wizard-actions">
        <Button variant="secondary" @click="currentStep = 'response'">{{ $t('common.back') }}</Button>
        <Button
          :loading="saving"
          :disabled="!publishReadiness.ready || !canPublishTenders()"
          @click="publishTender"
        >
          {{ $t('tenders.publish') }}
        </Button>
      </div>
    </Card>
  </div>
</template>

<style scoped>
.wizard-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 1.25rem;
}

.item-list {
  margin: 1rem 0;
  padding-left: 1.25rem;
}

.review-list {
  display: grid;
  grid-template-columns: 10rem 1fr;
  gap: 0.5rem 1rem;
  margin: 0;
}

.review-list dt {
  font-weight: 600;
  color: var(--color-text-muted);
}

.review-list dd {
  margin: 0;
}

.readiness-errors {
  margin: 1rem 0 0;
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  background: #fee2e2;
  color: #991b1b;
}

.readiness-warning {
  margin: 1rem 0 0;
  color: var(--color-warning);
}
</style>
