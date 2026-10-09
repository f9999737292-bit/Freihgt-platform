<script setup lang="ts">
import { Badge, Button, Card, CustomerPageHeader, Input, Toast } from '@freight-platform/ui'
import PortalState from '../../components/PortalState.vue'
import { isMissingRecord, viewKindFromError, type PortalViewKind } from '../../domain/viewState'
import type { CarrierResponse, OwnAward, TenderSummary } from '../../composables/useCarrierApi'

const route = useRoute()
const { t } = useI18n()
const api = useCarrierApi()
const office = useCarrierOffice()
const tender = ref<TenderSummary | null>(null)
const response = ref<CarrierResponse | null>(null)
const award = ref<OwnAward | null>(null)
const kind = ref<PortalViewKind>('loading')
const detail = ref('')
const amount = ref('')
const currency = ref('RUB')
const comment = ref('')
const lotId = ref('')

const eventId = computed(() => String(route.params.id))

async function load() {
  kind.value = 'loading'
  tender.value = null
  response.value = null
  award.value = null
  detail.value = ''
  try {
    tender.value = await api.getTender(eventId.value)
    try {
      response.value = await api.getOwnResponse(eventId.value)
      const line = response.value.offer_lines?.[0]
      if (line) {
        amount.value = String(line.amount ?? '')
        currency.value = line.currency_code || 'RUB'
        comment.value = line.comment ?? ''
        lotId.value = line.rfx_lot_id ?? ''
      }
    } catch (error) {
      if (!isMissingRecord(error)) throw error
    }
    try {
      award.value = await api.getOwnAward(eventId.value)
    } catch (error) {
      if (!isMissingRecord(error)) throw error
    }
    kind.value = 'ready'
  } catch (error) {
    tender.value = null
    kind.value = viewKindFromError(error)
    detail.value = error instanceof Error ? error.message : ''
  }
}

onMounted(load)

async function createResponse() {
  response.value = await api.createResponse(eventId.value)
  office.notice.value = t('carrier.saved')
}

async function saveOffer() {
  if (!response.value) return
  response.value = await api.saveOffer(response.value.id, [{
    rfx_lot_id: lotId.value || null,
    amount: Number(amount.value),
    currency_code: currency.value,
    comment: comment.value || null,
  }])
  office.notice.value = t('carrier.saved')
}

async function submitResponse() {
  if (!response.value) return
  response.value = await api.submitResponse(response.value.id)
  office.notice.value = t('carrier.submitted')
  await load()
}

function titleFor(state: PortalViewKind) {
  if (state === 'forbidden') return t('carrier.forbidden')
  if (state === 'not_found') return t('carrier.notFound')
  if (state === 'loading') return t('carrier.loading')
  return t('carrier.unavailable')
}
</script>

<template>
  <section data-testid="tender-detail">
    <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
    <template v-else-if="tender">
      <CustomerPageHeader :title="tender.title || t('carrier.tenderDetail')" :subtitle="tender.rfx_number" />
      <Card>
        <p><Badge :label="tender.status || ''" /></p>
        <p data-testid="tender-deadline">{{ t('carrier.deadline') }}: {{ tender.response_deadline }}</p>
        <p>{{ tender.participant_status }}</p>
        <p>{{ tender.own_response_status }}</p>
      </Card>
      <Card data-testid="own-response">
        <h2>{{ t('carrier.response') }}</h2>
        <p v-if="!response">{{ t('carrier.emptyResponse') }}</p>
        <template v-else>
          <Badge :label="response.status || ''" />
          <form class="offer" @submit.prevent="saveOffer">
            <Input v-model="amount" name="amount" data-testid="offer-amount" :label="t('carrier.amount')" />
            <Input v-model="currency" name="currency" data-testid="offer-currency" :label="t('carrier.currency')" />
            <Input v-model="comment" name="comment" :label="t('carrier.comment')" />
            <Button type="submit" data-testid="save-offer">{{ t('carrier.saveOffer') }}</Button>
          </form>
          <Button type="button" data-testid="submit-response" @click="submitResponse">{{ t('carrier.submitResponse') }}</Button>
        </template>
        <Button v-if="!response" type="button" data-testid="create-response" @click="createResponse">
          {{ t('carrier.createResponse') }}
        </Button>
      </Card>
      <Card data-testid="own-award">
        <h2>{{ t('carrier.award') }}</h2>
        <p v-if="!award">{{ t('carrier.emptyAward') }}</p>
        <p v-else data-testid="award-amount">{{ award.total_amount }} {{ award.currency_code }}</p>
      </Card>
    </template>
    <Toast v-if="office.notice.value" :message="office.notice.value" />
  </section>
</template>

<style scoped>
.offer {
  display: grid;
  gap: 0.75rem;
  max-width: 24rem;
  margin: 0.75rem 0;
}
</style>
