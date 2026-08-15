<script setup lang="ts">
import { formatApiErrorForUser } from '~/composables/useApi'
import { useTenderEvaluationApi } from '~/composables/useTenderEvaluationApi'
import { useTenderWorkspaceState } from '~/composables/useTenderWorkspaceState'
import type { CarrierScoreResult, TenderBidRevision } from '~/types/tender'

const props = defineProps<{
  rfxEventId: string
  companyName?: (id: string) => string
}>()

const { listEventBids, getMyResponse, listResponseRevisions, submitResponseRevision } =
  useTenderEvaluationApi()
const { pushToast } = useToast()
const { t } = useI18n()
const { hasProductRole, isPlatformAdmin } = usePermissions()
const { currentCompanyId } = useTenantContext()
const workspace = useTenderWorkspaceState()

const loading = ref(false)
const bids = ref<TenderBidRevision[]>([])
const myResponse = ref<TenderBidRevision | null>(null)
const revisionHistory = ref<TenderBidRevision[]>([])
const expandedCarrier = ref<string | null>(null)

const isCarrierView = computed(
  () => hasProductRole('carrier') && !isPlatformAdmin(),
)
const carrierCompanyId = computed(() => currentCompanyId.value || null)

const scoreByCarrier = computed(() => {
  const map = new Map<string, CarrierScoreResult>()
  for (const score of workspace.value.evaluation?.scores ?? []) {
    map.set(score.carrier_company_id, score)
  }
  return map
})

const qualificationByCarrier = computed(() => {
  const map = new Map<string, { result: string; reasons: string[] }>()
  for (const item of workspace.value.evaluation?.qualification ?? []) {
    map.set(item.carrier_company_id, { result: item.result, reasons: item.reasons })
  }
  return map
})

const revisionForm = reactive({
  price_amount: '0',
  currency_code: 'RUB',
  capacity_units: '0',
  transit_hours: '0',
  sla_score_input: '80',
  carrier_kpi_score_input: '80',
  reliability_score_input: '80',
  comment: '',
})

function carrierLabel(id: string) {
  return props.companyName?.(id) || id
}

function bidPrice(bid: TenderBidRevision) {
  return bid.price_amount ?? bid.total_amount ?? 0
}

async function loadBids() {
  loading.value = true
  try {
    if (isCarrierView.value) {
      if (!carrierCompanyId.value) {
        bids.value = []
        myResponse.value = null
        revisionHistory.value = []
        return
      }
      myResponse.value = await getMyResponse(props.rfxEventId, carrierCompanyId.value)
      bids.value = myResponse.value ? [myResponse.value] : []
      if (myResponse.value?.rfx_response_id) {
        const history = await listResponseRevisions(
          props.rfxEventId,
          myResponse.value.rfx_response_id,
          carrierCompanyId.value,
        )
        revisionHistory.value = history.items
      }
      return
    }

    const response = await listEventBids(props.rfxEventId)
    bids.value = response.items
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
    bids.value = []
  } finally {
    loading.value = false
  }
}

async function submitRevision() {
  if (!myResponse.value?.rfx_response_id || !carrierCompanyId.value) return
  loading.value = true
  try {
    await submitResponseRevision(
      props.rfxEventId,
      myResponse.value.rfx_response_id,
      {
        participant_company_id: carrierCompanyId.value,
        price_amount: Number(revisionForm.price_amount),
        currency_code: revisionForm.currency_code,
        capacity_units: Number(revisionForm.capacity_units),
        transit_hours: Number(revisionForm.transit_hours),
        sla_score_input: Number(revisionForm.sla_score_input),
        carrier_kpi_score_input: Number(revisionForm.carrier_kpi_score_input),
        reliability_score_input: Number(revisionForm.reliability_score_input),
        comment: revisionForm.comment || undefined,
      },
      carrierCompanyId.value,
    )
    pushToast('success', t('tender.revisionSubmitted'))
    await loadBids()
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}

watch(() => props.rfxEventId, loadBids, { immediate: true })
watch(carrierCompanyId, loadBids)
</script>

<template>
  <section class="tender-bids">
    <header class="tender-bids__header">
      <h3>{{ $t('tender.tabs.bids') }}</h3>
      <UiButton size="sm" variant="secondary" :loading="loading" @click="loadBids">
        {{ $t('common.refresh') }}
      </UiButton>
    </header>

    <p v-if="isCarrierView && !carrierCompanyId" class="tender-bids__hint">
      {{ $t('tender.carrierCompanyRequired') }}
    </p>

    <table v-else-if="bids.length" class="tender-bids__table">
      <thead>
        <tr>
          <th>{{ $t('tender.carrier') }}</th>
          <th>{{ $t('tender.revision') }}</th>
          <th>{{ $t('tender.price') }}</th>
          <th>{{ $t('tender.currency') }}</th>
          <th>{{ $t('tender.capacity') }}</th>
          <th>{{ $t('tender.transitTime') }}</th>
          <th v-if="!isCarrierView">{{ $t('tender.qualification') }}</th>
          <th v-if="!isCarrierView">{{ $t('tender.sla') }}</th>
          <th v-if="!isCarrierView">{{ $t('tender.kpi') }}</th>
          <th v-if="!isCarrierView">{{ $t('tender.totalScore') }}</th>
          <th v-if="!isCarrierView" />
        </tr>
      </thead>
      <tbody>
        <template v-for="bid in bids" :key="bid.id">
          <tr>
            <td>{{ carrierLabel(bid.participant_company_id || bid.carrier_company_id || '') }}</td>
            <td>{{ bid.revision_number }}</td>
            <td>{{ bidPrice(bid).toFixed(2) }}</td>
            <td>{{ bid.currency_code }}</td>
            <td>{{ bid.capacity_units }}</td>
            <td>{{ bid.transit_hours }}</td>
            <td v-if="!isCarrierView">
              {{
                qualificationByCarrier.get(
                  bid.participant_company_id || bid.carrier_company_id || '',
                )?.result || '—'
              }}
            </td>
            <td v-if="!isCarrierView">{{ bid.sla_score_input }}</td>
            <td v-if="!isCarrierView">{{ bid.carrier_kpi_score_input }}</td>
            <td v-if="!isCarrierView">
              {{
                scoreByCarrier
                  .get(bid.participant_company_id || bid.carrier_company_id || '')
                  ?.total_score.toFixed(2) || '—'
              }}
            </td>
            <td v-if="!isCarrierView">
              <UiButton
                size="sm"
                variant="secondary"
                @click="
                  expandedCarrier =
                    expandedCarrier === (bid.participant_company_id || bid.carrier_company_id)
                      ? null
                      : (bid.participant_company_id || bid.carrier_company_id || null)
                "
              >
                {{ $t('tender.scoreBreakdown') }}
              </UiButton>
            </td>
          </tr>
          <tr
            v-if="
              !isCarrierView
              && expandedCarrier === (bid.participant_company_id || bid.carrier_company_id)
            "
          >
            <td :colspan="11">
              <RfxTenderScoreBreakdown
                :score="
                  scoreByCarrier.get(
                    bid.participant_company_id || bid.carrier_company_id || '',
                  )
                "
                :qualification="
                  qualificationByCarrier.get(
                    bid.participant_company_id || bid.carrier_company_id || '',
                  )
                "
              />
            </td>
          </tr>
        </template>
      </tbody>
    </table>

    <UiEmptyState v-else-if="!loading" :title="$t('tender.noBids')" />

    <section v-if="isCarrierView && myResponse" class="tender-bids__revision">
      <h4>{{ $t('tender.submitRevision') }}</h4>
      <div class="tender-bids__form">
        <UiInput v-model="revisionForm.price_amount" type="number" :label="$t('tender.price')" />
        <UiInput v-model="revisionForm.currency_code" :label="$t('tender.currency')" />
        <UiInput v-model="revisionForm.capacity_units" type="number" :label="$t('tender.capacity')" />
        <UiInput v-model="revisionForm.transit_hours" type="number" :label="$t('tender.transitTime')" />
        <UiInput v-model="revisionForm.sla_score_input" type="number" :label="$t('tender.sla')" />
        <UiInput
          v-model="revisionForm.carrier_kpi_score_input"
          type="number"
          :label="$t('tender.kpi')"
        />
        <UiInput
          v-model="revisionForm.reliability_score_input"
          type="number"
          :label="$t('tender.reliability')"
        />
        <UiInput v-model="revisionForm.comment" :label="$t('tender.comment')" />
      </div>
      <UiButton :loading="loading" @click="submitRevision">{{ $t('tender.submitRevision') }}</UiButton>

      <table v-if="revisionHistory.length" class="tender-bids__table tender-bids__table--history">
        <thead>
          <tr>
            <th>{{ $t('tender.revision') }}</th>
            <th>{{ $t('tender.price') }}</th>
            <th>{{ $t('common.status') }}</th>
            <th>{{ $t('tender.submittedAt') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rev in revisionHistory" :key="rev.id">
            <td>{{ rev.revision_number }}</td>
            <td>{{ bidPrice(rev).toFixed(2) }}</td>
            <td>{{ rev.is_active ? $t('tender.activeRevision') : $t('tender.historicalRevision') }}</td>
            <td>{{ rev.submitted_at || rev.created_at }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </section>
</template>

<style scoped>
.tender-bids__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.tender-bids__hint {
  color: var(--color-text-muted);
  margin-top: 0.75rem;
}

.tender-bids__table {
  width: 100%;
  margin-top: 1rem;
  border-collapse: collapse;
}

.tender-bids__table th,
.tender-bids__table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.tender-bids__revision {
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid var(--color-border);
}

.tender-bids__form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  margin: 1rem 0;
}

.tender-bids__table--history {
  margin-top: 1rem;
}
</style>
