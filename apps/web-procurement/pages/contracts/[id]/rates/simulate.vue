<script setup lang="ts">
import type { LocationSummary, RateResolutionResult, TransportContract } from '~/types/contractRate'
import { buildSimulationRequest } from '~/utils/contractRate'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: ['auth', 'contract-rate-workspace'], layout: 'default' })

const route = useRoute()
const contractId = computed(() => String(route.params.id))
const { getTransportContract, resolveRate } = useContractRatesApi()
const { listLocations } = useLocationsApi()
const { pushToast } = useToast()
const { t } = useI18n()
const { canSimulateRates } = usePermissions()

const loading = ref(true)
const resolving = ref(false)
const apiUnavailable = ref(false)
const contract = ref<TransportContract | null>(null)
const locations = ref<LocationSummary[]>([])
const result = ref<RateResolutionResult | null>(null)

const form = reactive({
  origin_location_id: '',
  destination_location_id: '',
  equipment_type: '',
  pricing_date: new Date().toISOString().slice(0, 10),
})

const locationLabel = computed(() => {
  const map = new Map<string, string>()
  for (const loc of locations.value) {
    map.set(loc.id, [loc.name, loc.city, loc.region].filter(Boolean).join(', '))
  }
  return map
})

async function loadPage() {
  loading.value = true
  apiUnavailable.value = false
  try {
    contract.value = await getTransportContract(contractId.value)
    const locPage = await listLocations({ limit: 500 })
    locations.value = locPage.items
  } catch (error) {
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function runSimulation() {
  if (!contract.value || !canSimulateRates()) return
  resolving.value = true
  result.value = null
  try {
    const payload = buildSimulationRequest({
      buyer_company_id: contract.value.buyer_company_id,
      carrier_company_id: contract.value.carrier_company_id,
      origin_location_id: form.origin_location_id,
      destination_location_id: form.destination_location_id,
      equipment_type: form.equipment_type,
      pricing_date: form.pricing_date,
      currency_code: contract.value.currency_code,
    })
    result.value = await resolveRate(payload)
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('contracts.loadFailed'))
  } finally {
    resolving.value = false
  }
}

onMounted(loadPage)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('rateSimulation.title')" :subtitle="t('rateSimulation.subtitle')" />

    <EmptyState v-if="loading" :title="t('common.loading')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('contracts.backendUnavailable')" />
    <template v-else-if="contract">
      <Card>
        <dl class="detail-grid">
          <dt>{{ t('contracts.buyer') }}</dt>
          <dd>{{ contract.buyer_company_id }}</dd>
          <dt>{{ t('contracts.carrier') }}</dt>
          <dd>{{ contract.carrier_company_id }}</dd>
          <dt>{{ t('rates.mode') }}</dt>
          <dd>ROAD</dd>
        </dl>
      </Card>

      <Card>
        <div class="form-grid">
          <Select v-model="form.origin_location_id" :label="t('rates.origin')" required>
            <option value="" disabled>{{ t('rates.origin') }}</option>
            <option v-for="loc in locations" :key="loc.id" :value="loc.id">
              {{ locationLabel.get(loc.id) }}
            </option>
          </Select>
          <Select v-model="form.destination_location_id" :label="t('rates.destination')" required>
            <option value="" disabled>{{ t('rates.destination') }}</option>
            <option v-for="loc in locations" :key="loc.id" :value="loc.id">
              {{ locationLabel.get(loc.id) }}
            </option>
          </Select>
          <Input v-model="form.equipment_type" :label="t('rates.equipment')" required />
          <Input v-model="form.pricing_date" type="date" :label="t('rateSimulation.pricingDate')" required />
        </div>
        <div class="actions-row">
          <Button :loading="resolving" @click="runSimulation">{{ t('rateSimulation.run') }}</Button>
          <NuxtLink :to="`/contracts/${contract.id}/rates`">{{ t('common.back') }}</NuxtLink>
        </div>
      </Card>

      <Card v-if="result?.status === 'MATCHED'">
        <h2>{{ t('rateSimulation.matched') }}</h2>
        <p>
          {{
            t('rateSimulation.contractContext', {
              number: result.contract_number ?? '',
              card: result.rate_card_name ?? '',
              version: result.version_number ?? '',
            })
          }}
        </p>
        <p>{{ t('rateSimulation.total') }}: {{ result.total_amount }} {{ result.currency_code }}</p>
        <ul v-if="result.components?.length">
          <li v-for="(component, index) in result.components" :key="index">
            {{ component.component_type }} — {{ component.amount ?? component.percent_value }}
          </li>
        </ul>
      </Card>

      <EmptyState v-else-if="result?.status === 'NO_MATCH'" :title="t('rateSimulation.noMatch')" />
      <EmptyState
        v-else-if="result?.status === 'AMBIGUOUS'"
        :title="t('rateSimulation.ambiguous')"
      />
      <EmptyState
        v-else-if="result?.reason_code === 'RATE_NOT_FOUND'"
        :title="t('rateSimulation.rateNotFound')"
      />
    </template>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: 12rem 1fr;
  gap: 0.5rem 1rem;
}

.actions-row {
  display: flex;
  gap: 1rem;
  align-items: center;
  margin-top: 1rem;
}
</style>
