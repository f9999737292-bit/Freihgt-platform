<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { clearDraft, draftKey, loadDraft, saveDraft, useSubmissionLock } from '@/composables/useSubmission'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import { DRIVER_DELAY_REASON_CODES, type DriverDelayReasonCode } from '@/types/driver'
import { createOperationId } from '@/utils/idempotency'
import { buildDelayRequestPayload } from '@/utils/contractSchemas'

function toIsoOrUndefined(value: string): string | undefined {
  if (!value.trim()) return undefined
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

interface DelayDraft {
  reasonCode: DriverDelayReasonCode
  reasonText: string
  newEta: string
  operationId: string
}

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const network = useNetworkStore()
const { t } = useI18n()
const { submitting, runOnce } = useSubmissionLock()

const shipmentId = String(route.params.shipmentId)
const draftStorageKey = draftKey('delay', shipmentId)

const reasonCode = ref<DriverDelayReasonCode>('TRAFFIC')
const reasonText = ref('')
const newEta = ref('')
const operationId = ref(createOperationId('delay', shipmentId))
const errorMessage = ref('')

const reasonOptions = DRIVER_DELAY_REASON_CODES.map((code) => ({
  value: code,
  labelKey: `delay.reasons.${code}`,
}))

onMounted(() => {
  const saved = loadDraft<DelayDraft>(draftStorageKey)
  if (saved) {
    reasonCode.value = saved.reasonCode
    reasonText.value = saved.reasonText
    newEta.value = saved.newEta
    operationId.value = saved.operationId
  }
})

function persistDraft() {
  saveDraft<DelayDraft>(draftStorageKey, {
    reasonCode: reasonCode.value,
    reasonText: reasonText.value,
    newEta: newEta.value,
    operationId: operationId.value,
  })
}

async function submit() {
  errorMessage.value = ''
  persistDraft()

  const payload = buildDelayRequestPayload({
    reasonCode: reasonCode.value,
    reasonText: reasonText.value,
    newEta: toIsoOrUndefined(newEta.value),
    idempotencyKey: operationId.value,
  })

  const api = auth.createApi(() => network.online)
  const result = await runOnce(() => api.reportDelay(shipmentId, payload))

  if (result.outcome === 'SUCCESS') {
    clearDraft(draftStorageKey)
    await router.replace({
      name: 'submission-result',
      query: {
        kind: 'delay',
        status: 'success',
        shipmentId,
        replayed: result.data?.replayed ? '1' : '0',
      },
    })
    return
  }

  if (result.outcome === 'REQUEST_SENT_RESPONSE_UNKNOWN') {
    await router.replace({
      name: 'submission-result',
      query: { kind: 'delay', status: 'unknown', shipmentId },
    })
    return
  }

  if (result.outcome === 'REQUEST_NOT_SENT') {
    errorMessage.value = t('delay.offline')
    return
  }

  errorMessage.value = result.error?.message || t('delay.failed')
}
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-buttons slot="start">
          <ion-back-button :default-href="`/shipments/${shipmentId}`" />
        </ion-buttons>
        <ion-title>{{ t('delay.title') }}</ion-title>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding form-content">
      <ion-list>
        <ion-item>
          <ion-label position="stacked">{{ t('delay.reason') }}</ion-label>
          <ion-select v-model="reasonCode" interface="action-sheet" @ion-change="persistDraft">
            <ion-select-option v-for="option in reasonOptions" :key="option.value" :value="option.value">
              {{ t(option.labelKey) }}
            </ion-select-option>
          </ion-select>
        </ion-item>
        <ion-item>
          <ion-label position="stacked">{{ t('delay.comment') }}</ion-label>
          <ion-textarea v-model="reasonText" rows="3" @ion-input="persistDraft" />
        </ion-item>
        <ion-item>
          <ion-label position="stacked">{{ t('delay.newEta') }}</ion-label>
          <ion-input v-model="newEta" type="datetime-local" @ion-input="persistDraft" />
        </ion-item>
      </ion-list>

      <ion-text v-if="errorMessage" color="danger">
        <p class="error">{{ errorMessage }}</p>
      </ion-text>

      <ion-button
        expand="block"
        size="large"
        color="warning"
        class="submit-btn"
        :disabled="submitting"
        @click="submit"
      >
        {{ submitting ? t('common.loading') : t('delay.submit') }}
      </ion-button>
    </ion-content>
  </ion-page>
</template>

<style scoped>
.error {
  margin: 12px 4px;
}
.submit-btn {
  margin-top: 20px;
  min-height: 56px;
}
</style>
