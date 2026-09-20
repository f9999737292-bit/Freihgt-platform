<script setup lang="ts">
import { LATE_SUBMISSION_REASON_CODES, type LateSubmissionReasonCode, type LateSubmissionRequest } from '~/types/lateSubmission'
import { formatRfxDate, toRFC3339 } from '~/types/rfx'
import {
  canCreateLateSubmissionRequest,
  canShowCarrierLateRequestPanel,
  latestOwnLateRequest,
} from '~/utils/lateSubmissionAccess'
import { classifyLateSubmissionHttpError } from '~/utils/lateSubmissionErrors'
import { createLateSubmissionIdempotencyStore } from '~/utils/lateSubmissionIdempotency'

const props = defineProps<{
  eventId: string
  carrierCompanyId: string
  deadline?: string | null
}>()

const emit = defineEmits<{
  updated: [request: LateSubmissionRequest | null]
}>()

const { t } = useI18n()
const { pushToast } = useToast()
const authStore = useAuthStore()
const { enabled } = useRfxLateSubmissionFeature()
const api = useLateSubmissionApi()
const idempotency = createLateSubmissionIdempotencyStore()

const loading = ref(false)
const acting = ref(false)
const errorKey = ref<string | null>(null)
const items = ref<LateSubmissionRequest[]>([])
const reasonCode = ref<LateSubmissionReasonCode>('TECHNICAL_FAILURE')
const reasonText = ref('')
const requestedUntil = ref('')

const roles = computed(() => authStore.user?.roles ?? [])
const current = computed(() => latestOwnLateRequest(items.value))
const showPanel = computed(() => canShowCarrierLateRequestPanel({
  lateSubmissionEnabled: enabled.value,
  roles: roles.value,
  deadline: props.deadline,
}))
const canCreate = computed(() => canCreateLateSubmissionRequest({
  lateSubmissionEnabled: enabled.value,
  roles: roles.value,
  deadline: props.deadline,
  request: current.value,
}))

function defaultRequestedUntil() {
  const next = new Date(Date.now() + 24 * 3600 * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${next.getFullYear()}-${pad(next.getMonth() + 1)}-${pad(next.getDate())}T${pad(next.getHours())}:${pad(next.getMinutes())}`
}

async function loadMine() {
  if (!showPanel.value || !props.carrierCompanyId) {
    items.value = []
    emit('updated', null)
    return
  }
  loading.value = true
  errorKey.value = null
  try {
    const result = await api.listOwnRequests(props.eventId, props.carrierCompanyId)
    items.value = result.items ?? []
    emit('updated', current.value)
  } catch (error) {
    errorKey.value = classifyLateSubmissionHttpError(error)
    items.value = []
    emit('updated', null)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  if (!canCreate.value) return
  acting.value = true
  errorKey.value = null
  try {
    const created = await api.createRequest(
      props.eventId,
      {
        reason_code: reasonCode.value,
        reason_text: reasonText.value.trim(),
        requested_until: toRFC3339(requestedUntil.value),
      },
      idempotency.keyForCreate(props.eventId, props.carrierCompanyId),
      props.carrierCompanyId,
    )
    items.value = [created, ...items.value.filter((item) => item.id !== created.id)]
    emit('updated', created)
    pushToast('success', t('lateSubmission.carrier.submitSuccess'))
  } catch (error) {
    errorKey.value = classifyLateSubmissionHttpError(error)
  } finally {
    acting.value = false
  }
}

watch(() => [props.eventId, props.carrierCompanyId, props.deadline, enabled.value], () => {
  void loadMine()
}, { immediate: true })

onMounted(() => {
  if (!requestedUntil.value) requestedUntil.value = defaultRequestedUntil()
})

defineExpose({ reload: loadMine, current })
</script>

<template>
  <section
    v-if="showPanel"
    class="late-panel"
    data-testid="carrier-late-submission-panel"
  >
    <h2>{{ t('lateSubmission.carrier.title') }}</h2>
    <p class="muted">{{ t('lateSubmission.carrier.hint') }}</p>
    <p class="muted" data-testid="carrier-late-commercial-locked">
      {{ t('lateSubmission.carrier.commercialLocked') }}
    </p>
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="current" data-testid="carrier-late-request-status">
      {{ t('lateSubmission.carrier.status') }}:
      {{ t(`lateSubmission.status.${current.status}`, current.status) }}
      <span v-if="current.approved_valid_from && current.approved_valid_until">
        — {{ t('lateSubmission.carrier.window') }}
        {{ formatRfxDate(current.approved_valid_from) }}
        → {{ formatRfxDate(current.approved_valid_until) }}
      </span>
    </p>
    <p v-else class="muted" data-testid="carrier-late-request-empty">
      {{ t('lateSubmission.carrier.noRequest') }}
    </p>
    <p v-if="errorKey" class="error" data-testid="carrier-late-request-error">
      {{ t(`lateSubmission.errors.${errorKey}`) }}
    </p>
    <form v-if="canCreate" class="late-form" @submit.prevent="onCreate">
      <label>
        {{ t('lateSubmission.carrier.reasonCode') }}
        <select v-model="reasonCode" data-testid="carrier-late-reason-code">
          <option v-for="code in LATE_SUBMISSION_REASON_CODES" :key="code" :value="code">
            {{ t(`lateSubmission.reason.${code}`) }}
          </option>
        </select>
      </label>
      <label>
        {{ t('lateSubmission.carrier.reasonText') }}
        <textarea v-model="reasonText" required rows="3" data-testid="carrier-late-reason-text" />
      </label>
      <label>
        {{ t('lateSubmission.carrier.requestedUntil') }}
        <input v-model="requestedUntil" type="datetime-local" required data-testid="carrier-late-requested-until">
      </label>
      <Button
        type="submit"
        :loading="acting"
        data-testid="carrier-late-request-submit"
      >
        {{ t('lateSubmission.carrier.submit') }}
      </Button>
    </form>
  </section>
</template>

<style scoped>
.late-panel {
  display: grid;
  gap: 0.75rem;
}
.late-form {
  display: grid;
  gap: 0.75rem;
  max-width: 32rem;
}
.late-form label {
  display: grid;
  gap: 0.35rem;
}
.error {
  color: var(--color-danger, #b42318);
}
.muted {
  color: var(--color-muted, #667085);
}
</style>
