<script setup lang="ts">
import type { Company } from '~/types/company'
import type { RfxEvent } from '~/types/rfx'
import type { EvaluationResponseItem, AuditEventItem } from '~/types/evaluation'
import { formatMoney, sortEvaluationItems } from '~/types/evaluation'
import { formatRfxDate } from '~/types/rfx'
import { shouldShowNotFound, isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getRfxEvent } = useRfxApi()
const { listCompanies } = useCompanies()
const { setCompany } = useTenantContext()
const { canManageTenders } = usePermissions()
const {
  listEvaluationResponses,
  recalculateEvaluation,
  addToShortlist,
  removeFromShortlist,
  awardResponse,
  listAuditEvents,
} = useRfxEvaluationApi()
const { pushToast } = useToast()
const { t } = useI18n()

const eventId = computed(() => String(route.params.id))
const event = ref<RfxEvent | null>(null)
const items = ref<EvaluationResponseItem[]>([])
const auditEvents = ref<AuditEventItem[]>([])
const companies = ref<Company[]>([])
const loading = ref(true)
const acting = ref(false)
const notFound = ref(false)
const apiUnavailable = ref(false)
const showAwardModal = ref(false)
const awardTarget = ref<EvaluationResponseItem | null>(null)

const sortedItems = computed(() => sortEvaluationItems(items.value))

function companyName(id: string) {
  return companies.value.find((c) => c.id === id)?.legal_name || id
}

function comparableLabel(item: EvaluationResponseItem) {
  if (item.comparable === false) return t('tenders.evaluation.notComparable')
  return formatMoney(item.total_amount, item.currency_code)
}

async function loadCompanies() {
  try {
    companies.value = (await listCompanies({ limit: 200, status: 'ACTIVE' })).items
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
    items.value = await listEvaluationResponses(eventId.value)
    auditEvents.value = await listAuditEvents(eventId.value)
  } catch (error) {
    event.value = null
    items.value = []
    auditEvents.value = []
    if (shouldShowNotFound(error)) notFound.value = true
    else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) pushToast('error', error instanceof Error ? error.message : t('tenders.evaluation.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function handleRecalculate() {
  acting.value = true
  try {
    items.value = await recalculateEvaluation(eventId.value)
    pushToast('success', t('tenders.evaluation.recalculated'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

async function toggleShortlist(item: EvaluationResponseItem) {
  acting.value = true
  try {
    if (item.shortlisted) await removeFromShortlist(item.id)
    else await addToShortlist(item.id)
    items.value = await listEvaluationResponses(eventId.value)
    auditEvents.value = await listAuditEvents(eventId.value)
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

function openAwardConfirm(item: EvaluationResponseItem) {
  awardTarget.value = item
  showAwardModal.value = true
}

async function confirmAward() {
  if (!awardTarget.value) return
  acting.value = true
  try {
    await awardResponse(eventId.value, awardTarget.value.id)
    pushToast('success', t('tenders.evaluation.awardSuccess'))
    showAwardModal.value = false
    awardTarget.value = null
    await loadWorkspace()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('tenders.evaluation.awardFailed'))
  } finally {
    acting.value = false
  }
}

watch(eventId, loadWorkspace, { immediate: true })
onMounted(loadCompanies)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/tenders">{{ t('tenders.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink :to="`/tenders/${eventId}`">{{ event?.rfx_number || eventId }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ t('tenders.evaluation.title') }}</span>
    </nav>

    <PageHeader :title="t('tenders.evaluation.title')" :subtitle="event?.title">
      <template #actions>
        <Button variant="secondary" @click="$router.push(`/tenders/${eventId}`)">{{ t('common.back') }}</Button>
        <Button v-if="canManageTenders()" :loading="acting" @click="handleRecalculate">
          {{ t('tenders.evaluation.recalculate') }}
        </Button>
      </template>
    </PageHeader>

    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('tenders.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('tenders.loadFailed')" />

    <template v-else>
      <Card>
        <template #header>
          <h3>{{ t('tenders.evaluation.comparison') }}</h3>
        </template>
        <EmptyState v-if="sortedItems.length === 0" :title="t('tenders.evaluation.empty')" />
        <div v-else class="table-scroll">
          <Table
            :columns="[
              t('tenders.evaluation.carrier'),
              t('common.status'),
              t('tenders.evaluation.submitted'),
              t('tenders.evaluation.price'),
              t('tenders.evaluation.score'),
              t('tenders.evaluation.rank'),
              t('tenders.evaluation.shortlist'),
              t('tenders.evaluation.award'),
              '',
            ]"
          >
            <tr v-for="item in sortedItems" :key="item.id">
              <td>{{ companyName(item.participant_company_id) }}</td>
              <td><Badge :status="item.status" /></td>
              <td>{{ formatRfxDate(item.submitted_at) }}</td>
              <td>{{ comparableLabel(item) }}</td>
              <td>{{ item.total_score ?? '—' }}</td>
              <td>{{ item.rank || '—' }}</td>
              <td>
                <Button
                  v-if="canManageTenders()"
                  size="sm"
                  variant="secondary"
                  :disabled="acting"
                  @click="toggleShortlist(item)"
                >
                  {{ item.shortlisted ? t('tenders.evaluation.removeShortlist') : t('tenders.evaluation.addShortlist') }}
                </Button>
              </td>
              <td>
                <Badge v-if="item.awarded" status="AWARDED" />
                <span v-else-if="!item.offer_complete" class="muted">{{ t('tenders.evaluation.incomplete') }}</span>
              </td>
              <td>
                <Button
                  v-if="canManageTenders() && !item.awarded && item.offer_complete"
                  size="sm"
                  variant="danger"
                  :disabled="acting"
                  @click="openAwardConfirm(item)"
                >
                  {{ t('tenders.evaluation.award') }}
                </Button>
              </td>
            </tr>
          </Table>
        </div>
      </Card>

      <Card>
        <template #header>
          <h3>{{ t('tenders.evaluation.decisionHistory') }}</h3>
        </template>
        <EmptyState v-if="auditEvents.length === 0" :title="t('tenders.evaluation.noHistory')" />
        <ul v-else class="audit-list">
          <li v-for="entry in auditEvents" :key="entry.id">
            <strong>{{ entry.action }}</strong>
            <span class="muted"> — {{ formatRfxDate(entry.created_at) }}</span>
          </li>
        </ul>
      </Card>
    </template>

    <Modal v-model="showAwardModal" :title="t('tenders.evaluation.awardConfirmTitle')">
      <p v-if="awardTarget">
        {{ t('tenders.evaluation.awardConfirmBody', {
          carrier: companyName(awardTarget.participant_company_id),
          amount: comparableLabel(awardTarget),
        }) }}
      </p>
      <template #footer>
        <Button variant="secondary" @click="showAwardModal = false">{{ t('common.cancel') }}</Button>
        <Button variant="danger" :loading="acting" @click="confirmAward">{{ t('tenders.evaluation.confirmAward') }}</Button>
      </template>
    </Modal>
  </div>
</template>

<style scoped>
.table-scroll {
  overflow-x: auto;
}

.audit-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.audit-list li + li {
  margin-top: 0.5rem;
}

.muted {
  color: var(--color-text-muted);
}
</style>
