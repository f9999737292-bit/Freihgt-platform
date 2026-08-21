<script setup lang="ts">
import type { Company } from '~/types/company'
import type { ContractStatus, TransportContract } from '~/types/contractRate'
import { ApiError } from '~/utils/apiClient'
import {
  canEditContractField,
  contractLifecycleActions,
  isContractDraftEditable,
  isContractMetadataEditable,
  isContractTerminal,
} from '~/utils/contractRate'
import { isApiUnavailableError, shouldShowNotFound } from '~/utils/apiError'

definePageMeta({ middleware: ['auth', 'contract-rate-workspace'], layout: 'default' })

const route = useRoute()
const contractId = computed(() => String(route.params.id))
const {
  getTransportContract,
  patchTransportContract,
  activateTransportContract,
  suspendTransportContract,
  reactivateTransportContract,
  terminateTransportContract,
  cancelTransportContract,
} = useContractRatesApi()
const { listCompanies } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()
const {
  canEditDraftContracts,
  canActivateContracts,
  canSuspendContracts,
  canTerminateContracts,
  isCarrierContractReader,
} = usePermissions()

const loading = ref(true)
const saving = ref(false)
const lifecycleLoading = ref(false)
const apiUnavailable = ref(false)
const notFound = ref(false)
const forbidden = ref(false)
const contract = ref<TransportContract | null>(null)
const companies = ref<Company[]>([])
const showEditModal = ref(false)
const confirmAction = ref<'activate' | 'suspend' | 'reactivate' | 'terminate' | 'cancel' | null>(null)

const editForm = reactive({
  name: '',
  description: '',
  external_reference: '',
  valid_to: '',
})

const companyNameById = computed(() => {
  const map = new Map<string, string>()
  for (const company of companies.value) map.set(company.id, company.legal_name)
  return map
})

const lifecycleActions = computed(() =>
  contract.value ? contractLifecycleActions(contract.value.status) : [],
)

const canMutate = computed(() => !isCarrierContractReader())

function statusLabel(status: ContractStatus) {
  return t(`contracts.statuses.${status}`)
}

function showAction(action: string): boolean {
  if (!canMutate.value) return false
  if (!lifecycleActions.value.includes(action)) return false
  if (action === 'edit') {
    return contract.value
      ? isContractDraftEditable(contract.value.status) || isContractMetadataEditable(contract.value.status)
      : false
  }
  if (action === 'activate' || action === 'cancel') return canActivateContracts()
  if (action === 'suspend' || action === 'reactivate') return canSuspendContracts()
  if (action === 'terminate') return canTerminateContracts()
  return false
}

async function loadContract() {
  loading.value = true
  apiUnavailable.value = false
  notFound.value = false
  forbidden.value = false
  try {
    const [detail, companyPage] = await Promise.all([
      getTransportContract(contractId.value),
      listCompanies({ limit: 200 }),
    ])
    contract.value = detail
    companies.value = companyPage.items
  } catch (error) {
    contract.value = null
    if (error instanceof ApiError && error.status === 403) forbidden.value = true
    else if (shouldShowNotFound(error)) notFound.value = true
    else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) {
        pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

function openEdit() {
  if (!contract.value) return
  editForm.name = contract.value.name
  editForm.description = contract.value.description ?? ''
  editForm.external_reference = contract.value.external_reference ?? ''
  editForm.valid_to = contract.value.valid_to ?? ''
  showEditModal.value = true
}

async function saveEdit() {
  if (!contract.value) return
  saving.value = true
  try {
    const payload: Record<string, string | null> = {}
    if (isContractDraftEditable(contract.value.status) && canEditContractField(contract.value.status, 'name')) {
      payload.name = editForm.name
    }
    if (canEditContractField(contract.value.status, 'description')) payload.description = editForm.description || null
    if (canEditContractField(contract.value.status, 'external_reference')) {
      payload.external_reference = editForm.external_reference || null
    }
    if (isContractDraftEditable(contract.value.status) && canEditContractField(contract.value.status, 'valid_to')) {
      payload.valid_to = editForm.valid_to || null
    }
    contract.value = await patchTransportContract(contract.value.id, payload)
    pushToast('success', t('contracts.updateSuccess'))
    showEditModal.value = false
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    saving.value = false
  }
}

async function runLifecycle(action: NonNullable<typeof confirmAction.value>) {
  if (!contract.value) return
  lifecycleLoading.value = true
  try {
    const id = contract.value.id
    if (action === 'activate') contract.value = await activateTransportContract(id)
    if (action === 'suspend') contract.value = await suspendTransportContract(id)
    if (action === 'reactivate') contract.value = await reactivateTransportContract(id)
    if (action === 'terminate') contract.value = await terminateTransportContract(id)
    if (action === 'cancel') contract.value = await cancelTransportContract(id)
    pushToast('success', t('contracts.lifecycleSuccess'))
    confirmAction.value = null
  } catch (error) {
    if (error instanceof ApiError) {
      const detailCode = String(error.details?.code ?? error.code ?? '')
      pushToast('error', t(`contracts.errors.${detailCode}`) || error.message)
    } else {
      pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
    }
  } finally {
    lifecycleLoading.value = false
  }
}

const confirmTitle = computed(() => {
  if (!confirmAction.value) return ''
  const map = {
    activate: 'contracts.confirmActivateTitle',
    suspend: 'contracts.confirmSuspendTitle',
    reactivate: 'contracts.confirmReactivateTitle',
    terminate: 'contracts.confirmTerminateTitle',
    cancel: 'contracts.confirmCancelTitle',
  } as const
  return t(map[confirmAction.value])
})

const confirmBody = computed(() => {
  if (!confirmAction.value) return ''
  const map = {
    activate: 'contracts.confirmActivateBody',
    suspend: 'contracts.confirmSuspendBody',
    reactivate: 'contracts.confirmReactivateBody',
    terminate: 'contracts.confirmTerminateBody',
    cancel: 'contracts.confirmCancelBody',
  } as const
  return t(map[confirmAction.value])
})

onMounted(loadContract)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="contract?.name ?? t('contracts.detailTitle')">
      <template v-if="contract" #subtitle>
        {{ contract.contract_number }}
      </template>
      <template v-if="contract && !isContractTerminal(contract.status)" #actions>
        <NuxtLink :to="`/contracts/${contract.id}/rates`" class="link-button">{{ t('contracts.ratesLink') }}</NuxtLink>
        <NuxtLink :to="`/contracts/${contract.id}/rates/simulate`" class="link-button">{{ t('contracts.simulateLink') }}</NuxtLink>
        <Button v-if="showAction('edit') && canEditDraftContracts()" variant="secondary" @click="openEdit">
          {{ t('contracts.edit') }}
        </Button>
        <Button v-if="showAction('activate')" @click="confirmAction = 'activate'">{{ t('contracts.activate') }}</Button>
        <Button v-if="showAction('suspend')" variant="secondary" @click="confirmAction = 'suspend'">{{ t('contracts.suspend') }}</Button>
        <Button v-if="showAction('reactivate')" @click="confirmAction = 'reactivate'">{{ t('contracts.reactivate') }}</Button>
        <Button v-if="showAction('terminate')" variant="danger" @click="confirmAction = 'terminate'">{{ t('contracts.terminate') }}</Button>
        <Button v-if="showAction('cancel')" variant="danger" @click="confirmAction = 'cancel'">{{ t('contracts.cancel') }}</Button>
      </template>
    </PageHeader>

    <EmptyState v-if="loading" :title="t('common.loading')" />
    <EmptyState v-else-if="notFound" :title="t('contracts.notFound')" />
    <EmptyState v-else-if="forbidden" :title="t('contracts.forbidden')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('contracts.backendUnavailable')" />
    <template v-else-if="contract">
      <Card>
        <h2>{{ t('contracts.overview') }}</h2>
        <dl class="detail-grid">
          <dt>{{ t('contracts.status') }}</dt>
          <dd><Badge :status="contract.status">{{ statusLabel(contract.status) }}</Badge></dd>
          <dt>{{ t('contracts.buyer') }}</dt>
          <dd>{{ companyNameById.get(contract.buyer_company_id) ?? contract.buyer_company_id }}</dd>
          <dt>{{ t('contracts.carrier') }}</dt>
          <dd>{{ companyNameById.get(contract.carrier_company_id) ?? contract.carrier_company_id }}</dd>
          <dt>{{ t('contracts.currency') }}</dt>
          <dd>{{ contract.currency_code }}</dd>
          <dt>{{ t('contracts.validFrom') }}</dt>
          <dd>{{ contract.valid_from }}</dd>
          <dt>{{ t('contracts.validTo') }}</dt>
          <dd>{{ contract.valid_to ?? '—' }}</dd>
          <dt>{{ t('contracts.externalReference') }}</dt>
          <dd>{{ contract.external_reference ?? '—' }}</dd>
          <dt>{{ t('contracts.description') }}</dt>
          <dd>{{ contract.description ?? '—' }}</dd>
        </dl>
      </Card>
    </template>

    <Modal :open="showEditModal" :title="t('contracts.edit')" @close="showEditModal = false">
      <div class="form-grid">
        <Input
          v-if="contract && isContractDraftEditable(contract.status)"
          v-model="editForm.name"
          :label="t('contracts.name')"
        />
        <Input v-model="editForm.external_reference" :label="t('contracts.externalReference')" />
        <Input v-model="editForm.description" :label="t('contracts.description')" />
        <Input
          v-if="contract && isContractDraftEditable(contract.status)"
          v-model="editForm.valid_to"
          type="date"
          :label="t('contracts.validTo')"
        />
      </div>
      <template #footer>
        <Button variant="secondary" @click="showEditModal = false">{{ t('common.cancel') }}</Button>
        <Button :loading="saving" @click="saveEdit">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>

    <Modal
      v-if="confirmAction"
      :open="!!confirmAction"
      :title="confirmTitle"
      @close="confirmAction = null"
    >
      <p>{{ confirmBody }}</p>
      <template #footer>
        <Button variant="secondary" @click="confirmAction = null">{{ t('common.cancel') }}</Button>
        <Button :loading="lifecycleLoading" @click="runLifecycle(confirmAction!)">{{ t('common.yes') }}</Button>
      </template>
    </Modal>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: 12rem 1fr;
  gap: 0.5rem 1rem;
}

.link-button {
  display: inline-flex;
  align-items: center;
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-md);
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  text-decoration: none;
  color: var(--color-primary);
}
</style>
