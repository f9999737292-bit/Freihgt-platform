<script setup lang="ts">
import type { Company } from '~/types/company'
import type { ContractStatus, TransportContract } from '~/types/contractRate'
import { CONTRACT_STATUSES } from '~/types/contractRate'
import { ApiError } from '~/utils/apiClient'
import { buildCreateContractPayload, filterContracts, paginateItems } from '~/utils/contractRate'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: ['auth', 'contract-rate-workspace'], layout: 'default' })

const { listTransportContracts, createTransportContract } = useContractRatesApi()
const { listCompanies } = useCompanies()
const { currentCompanyId } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()
const {
  canCreateContracts,
  canReadContracts,
  isCarrierContractReader,
} = usePermissions()

const loading = ref(true)
const apiUnavailable = ref(false)
const forbidden = ref(false)
const allItems = ref<TransportContract[]>([])
const carriers = ref<Company[]>([])
const showCreateModal = ref(false)
const creating = ref(false)

const filters = reactive({
  q: '',
  status: '' as ContractStatus | '',
  carrier_company_id: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const filteredItems = computed(() => filterContracts(allItems.value, filters))
const page = computed(() => paginateItems(filteredItems.value, pagination.limit, pagination.offset))
const items = computed(() => page.value.items)
const total = computed(() => page.value.total)
const missingCompany = computed(() => !currentCompanyId.value)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

const createForm = reactive({
  buyer_company_id: '',
  carrier_company_id: '',
  contract_number: '',
  name: '',
  external_reference: '',
  description: '',
  valid_from: '',
  valid_to: '',
  currency_code: 'RUB',
})

const companyNameById = computed(() => {
  const map = new Map<string, string>()
  for (const carrier of carriers.value) map.set(carrier.id, carrier.legal_name)
  return map
})

function statusLabel(status: ContractStatus) {
  return t(`contracts.statuses.${status}`)
}

async function loadCarriers() {
  const data = await listCompanies({ company_type: 'CARRIER', limit: 200 })
  carriers.value = data.items
}

async function loadContracts() {
  loading.value = true
  apiUnavailable.value = false
  forbidden.value = false
  try {
    if (!canReadContracts()) {
      forbidden.value = true
      allItems.value = []
      return
    }
    if (!currentCompanyId.value) {
      allItems.value = []
      return
    }
    await loadCarriers()
    allItems.value = await listTransportContracts()
  } catch (error) {
    allItems.value = []
    if (error instanceof ApiError && error.status === 403) {
      forbidden.value = true
    } else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) {
        pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

function openCreate() {
  createForm.buyer_company_id = currentCompanyId.value ?? ''
  createForm.carrier_company_id = ''
  createForm.contract_number = ''
  createForm.name = ''
  createForm.external_reference = ''
  createForm.description = ''
  createForm.valid_from = new Date().toISOString().slice(0, 10)
  createForm.valid_to = ''
  createForm.currency_code = 'RUB'
  showCreateModal.value = true
}

async function submitCreate() {
  creating.value = true
  try {
    const payload = buildCreateContractPayload(createForm)
    await createTransportContract(payload)
    pushToast('success', t('contracts.createSuccess'))
    showCreateModal.value = false
    pagination.offset = 0
    await loadContracts()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    creating.value = false
  }
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
}

function goNext() {
  pagination.offset += pagination.limit
}

watch([() => filters.q, () => filters.status, () => filters.carrier_company_id], () => {
  pagination.offset = 0
})

watch(currentCompanyId, () => {
  pagination.offset = 0
  loadContracts()
}, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('contracts.title')">
      <template v-if="canCreateContracts() && !isCarrierContractReader()" #actions>
        <Button @click="openCreate">{{ t('contracts.create') }}</Button>
      </template>
    </PageHeader>

    <Card>
      <div class="filters-row">
        <Input v-model="filters.q" :label="t('contracts.search')" />
        <Select v-model="filters.status" :label="t('contracts.statusFilter')">
          <option value="">{{ t('contracts.allStatuses') }}</option>
          <option v-for="status in CONTRACT_STATUSES" :key="status" :value="status">
            {{ statusLabel(status) }}
          </option>
        </Select>
        <Select v-model="filters.carrier_company_id" :label="t('contracts.carrierFilter')">
          <option value="">{{ t('contracts.allCarriers') }}</option>
          <option v-for="carrier in carriers" :key="carrier.id" :value="carrier.id">
            {{ carrier.legal_name }}
          </option>
        </Select>
      </div>
    </Card>

    <EmptyState
      v-if="missingCompany"
      :title="t('contracts.missingCompany')"
    />
    <EmptyState
      v-else-if="forbidden"
      :title="t('contracts.forbidden')"
    />
    <EmptyState
      v-else-if="apiUnavailable"
      :title="t('contracts.backendUnavailable')"
    />
    <EmptyState
      v-else-if="!loading && items.length === 0"
      :title="t('contracts.empty')"
    />
    <Card v-else>
      <div class="table-scroll">
        <Table
          :columns="[
            t('contracts.contractNumber'),
            t('contracts.name'),
            t('contracts.carrier'),
            t('contracts.status'),
            t('contracts.validFrom'),
            t('contracts.validTo'),
            t('contracts.currency'),
            t('contracts.externalReference'),
            t('contracts.updated'),
            '',
          ]"
          :loading="loading"
        >
          <tr v-for="contract in items" :key="contract.id">
            <td>
              <NuxtLink :to="`/contracts/${contract.id}`">{{ contract.contract_number }}</NuxtLink>
            </td>
            <td>{{ contract.name }}</td>
            <td>{{ companyNameById.get(contract.carrier_company_id) ?? contract.carrier_company_id }}</td>
            <td><Badge :status="contract.status">{{ statusLabel(contract.status) }}</Badge></td>
            <td>{{ contract.valid_from }}</td>
            <td>{{ contract.valid_to ?? '—' }}</td>
            <td>{{ contract.currency_code }}</td>
            <td>{{ contract.external_reference ?? '—' }}</td>
            <td>{{ contract.updated_at }}</td>
            <td>
              <NuxtLink :to="`/contracts/${contract.id}`">{{ t('common.details') }}</NuxtLink>
            </td>
          </tr>
        </Table>
      </div>
      <div class="pagination">
        <Button variant="secondary" :disabled="!canGoPrev" @click="goPrev">{{ t('common.back') }}</Button>
        <Button variant="secondary" :disabled="!canGoNext" @click="goNext">{{ t('common.next') }}</Button>
      </div>
    </Card>

    <Modal :open="showCreateModal" :title="t('contracts.create')" @close="showCreateModal = false">
      <div class="form-grid">
        <Select v-model="createForm.buyer_company_id" :label="t('contracts.buyer')" required>
          <option v-if="currentCompanyId" :value="currentCompanyId">{{ currentCompanyId }}</option>
        </Select>
        <Select v-model="createForm.carrier_company_id" :label="t('contracts.carrier')" required>
          <option value="" disabled>{{ t('contracts.allCarriers') }}</option>
          <option v-for="carrier in carriers" :key="carrier.id" :value="carrier.id">
            {{ carrier.legal_name }}
          </option>
        </Select>
        <Input v-model="createForm.contract_number" :label="t('contracts.contractNumber')" required />
        <Input v-model="createForm.name" :label="t('contracts.name')" required />
        <Input v-model="createForm.external_reference" :label="t('contracts.externalReference')" />
        <Input v-model="createForm.valid_from" type="date" :label="t('contracts.validFrom')" required />
        <Input v-model="createForm.valid_to" type="date" :label="t('contracts.validTo')" />
        <Input v-model="createForm.currency_code" :label="t('contracts.currency')" required />
        <Input v-model="createForm.description" :label="t('contracts.description')" />
      </div>
      <template #footer>
        <Button variant="secondary" @click="showCreateModal = false">{{ t('common.cancel') }}</Button>
        <Button :loading="creating" @click="submitCreate">{{ t('contracts.save') }}</Button>
      </template>
    </Modal>
  </div>
</template>
