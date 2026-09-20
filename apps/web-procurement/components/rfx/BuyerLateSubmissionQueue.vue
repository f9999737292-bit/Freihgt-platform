<script setup lang="ts">
import type { LateSubmissionRequest } from '~/types/lateSubmission'
import { formatRfxDate, toRFC3339 } from '~/types/rfx'
import {
  canDecideLateSubmission,
  canShowBuyerLateQueue,
  hasLateSubmissionBuyerManageRole,
} from '~/utils/lateSubmissionAccess'
import { classifyLateSubmissionHttpError } from '~/utils/lateSubmissionErrors'
import { createLateSubmissionIdempotencyStore } from '~/utils/lateSubmissionIdempotency'

const props = defineProps<{
  eventId: string
}>()

const { t } = useI18n()
const { pushToast } = useToast()
const authStore = useAuthStore()
const { enabled } = useRfxLateSubmissionFeature()
const api = useLateSubmissionApi()
const idempotency = createLateSubmissionIdempotencyStore()

const loading = ref(false)
const actingId = ref<string | null>(null)
const errorKey = ref<string | null>(null)
const items = ref<LateSubmissionRequest[]>([])
const windows = reactive<Record<string, { from: string; until: string; comment: string }>>({})

const roles = computed(() => authStore.user?.roles ?? [])
const showQueue = computed(() => canShowBuyerLateQueue({
  lateSubmissionEnabled: enabled.value,
  roles: roles.value,
}))
const canManage = computed(() => hasLateSubmissionBuyerManageRole(roles.value) && enabled.value)

function defaultWindow() {
  const from = new Date()
  const until = new Date(Date.now() + 24 * 3600 * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  const fmt = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
  return { from: fmt(from), until: fmt(until), comment: '' }
}

function windowFor(id: string) {
  if (!windows[id]) windows[id] = defaultWindow()
  return windows[id]
}

async function loadQueue() {
  if (!showQueue.value) {
    items.value = []
    return
  }
  loading.value = true
  errorKey.value = null
  try {
    const result = await api.listBuyerQueue(props.eventId)
    items.value = result.items ?? []
    for (const item of items.value) {
      windowFor(item.id)
    }
  } catch (error) {
    errorKey.value = classifyLateSubmissionHttpError(error)
    items.value = []
  } finally {
    loading.value = false
  }
}

async function onApprove(item: LateSubmissionRequest) {
  if (!canDecideLateSubmission({
    lateSubmissionEnabled: enabled.value,
    roles: roles.value,
    request: item,
  })) return
  actingId.value = item.id
  errorKey.value = null
  const win = windowFor(item.id)
  try {
    const updated = await api.approveRequest(
      props.eventId,
      item.id,
      {
        expected_version: item.version,
        approved_valid_from: toRFC3339(win.from),
        approved_valid_until: toRFC3339(win.until),
        decision_comment: win.comment.trim() || undefined,
      },
      idempotency.keyForApprove(item.id, item.version),
    )
    items.value = items.value.map((row) => row.id === updated.id ? updated : row)
    pushToast('success', t('lateSubmission.buyer.approveSuccess'))
  } catch (error) {
    errorKey.value = classifyLateSubmissionHttpError(error)
  } finally {
    actingId.value = null
  }
}

async function onReject(item: LateSubmissionRequest) {
  if (!canDecideLateSubmission({
    lateSubmissionEnabled: enabled.value,
    roles: roles.value,
    request: item,
  })) return
  actingId.value = item.id
  errorKey.value = null
  const win = windowFor(item.id)
  try {
    const updated = await api.rejectRequest(
      props.eventId,
      item.id,
      {
        expected_version: item.version,
        decision_comment: win.comment.trim() || undefined,
      },
      idempotency.keyForReject(item.id, item.version),
    )
    items.value = items.value.map((row) => row.id === updated.id ? updated : row)
    pushToast('success', t('lateSubmission.buyer.rejectSuccess'))
  } catch (error) {
    errorKey.value = classifyLateSubmissionHttpError(error)
  } finally {
    actingId.value = null
  }
}

watch(() => [props.eventId, enabled.value], () => {
  void loadQueue()
}, { immediate: true })
</script>

<template>
  <section
    v-if="showQueue"
    class="late-queue"
    data-testid="buyer-late-submission-queue"
  >
    <h2>{{ t('lateSubmission.buyer.title') }}</h2>
    <p v-if="!canManage" class="muted" data-testid="buyer-late-readonly">
      {{ t('lateSubmission.buyer.readOnly') }}
    </p>
    <p v-if="loading" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="items.length === 0" data-testid="buyer-late-empty">
      {{ t('lateSubmission.buyer.empty') }}
    </p>
    <p v-if="errorKey" class="error" data-testid="buyer-late-error">
      {{ t(`lateSubmission.errors.${errorKey}`) }}
    </p>
    <ul v-if="items.length" class="late-list">
      <li
        v-for="item in items"
        :key="item.id"
        class="late-item"
        :data-testid="`buyer-late-item-${item.id}`"
      >
        <p>
          <strong>{{ t('lateSubmission.buyer.carrier') }}:</strong>
          <span data-testid="buyer-late-carrier">{{ item.carrier_company_id }}</span>
        </p>
        <p>
          <strong>{{ t('lateSubmission.buyer.reason') }}:</strong>
          {{ t(`lateSubmission.reason.${item.reason_code}`, item.reason_code) }}
          — {{ item.reason_text }}
        </p>
        <p>
          <strong>{{ t('lateSubmission.carrier.status') }}:</strong>
          <span data-testid="buyer-late-status">
            {{ t(`lateSubmission.status.${item.status}`, item.status) }}
          </span>
        </p>
        <p>
          <strong>{{ t('lateSubmission.buyer.requestedUntil') }}:</strong>
          {{ formatRfxDate(item.requested_until) }}
        </p>
        <template v-if="canDecideLateSubmission({ lateSubmissionEnabled: enabled, roles, request: item })">
          <label>
            {{ t('lateSubmission.buyer.windowFrom') }}
            <input v-model="windowFor(item.id).from" type="datetime-local" data-testid="buyer-late-valid-from">
          </label>
          <label>
            {{ t('lateSubmission.buyer.windowUntil') }}
            <input v-model="windowFor(item.id).until" type="datetime-local" data-testid="buyer-late-valid-until">
          </label>
          <label>
            {{ t('lateSubmission.buyer.comment') }}
            <input v-model="windowFor(item.id).comment" type="text" data-testid="buyer-late-comment">
          </label>
          <div class="actions">
            <Button
              :loading="actingId === item.id"
              data-testid="buyer-late-approve"
              @click="onApprove(item)"
            >
              {{ t('lateSubmission.buyer.approve') }}
            </Button>
            <Button
              variant="danger"
              :loading="actingId === item.id"
              data-testid="buyer-late-reject"
              @click="onReject(item)"
            >
              {{ t('lateSubmission.buyer.reject') }}
            </Button>
          </div>
        </template>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.late-queue {
  display: grid;
  gap: 0.75rem;
}
.late-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 1rem;
}
.late-item {
  display: grid;
  gap: 0.4rem;
  border: 1px solid var(--color-border, #d0d5dd);
  padding: 0.75rem;
}
.late-item label {
  display: grid;
  gap: 0.25rem;
}
.actions {
  display: flex;
  gap: 0.5rem;
}
.error {
  color: var(--color-danger, #b42318);
}
.muted {
  color: var(--color-muted, #667085);
}
</style>
