<script setup lang="ts">
import type {
  LocationSummary,
  RateCard,
  RateCardVersion,
  RateComponent,
  RateComponentType,
  RateLine,
  TransportContract,
} from '~/types/contractRate'
import {
  availableComponentTypes,
  buildCreateRateLinePayload,
  buildPatchRateComponentPayload,
  buildPatchRateVersionPayload,
  buildRateComponentPayload,
  diffRateVersions,
  findSupersededPredecessor,
  isVersionEditable,
  validateLaneComponents,
  versionLifecycleActions,
} from '~/utils/contractRate'
import { isApiUnavailableError } from '~/utils/apiError'
import {
  deleteDraftRateComponent,
  deleteDraftRateLine,
  mapContractRateErrorCode,
  patchDraftRateComponent,
  patchDraftRateLine,
} from '~/utils/contractRateWorkspace'
import { formatDateTime } from '~/utils/format'

definePageMeta({ middleware: ['auth', 'contract-rate-workspace'], layout: 'default' })

const route = useRoute()
const contractId = computed(() => String(route.params.id))

const contractRatesApi = useContractRatesApi()
const {
  getTransportContract,
  listRateCards,
  createRateCard,
  listRateCardVersions,
  createRateCardVersion,
  patchRateCardVersion,
  discardRateCardVersion,
  activateRateCardVersion,
  listRateLines,
  createRateLine,
  patchRateLine,
  deleteRateLine,
  listRateComponents,
  createRateComponent,
  patchRateComponent,
  deleteRateComponent,
} = contractRatesApi

const { listLocations } = useLocationsApi()
const { pushToast } = useToast()
const { t } = useI18n()
const {
  canEditDraftRates,
  canActivateRateVersions,
  isCarrierContractReader,
} = usePermissions()

const loading = ref(true)
const apiUnavailable = ref(false)
const contract = ref<TransportContract | null>(null)
const rateCards = ref<RateCard[]>([])
const selectedCardId = ref<string | null>(null)
const versions = ref<RateCardVersion[]>([])
const selectedVersionId = ref<string | null>(null)
const lines = ref<RateLine[]>([])
const componentsByLine = ref<Record<string, RateComponent[]>>({})
const compareComponentsByLine = ref<Record<string, RateComponent[]>>({})
const compareLines = ref<RateLine[]>([])
const locations = ref<LocationSummary[]>([])

const showCreateCard = ref(false)
const showCreateVersion = ref(false)
const showEditVersion = ref(false)
const showLaneModal = ref(false)
const showComponentModal = ref(false)
const confirmDiscard = ref(false)
const confirmActivate = ref(false)
const confirmDeleteLine = ref(false)
const confirmDeleteComponent = ref(false)
const saving = ref(false)

const laneModalMode = ref<'create' | 'edit'>('create')
const componentModalMode = ref<'create' | 'edit'>('create')
const editingLineId = ref<string | null>(null)
const editingComponentId = ref<string | null>(null)
const pendingDeleteLineId = ref<string | null>(null)
const pendingDeleteComponentId = ref<string | null>(null)

const cardForm = reactive({ name: '', description: '' })
const versionForm = reactive({ valid_from: '', valid_to: '' })
const editVersionForm = reactive({ valid_from: '', valid_to: '' })
const laneForm = reactive({
  origin_location_id: '',
  destination_location_id: '',
  equipment_type: '',
  transport_mode: 'ROAD',
})
const componentForm = reactive({
  line_id: '',
  component_type: 'BASE_FREIGHT' as RateComponentType,
  amount: '',
  percent_value: '',
  unit_code: 'HOUR',
})

const selectedCard = computed(() => rateCards.value.find((c) => c.id === selectedCardId.value) ?? null)
const selectedVersion = computed(() => versions.value.find((v) => v.id === selectedVersionId.value) ?? null)
const canMutate = computed(() => canEditDraftRates() && !isCarrierContractReader())
const versionActions = computed(() =>
  selectedVersion.value ? versionLifecycleActions(selectedVersion.value.status) : [],
)
const laneValidationByLine = computed(() => {
  const map = new Map<string, string[]>()
  for (const line of lines.value) {
    map.set(line.id, validateLaneComponents(componentsByLine.value[line.id] ?? []))
  }
  return map
})
const versionDiff = computed(() => {
  if (!selectedVersion.value || selectedVersion.value.status !== 'DRAFT') return []
  const compareVersion = findSupersededPredecessor(selectedVersion.value, versions.value)
  if (!compareVersion) return []
  return diffRateVersions(lines.value, componentsByLine.value, compareLines.value, compareComponentsByLine.value)
})

const locationLabel = computed(() => {
  const map = new Map<string, string>()
  for (const loc of locations.value) {
    map.set(loc.id, [loc.name, loc.city, loc.region].filter(Boolean).join(', '))
  }
  return map
})

function supersedesLabel(version: RateCardVersion) {
  if (!version.supersedes_version_id) return '—'
  const predecessor = versions.value.find((v) => v.id === version.supersedes_version_id)
  return predecessor ? String(predecessor.version_number) : version.supersedes_version_id
}

function versionStatusLabel(status: string) {
  return t(`rates.statuses.${status}`)
}

function componentSummary(lineId: string): string {
  const components = componentsByLine.value[lineId] ?? []
  return components.map((c) => c.component_type).join(', ') || '—'
}

function addableComponentTypes(lineId: string): RateComponentType[] {
  return availableComponentTypes(componentsByLine.value[lineId] ?? [])
}

async function loadLocations() {
  const data = await listLocations({ limit: 500 })
  locations.value = data.items
}

async function loadContractAndCards() {
  loading.value = true
  apiUnavailable.value = false
  try {
    contract.value = await getTransportContract(contractId.value)
    rateCards.value = await listRateCards(contractId.value)
    if (!selectedCardId.value && rateCards.value[0]) selectedCardId.value = rateCards.value[0].id
    await loadLocations()
    if (selectedCardId.value) await loadVersions()
  } catch (error) {
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function loadVersions() {
  if (!selectedCardId.value) return
  versions.value = await listRateCardVersions(selectedCardId.value)
  if (!selectedVersionId.value && versions.value[0]) selectedVersionId.value = versions.value[0].id
  await loadLinesAndComponents()
}

async function loadLinesAndComponents() {
  if (!selectedVersionId.value) {
    lines.value = []
    componentsByLine.value = {}
    compareLines.value = []
    compareComponentsByLine.value = {}
    return
  }
  lines.value = await listRateLines(selectedVersionId.value)
  const componentEntries = await Promise.all(
    lines.value.map(async (line) => [line.id, await listRateComponents(line.id)] as const),
  )
  componentsByLine.value = Object.fromEntries(componentEntries)

  const compareVersion = selectedVersion.value
    ? findSupersededPredecessor(selectedVersion.value, versions.value)
    : undefined
  if (compareVersion) {
    compareLines.value = await listRateLines(compareVersion.id)
    const compareEntries = await Promise.all(
      compareLines.value.map(async (line) => [line.id, await listRateComponents(line.id)] as const),
    )
    compareComponentsByLine.value = Object.fromEntries(compareEntries)
  } else {
    compareLines.value = []
    compareComponentsByLine.value = {}
  }
}

async function submitCreateCard() {
  saving.value = true
  try {
    const card = await createRateCard(contractId.value, {
      name: cardForm.name.trim(),
      description: cardForm.description.trim() || null,
    })
    rateCards.value = [...rateCards.value, card]
    selectedCardId.value = card.id
    showCreateCard.value = false
    await loadVersions()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

async function submitCreateVersion() {
  if (!selectedCardId.value) return
  saving.value = true
  try {
    const version = await createRateCardVersion(selectedCardId.value, {
      valid_from: versionForm.valid_from,
      valid_to: versionForm.valid_to || null,
    })
    versions.value = [...versions.value, version]
    selectedVersionId.value = version.id
    showCreateVersion.value = false
    await loadLinesAndComponents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

function openEditVersion() {
  if (!selectedVersion.value) return
  editVersionForm.valid_from = selectedVersion.value.valid_from
  editVersionForm.valid_to = selectedVersion.value.valid_to ?? ''
  showEditVersion.value = true
}

async function submitEditVersion() {
  if (!selectedVersionId.value) return
  saving.value = true
  try {
    const updated = await patchRateCardVersion(
      selectedVersionId.value,
      buildPatchRateVersionPayload(editVersionForm),
    )
    const index = versions.value.findIndex((v) => v.id === selectedVersionId.value)
    if (index >= 0) versions.value[index] = updated
    showEditVersion.value = false
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

function openCreateLaneModal() {
  laneModalMode.value = 'create'
  editingLineId.value = null
  laneForm.origin_location_id = ''
  laneForm.destination_location_id = ''
  laneForm.equipment_type = ''
  laneForm.transport_mode = 'ROAD'
  showLaneModal.value = true
}

function openEditLaneModal(line: RateLine) {
  laneModalMode.value = 'edit'
  editingLineId.value = line.id
  laneForm.origin_location_id = line.origin_location_id
  laneForm.destination_location_id = line.destination_location_id
  laneForm.equipment_type = line.equipment_type
  laneForm.transport_mode = line.transport_mode
  showLaneModal.value = true
}

async function submitLane() {
  saving.value = true
  try {
    const payload = buildCreateRateLinePayload(laneForm)
    if (laneModalMode.value === 'create') {
      if (!selectedVersionId.value) return
      await createRateLine(selectedVersionId.value, payload)
    } else if (editingLineId.value) {
      await patchDraftRateLine(patchRateLine, editingLineId.value, laneForm)
    }
    showLaneModal.value = false
    await loadLinesAndComponents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

function requestDeleteLine(lineId: string) {
  pendingDeleteLineId.value = lineId
  confirmDeleteLine.value = true
}

async function confirmDeleteLineAction() {
  if (!pendingDeleteLineId.value) return
  saving.value = true
  try {
    await deleteDraftRateLine(deleteRateLine, pendingDeleteLineId.value)
    confirmDeleteLine.value = false
    pendingDeleteLineId.value = null
    await loadLinesAndComponents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

function openCreateComponentModal(lineId: string, componentType: RateComponentType) {
  componentModalMode.value = 'create'
  editingComponentId.value = null
  componentForm.line_id = lineId
  componentForm.component_type = componentType
  componentForm.amount = ''
  componentForm.percent_value = ''
  componentForm.unit_code = componentType === 'BASE_FREIGHT' ? '' : 'HOUR'
  showComponentModal.value = true
}

function openEditComponentModal(component: RateComponent) {
  componentModalMode.value = 'edit'
  editingComponentId.value = component.id
  componentForm.line_id = component.rate_line_id
  componentForm.component_type = component.component_type
  componentForm.amount = component.amount ?? ''
  componentForm.percent_value = component.percent_value ?? ''
  componentForm.unit_code = component.unit_code ?? 'HOUR'
  showComponentModal.value = true
}

async function submitComponent() {
  saving.value = true
  try {
    if (componentModalMode.value === 'create') {
      await createRateComponent(
        componentForm.line_id,
        buildRateComponentPayload(componentForm),
      )
    } else if (editingComponentId.value) {
      await patchDraftRateComponent(
        patchRateComponent,
        editingComponentId.value,
        buildPatchRateComponentPayload(componentForm),
      )
    }
    showComponentModal.value = false
    await loadLinesAndComponents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

function requestDeleteComponent(componentId: string) {
  pendingDeleteComponentId.value = componentId
  confirmDeleteComponent.value = true
}

async function confirmDeleteComponentAction() {
  if (!pendingDeleteComponentId.value) return
  saving.value = true
  try {
    await deleteDraftRateComponent(deleteRateComponent, pendingDeleteComponentId.value)
    confirmDeleteComponent.value = false
    pendingDeleteComponentId.value = null
    await loadLinesAndComponents()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

async function discardVersion() {
  if (!selectedVersionId.value) return
  saving.value = true
  try {
    await discardRateCardVersion(selectedVersionId.value)
    confirmDiscard.value = false
    selectedVersionId.value = null
    await loadVersions()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

async function activateVersion() {
  if (!selectedVersionId.value || !selectedCard.value || !contract.value) return
  const errors = lines.value.flatMap((line) => laneValidationByLine.value.get(line.id) ?? [])
  if (errors.length > 0) {
    pushToast('error', t(`rates.validation.${errors[0]}`))
    return
  }
  saving.value = true
  try {
    await activateRateCardVersion(selectedVersionId.value)
    confirmActivate.value = false
    pushToast('success', t('contracts.lifecycleSuccess'))
    await loadVersions()
  } catch (error) {
    const detailCode = mapContractRateErrorCode(error)
    pushToast('error', t(`contracts.errors.${detailCode}`) || (error instanceof Error ? error.message : t('contracts.loadFailed')))
  } finally {
    saving.value = false
  }
}

watch(selectedCardId, async (id) => {
  if (id) {
    selectedVersionId.value = null
    await loadVersions()
  }
})

watch(selectedVersionId, async () => {
  await loadLinesAndComponents()
})

onMounted(loadContractAndCards)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('rates.title')">
      <template v-if="contract" #subtitle>
        {{ contract.contract_number }} · {{ contract.currency_code }}
      </template>
      <template v-if="canMutate" #actions>
        <Button @click="showCreateCard = true">{{ t('rates.createCard') }}</Button>
      </template>
    </PageHeader>

    <EmptyState v-if="loading" :title="t('common.loading')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('contracts.backendUnavailable')" />
    <template v-else>
      <div class="rates-layout">
        <Card class="rates-panel">
          <h2>{{ t('rates.title') }}</h2>
          <EmptyState v-if="rateCards.length === 0" :title="t('rates.emptyCards')" />
          <ul v-else class="list-plain">
            <li v-for="card in rateCards" :key="card.id">
              <button
                type="button"
                class="list-button"
                :class="{ active: card.id === selectedCardId }"
                @click="selectedCardId = card.id"
              >
                {{ card.name }}
              </button>
            </li>
          </ul>
        </Card>

        <Card v-if="selectedCard" class="rates-panel">
          <div class="panel-header">
            <h2>{{ t('rates.versions') }}</h2>
            <Button
              v-if="canMutate"
              size="sm"
              variant="secondary"
              @click="showCreateVersion = true; versionForm.valid_from = contract?.valid_from ?? ''"
            >
              {{ t('rates.createVersion') }}
            </Button>
          </div>
          <table class="simple-table">
            <thead>
              <tr>
                <th>{{ t('rates.versionNumber') }}</th>
                <th>{{ t('contracts.status') }}</th>
                <th>{{ t('rates.validFrom') }}</th>
                <th>{{ t('rates.validTo') }}</th>
                <th>{{ t('rates.createdAt') }}</th>
                <th>{{ t('rates.activatedAt') }}</th>
                <th>{{ t('rates.supersedesVersion') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="version in versions"
                :key="version.id"
                :class="{ selected: version.id === selectedVersionId }"
                @click="selectedVersionId = version.id"
              >
                <td>{{ version.version_number }}</td>
                <td><Badge :status="version.status">{{ versionStatusLabel(version.status) }}</Badge></td>
                <td>{{ version.valid_from }}</td>
                <td>{{ version.valid_to ?? '—' }}</td>
                <td>{{ formatDateTime(version.created_at) }}</td>
                <td>{{ version.activated_at ? formatDateTime(version.activated_at) : '—' }}</td>
                <td>{{ supersedesLabel(version) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-if="selectedVersion" class="version-actions">
            <p v-if="!isVersionEditable(selectedVersion.status)" class="muted">{{ t('rates.readOnlyVersion') }}</p>
            <template v-else-if="canMutate">
              <Button
                v-if="versionActions.includes('edit')"
                variant="secondary"
                @click="openEditVersion"
              >
                {{ t('rates.editVersion') }}
              </Button>
              <Button
                v-if="versionActions.includes('discard')"
                variant="secondary"
                @click="confirmDiscard = true"
              >
                {{ t('rates.discard') }}
              </Button>
              <Button
                v-if="versionActions.includes('activate') && canActivateRateVersions()"
                @click="confirmActivate = true"
              >
                {{ t('rates.activate') }}
              </Button>
            </template>
          </div>
        </Card>
      </div>

      <Card v-if="selectedVersion">
        <div class="panel-header">
          <h2>{{ t('rates.lanes') }}</h2>
          <Button
            v-if="canMutate && isVersionEditable(selectedVersion.status)"
            size="sm"
            @click="openCreateLaneModal"
          >
            {{ t('rates.addLane') }}
          </Button>
        </div>
        <EmptyState v-if="lines.length === 0" :title="t('rates.emptyLanes')" />
        <div v-else class="table-scroll">
          <Table
            :columns="[
              t('rates.origin'),
              t('rates.destination'),
              t('rates.equipment'),
              t('rates.mode'),
              t('rates.components'),
              t('common.actions'),
            ]"
          >
            <tr v-for="line in lines" :key="line.id">
              <td>{{ locationLabel.get(line.origin_location_id) ?? line.origin_location_id }}</td>
              <td>{{ locationLabel.get(line.destination_location_id) ?? line.destination_location_id }}</td>
              <td>{{ line.equipment_type }}</td>
              <td>{{ line.transport_mode }}</td>
              <td>
                <ul class="component-list">
                  <li v-for="component in componentsByLine[line.id] ?? []" :key="component.id">
                    {{ component.component_type }}
                    <template v-if="canMutate && isVersionEditable(selectedVersion.status)">
                      <Button size="sm" variant="ghost" @click="openEditComponentModal(component)">
                        {{ t('rates.editComponent') }}
                      </Button>
                      <Button size="sm" variant="ghost" @click="requestDeleteComponent(component.id)">
                        {{ t('rates.deleteComponent') }}
                      </Button>
                    </template>
                  </li>
                </ul>
              </td>
              <td>
                <template v-if="canMutate && isVersionEditable(selectedVersion.status)">
                  <Button size="sm" variant="secondary" @click="openEditLaneModal(line)">
                    {{ t('rates.editLane') }}
                  </Button>
                  <Button size="sm" variant="danger" @click="requestDeleteLine(line.id)">
                    {{ t('rates.deleteLane') }}
                  </Button>
                  <Button
                    v-for="componentType in addableComponentTypes(line.id)"
                    :key="`${line.id}-${componentType}`"
                    size="sm"
                    variant="secondary"
                    @click="openCreateComponentModal(line.id, componentType)"
                  >
                    {{ t(`rates.${componentType === 'BASE_FREIGHT' ? 'baseFreight' : componentType === 'FUEL_SURCHARGE' ? 'fuelSurcharge' : componentType === 'WAITING' ? 'waiting' : 'detention'}`) }}
                  </Button>
                </template>
                <span v-if="(laneValidationByLine.get(line.id) ?? []).length" class="validation-error">
                  {{ t(`rates.validation.${laneValidationByLine.get(line.id)?.[0]}`) }}
                </span>
              </td>
            </tr>
          </Table>
        </div>
      </Card>

      <Card v-if="versionDiff.length > 0">
        <h2>{{ t('rates.versionDiff') }}</h2>
        <ul>
          <li v-for="entry in versionDiff" :key="entry.key">
            <strong>{{ t(`rates.${entry.change}`) }}</strong>: {{ entry.key }}
            <span v-if="entry.componentChanges?.length"> — {{ entry.componentChanges.join(', ') }}</span>
          </li>
        </ul>
      </Card>
    </template>

    <Modal :open="showCreateCard" :title="t('rates.createCard')" @close="showCreateCard = false">
      <Input v-model="cardForm.name" :label="t('rates.cardName')" required />
      <Input v-model="cardForm.description" :label="t('contracts.description')" />
      <template #footer>
        <Button variant="secondary" @click="showCreateCard = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="submitCreateCard">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>

    <Modal :open="showCreateVersion" :title="t('rates.createVersion')" @close="showCreateVersion = false">
      <Input v-model="versionForm.valid_from" type="date" :label="t('rates.validFrom')" required />
      <Input v-model="versionForm.valid_to" type="date" :label="t('rates.validTo')" />
      <template #footer>
        <Button variant="secondary" @click="showCreateVersion = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="submitCreateVersion">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>

    <Modal :open="showEditVersion" :title="t('rates.editVersion')" @close="showEditVersion = false">
      <Input v-model="editVersionForm.valid_from" type="date" :label="t('rates.validFrom')" required />
      <Input v-model="editVersionForm.valid_to" type="date" :label="t('rates.validTo')" />
      <template #footer>
        <Button variant="secondary" @click="showEditVersion = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="submitEditVersion">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>

    <Modal
      :open="showLaneModal"
      :title="laneModalMode === 'create' ? t('rates.addLane') : t('rates.editLane')"
      @close="showLaneModal = false"
    >
      <Select v-model="laneForm.origin_location_id" :label="t('rates.origin')" required>
        <option value="" disabled>{{ t('rates.origin') }}</option>
        <option v-for="loc in locations" :key="loc.id" :value="loc.id">
          {{ locationLabel.get(loc.id) }}
        </option>
      </Select>
      <Select v-model="laneForm.destination_location_id" :label="t('rates.destination')" required>
        <option value="" disabled>{{ t('rates.destination') }}</option>
        <option v-for="loc in locations" :key="loc.id" :value="loc.id">
          {{ locationLabel.get(loc.id) }}
        </option>
      </Select>
      <Input v-model="laneForm.equipment_type" :label="t('rates.equipment')" required />
      <Input v-model="laneForm.transport_mode" :label="t('rates.mode')" readonly />
      <template #footer>
        <Button variant="secondary" @click="showLaneModal = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="submitLane">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>

    <Modal
      :open="showComponentModal"
      :title="componentModalMode === 'create' ? t('rates.components') : t('rates.editComponent')"
      @close="showComponentModal = false"
    >
      <Input
        v-if="componentForm.component_type === 'BASE_FREIGHT'"
        v-model="componentForm.amount"
        :label="t('rates.amount')"
        required
      />
      <Input
        v-if="componentForm.component_type === 'FUEL_SURCHARGE'"
        v-model="componentForm.percent_value"
        :label="t('rates.percent')"
        required
      />
      <Input
        v-if="componentForm.component_type === 'WAITING' || componentForm.component_type === 'DETENTION'"
        v-model="componentForm.amount"
        :label="t('rates.amount')"
        required
      />
      <Input
        v-if="componentForm.component_type === 'WAITING' || componentForm.component_type === 'DETENTION'"
        v-model="componentForm.unit_code"
        :label="t('rates.unitCode')"
        required
      />
      <template #footer>
        <Button variant="secondary" @click="showComponentModal = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="submitComponent">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>

    <Modal :open="confirmDiscard" :title="t('rates.confirmDiscardTitle')" @close="confirmDiscard = false">
      <p>{{ t('rates.confirmDiscardBody') }}</p>
      <template #footer>
        <Button variant="secondary" @click="confirmDiscard = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" variant="danger" @click="discardVersion">{{ t('rates.discard') }}</Button>
      </template>
    </Modal>

    <Modal :open="confirmActivate" :title="t('rates.confirmActivateTitle')" @close="confirmActivate = false">
      <p>{{ t('rates.confirmActivateBody') }}</p>
      <p>
        {{
          t('rates.activationSummary', {
            card: selectedCard?.name ?? '',
            version: selectedVersion?.version_number ?? '',
            lanes: lines.length,
            currency: contract?.currency_code ?? '',
          })
        }}
      </p>
      <template #footer>
        <Button variant="secondary" @click="confirmActivate = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="activateVersion">{{ t('rates.activate') }}</Button>
      </template>
    </Modal>

    <Modal :open="confirmDeleteLine" :title="t('rates.confirmDeleteLaneTitle')" @close="confirmDeleteLine = false">
      <p>{{ t('rates.confirmDeleteLaneBody') }}</p>
      <template #footer>
        <Button variant="secondary" @click="confirmDeleteLine = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" variant="danger" @click="confirmDeleteLineAction">{{ t('rates.deleteLane') }}</Button>
      </template>
    </Modal>

    <Modal :open="confirmDeleteComponent" :title="t('rates.confirmDeleteComponentTitle')" @close="confirmDeleteComponent = false">
      <p>{{ t('rates.confirmDeleteComponentBody') }}</p>
      <template #footer>
        <Button variant="secondary" @click="confirmDeleteComponent = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" variant="danger" @click="confirmDeleteComponentAction">{{ t('rates.deleteComponent') }}</Button>
      </template>
    </Modal>
  </div>
</template>

<style scoped>
.rates-layout {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1rem;
}

.rates-panel h2 {
  margin-top: 0;
}

.list-plain {
  list-style: none;
  padding: 0;
  margin: 0;
}

.list-button {
  width: 100%;
  text-align: left;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  cursor: pointer;
  margin-bottom: 0.5rem;
}

.list-button.active {
  border-color: var(--color-primary);
  background: #eff6ff;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
}

.simple-table {
  width: 100%;
  border-collapse: collapse;
}

.simple-table th,
.simple-table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.simple-table tr.selected {
  background: #f8fafc;
  cursor: pointer;
}

.version-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
  flex-wrap: wrap;
}

.component-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.validation-error {
  color: var(--color-danger);
  font-size: 0.75rem;
  display: block;
  margin-top: 0.25rem;
}

.muted {
  color: var(--color-text-muted);
}
</style>
