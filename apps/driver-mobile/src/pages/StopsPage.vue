<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { useSubmissionLock } from '@/composables/useSubmission'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import {
  beginCommand,
  buildDispositionBody,
  buildFailCommand,
  buildStopCommand,
  classifyStopFailure,
  clearStoredCommand,
  dispositionIsRejection,
  dispositionIssue,
  isFailReason,
  loadStoredCommand,
  safeValidationMessage,
  stopCommandId,
  stopCommandVisibility,
  type DispositionDraft,
  type StopOperationKind,
} from '@/stops/commands'
import type { DriverApiError, RequestResult } from '@/types/api'
import type { DriverCurrentNextStopsResponse, DriverStopActionFact, DriverStopTask } from '@/types/stops'
import { DISPOSITION_REASON_CODES, FAIL_REASON_CODES } from '@/types/stops'

const auth = useAuthStore()
const network = useNetworkStore()
const router = useRouter()
const { t } = useI18n()
const { submitting, runOnce } = useSubmissionLock()

const loaded = ref(false)
const stops = ref<DriverCurrentNextStopsResponse | null>(null)
const notice = ref('')
const uncertain = ref(false)
const pendingRetry = ref<{ kind: StopOperationKind; targetId: string } | null>(null)
const drafts = ref<Record<string, DispositionDraft>>({})
const failReason = ref<Record<string, string>>({})

function api() {
  return auth.createApi(() => network.online)
}

function nowIso(): string {
  return new Date().toISOString()
}

function showValue(value: string | null | undefined): string {
  return value ? value : '—'
}

function ensureDrafts(stop: DriverStopTask | null) {
  for (const action of stop?.actionSummary?.actions ?? []) {
    if (!drafts.value[action.actionId]) {
      drafts.value[action.actionId] = { accepted: 0, rejected: 0, reasonCode: '', reasonComment: '' }
    }
    if (failReason.value[action.actionId] === undefined) {
      failReason.value[action.actionId] = ''
    }
  }
}

async function loadStops(preserveNotice = false) {
  if (!preserveNotice) {
    notice.value = ''
    uncertain.value = false
    pendingRetry.value = null
  }
  const result = await api().getMyStops()
  loaded.value = true
  if (result.outcome === 'SUCCESS' && result.data) {
    ensureDrafts(result.data.current)
    stops.value = result.data
    return
  }
  if (result.error?.status === 401) {
    await auth.logout()
    await router.replace('/login')
    return
  }
  if (preserveNotice) return
  if (result.outcome === 'REQUEST_NOT_SENT') {
    notice.value = t('stops.offline')
    return
  }
  if (result.error && result.error.status < 500) {
    notice.value = safeValidationMessage(result.error, t('stops.loadError'))
    return
  }
  notice.value = t('stops.loadError')
}

async function handleRejection(error: DriverApiError | undefined) {
  const failure = classifyStopFailure(error)
  if (failure === 'unauthorized') {
    await auth.logout()
    await router.replace('/login')
    return
  }
  if (failure === 'forbidden') {
    notice.value = t('stops.forbidden')
    return
  }
  if (failure === 'missing') {
    notice.value = t('stops.notFound')
    await loadStops(true)
    return
  }
  if (failure === 'action_pending') {
    notice.value = t('stops.actionPending')
    await loadStops(true)
    return
  }
  if (failure === 'conflict') {
    notice.value = t('stops.routeChanged')
    await loadStops(true)
    return
  }
  if (failure === 'validation') {
    notice.value = error ? safeValidationMessage(error, t('stops.validation')) : t('stops.validation')
    return
  }
  notice.value = t('stops.server')
}

async function runMutation<T>(
  kind: StopOperationKind,
  targetId: string,
  build: () => T,
  send: (key: string, body: T) => Promise<RequestResult<unknown>>,
  onSuccess: (body: T) => void,
) {
  if (submitting.value) return
  const command = beginCommand(kind, targetId, build)
  const result = await runOnce(() => send(command.key, command.body))
  if (result.outcome === 'REQUEST_NOT_SENT') {
    if (network.online) return
    clearStoredCommand(kind, targetId)
    uncertain.value = false
    pendingRetry.value = null
    notice.value = t('stops.offline')
    return
  }
  if (result.outcome === 'REQUEST_SENT_RESPONSE_UNKNOWN') {
    uncertain.value = true
    pendingRetry.value = { kind, targetId }
    notice.value = t('stops.unknown')
    return
  }
  uncertain.value = false
  pendingRetry.value = null
  clearStoredCommand(kind, targetId)
  if (result.outcome === 'SUCCESS') {
    onSuccess(command.body)
    await loadStops(true)
    return
  }
  await handleRejection(result.error)
}

function currentStop(): DriverStopTask | null {
  return stops.value?.current ?? null
}

async function arrive() {
  const current = currentStop()
  if (!current) return
  const client = api()
  const stopId = stopCommandId(current)
  await runMutation(
    'arrive',
    stopId,
    () => buildStopCommand(current.version, nowIso()),
    (key, body) => client.arriveStop(stopId, body, key),
    () => {
      notice.value = ''
    },
  )
}

async function startService() {
  const current = currentStop()
  if (!current) return
  const client = api()
  const stopId = stopCommandId(current)
  await runMutation(
    'start-service',
    stopId,
    () => buildStopCommand(current.version, nowIso()),
    (key, body) => client.startStopService(stopId, body, key),
    () => {
      notice.value = ''
    },
  )
}

async function completeStop() {
  const current = currentStop()
  if (!current) return
  const client = api()
  const stopId = stopCommandId(current)
  await runMutation(
    'complete',
    stopId,
    () => buildStopCommand(current.version, nowIso()),
    (key, body) => client.completeStop(stopId, body, key),
    () => {
      notice.value = ''
    },
  )
}

async function confirmAction(action: DriverStopActionFact) {
  const current = currentStop()
  if (!current) return
  const client = api()
  const stopId = stopCommandId(current)
  await runMutation(
    'confirm',
    action.actionId,
    () => buildStopCommand(current.version, nowIso()),
    (key, body) => client.confirmStopAction(stopId, action.actionId, body, key),
    () => {
      notice.value = ''
    },
  )
}

async function failAction(action: DriverStopActionFact) {
  const current = currentStop()
  if (!current) return
  const selected = failReason.value[action.actionId] ?? ''
  if (!loadStoredCommand('fail', action.actionId) && !isFailReason(selected)) {
    notice.value = t('stops.reasonRequired')
    uncertain.value = false
    return
  }
  const client = api()
  const stopId = stopCommandId(current)
  await runMutation(
    'fail',
    action.actionId,
    () => {
      if (!isFailReason(selected)) {
        throw new Error('fail reason is required')
      }
      return buildFailCommand(current.version, nowIso(), selected)
    },
    (key, body) => client.failStopAction(stopId, action.actionId, body, key),
    () => {
      notice.value = ''
    },
  )
}

async function submitDisposition(action: DriverStopActionFact) {
  const current = currentStop()
  if (!current) return
  const draft = drafts.value[action.actionId]
  if (!draft) return
  if (!loadStoredCommand('disposition', action.actionId)) {
    const issue = dispositionIssue(action.shipmentId, action.cargoId, draft)
    if (issue) {
      notice.value = t(`stops.issues.${issue}`)
      uncertain.value = false
      return
    }
  }
  const client = api()
  const stopId = stopCommandId(current)
  await runMutation(
    'disposition',
    action.actionId,
    () => {
      const body = buildDispositionBody(action.shipmentId, action.cargoId, draft, nowIso())
      if (!body) {
        throw new Error('delivery disposition is invalid')
      }
      return body
    },
    (key, body) => client.reportDeliveryDisposition(stopId, action.actionId, body, key),
    (body) => {
      notice.value = dispositionIsRejection(body.rejectedQuantity)
        ? t('stops.deviationRegistered')
        : t('stops.registered')
    },
  )
}

async function retryPending() {
  const pending = pendingRetry.value
  const current = currentStop()
  if (!pending || !current) return
  if (pending.kind === 'arrive') {
    await arrive()
    return
  }
  if (pending.kind === 'start-service') {
    await startService()
    return
  }
  if (pending.kind === 'complete') {
    await completeStop()
    return
  }
  const action = current.actionSummary.actions.find((item) => item.actionId === pending.targetId)
  if (!action) {
    notice.value = t('stops.notFound')
    await loadStops(true)
    return
  }
  if (pending.kind === 'confirm') {
    await confirmAction(action)
    return
  }
  if (pending.kind === 'fail') {
    await failAction(action)
    return
  }
  await submitDisposition(action)
}

onMounted(loadStops)
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-buttons slot="start">
          <ion-button data-testid="back-shipments" @click="router.push('/shipments')">{{ t('common.back') }}</ion-button>
        </ion-buttons>
        <ion-title>{{ t('stops.title') }}</ion-title>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding">
      <p v-if="notice" data-testid="notice">{{ notice }}</p>
      <button v-if="uncertain" type="button" data-testid="retry" :disabled="submitting" @click="retryPending">
        {{ t('common.retry') }}
      </button>

      <div v-if="!loaded" class="center">
        <ion-spinner name="crescent" />
        <p>{{ t('common.loading') }}</p>
      </div>

      <template v-else>
        <section v-if="stops?.current" data-testid="current-stop">
          <h2>{{ t('stops.current') }}</h2>
          <p>{{ t('stops.ordinal') }}: {{ stops.current.ordinal }}</p>
          <p>{{ t('stops.plannedArrival') }}: {{ showValue(stops.current.plannedArrival) }}</p>
          <p data-testid="current-status">{{ t('stops.status') }}: {{ stops.current.status }}</p>
          <p>{{ t('stops.locationId') }}: {{ stops.current.locationId }}</p>
          <p>{{ t('stops.shipmentId') }}: {{ showValue(stops.current.shipmentId) }}</p>

          <button
            v-if="stopCommandVisibility(stops.current.status).arrive"
            type="button"
            data-testid="arrive"
            :disabled="submitting"
            @click="arrive"
          >
            {{ t('stops.arrive') }}
          </button>
          <button
            v-if="stopCommandVisibility(stops.current.status).startService"
            type="button"
            data-testid="start-service"
            :disabled="submitting"
            @click="startService"
          >
            {{ t('stops.startService') }}
          </button>
          <button
            v-if="stopCommandVisibility(stops.current.status).complete"
            type="button"
            data-testid="complete"
            :disabled="submitting"
            @click="completeStop"
          >
            {{ t('stops.complete') }}
          </button>

          <h3>{{ t('stops.actions') }}</h3>
          <article
            v-for="action in stops.current.actionSummary.actions"
            :key="action.actionId"
            :data-testid="`action-${action.actionId}`"
          >
            <p>{{ action.actionType }} · {{ action.ordinal }}</p>
            <p>{{ action.cargoId }}</p>
            <template v-if="stopCommandVisibility(stops.current.status).actions">
              <button
                type="button"
                :data-testid="`confirm-${action.actionId}`"
                :disabled="submitting"
                @click="confirmAction(action)"
              >
                {{ t('stops.confirm') }}
              </button>
              <label>
                {{ t('stops.reason') }}
                <select :data-testid="`fail-reason-${action.actionId}`" v-model="failReason[action.actionId]">
                  <option value="">{{ t('stops.reason') }}</option>
                  <option v-for="code in FAIL_REASON_CODES" :key="code" :value="code">
                    {{ t(`problem.categories.${code}`) }}
                  </option>
                </select>
              </label>
              <button
                type="button"
                :data-testid="`fail-${action.actionId}`"
                :disabled="submitting"
                @click="failAction(action)"
              >
                {{ t('stops.fail') }}
              </button>

              <form
                v-if="action.actionType === 'DELIVERY' && drafts[action.actionId]"
                :data-testid="`disposition-form-${action.actionId}`"
                @submit.prevent="submitDisposition(action)"
              >
                <h4>{{ t('stops.disposition') }}</h4>
                <p>{{ t('stops.shipmentId') }}: {{ showValue(action.shipmentId) }}</p>
                <p>{{ action.cargoId }}</p>
                <label>
                  {{ t('stops.accepted') }}
                  <input
                    :data-testid="`accepted-${action.actionId}`"
                    type="number"
                    min="0"
                    step="1"
                    v-model.number="drafts[action.actionId].accepted"
                  />
                </label>
                <label>
                  {{ t('stops.rejected') }}
                  <input
                    :data-testid="`rejected-${action.actionId}`"
                    type="number"
                    min="0"
                    step="1"
                    v-model.number="drafts[action.actionId].rejected"
                  />
                </label>
                <label>
                  {{ t('stops.uom') }}
                  <input :data-testid="`uom-${action.actionId}`" value="PALLET" disabled />
                </label>
                <label>
                  {{ t('stops.reason') }}
                  <select :data-testid="`disposition-reason-${action.actionId}`" v-model="drafts[action.actionId].reasonCode">
                    <option value="">{{ t('stops.reason') }}</option>
                    <option v-for="code in DISPOSITION_REASON_CODES" :key="code" :value="code">
                      {{ t(`stops.dispositionReasons.${code}`) }}
                    </option>
                  </select>
                </label>
                <label>
                  {{ t('stops.comment') }}
                  <textarea
                    :data-testid="`disposition-comment-${action.actionId}`"
                    v-model="drafts[action.actionId].reasonComment"
                  />
                </label>
                <button type="submit" :data-testid="`disposition-${action.actionId}`" :disabled="submitting">
                  {{ t('stops.disposition') }}
                </button>
              </form>
            </template>
          </article>
        </section>
        <p v-else data-testid="empty-current">{{ t('stops.emptyCurrent') }}</p>

        <section v-if="stops?.next" data-testid="next-stop">
          <h2>{{ t('stops.next') }}</h2>
          <p>{{ t('stops.summary') }}: {{ stops.next.ordinal }}</p>
          <p>{{ t('stops.plannedArrival') }}: {{ showValue(stops.next.plannedArrival) }}</p>
          <p data-testid="next-status">{{ t('stops.status') }}: {{ stops.next.status }}</p>
          <p>{{ t('stops.locationId') }}: {{ stops.next.locationId }}</p>
          <p>{{ t('stops.shipmentId') }}: {{ showValue(stops.next.shipmentId) }}</p>
        </section>
        <p v-else-if="loaded" data-testid="empty-next">{{ t('stops.emptyNext') }}</p>
      </template>
    </ion-content>
  </ion-page>
</template>

<style scoped>
.center {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-top: 24px;
}

button {
  display: block;
  width: 100%;
  margin: 8px 0;
  padding: 12px;
}

label {
  display: block;
  margin: 8px 0;
}

input,
select,
textarea {
  display: block;
  width: 100%;
  margin-top: 4px;
}
</style>
