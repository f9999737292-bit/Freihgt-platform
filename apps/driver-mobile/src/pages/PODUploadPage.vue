<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import type { RequestResult } from '@/types/api'
import { sha256Hex } from '@/utils/checksum'
import { createOperationId } from '@/utils/idempotency'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const network = useNetworkStore()
const { t } = useI18n()

const shipmentId = String(route.params.shipmentId)
const errorMessage = ref('')
const selectedFile = ref<File | null>(null)
const submitting = ref(false)

function onFileSelected(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFile.value = input.files?.[0] ?? null
}

async function submit() {
  errorMessage.value = ''
  if (!selectedFile.value) {
    errorMessage.value = t('pod.noFile')
    return
  }
  if (submitting.value) return
  submitting.value = true

  const file = selectedFile.value
  const mimeType = file.type || 'application/octet-stream'
  const operationId = createOperationId('pod', shipmentId)
  const api = auth.createApi(() => network.online)

  let result: RequestResult<unknown> = { outcome: 'REQUEST_NOT_SENT' }
  try {
    const intent = await api.initiatePODUpload(shipmentId, {
      mimeType,
      fileName: file.name,
      idempotencyKey: operationId,
    })
    if (intent.outcome !== 'SUCCESS' || !intent.data) {
      result = intent
    } else {
      const buffer = await file.arrayBuffer()
      const upload = await api.uploadPODContent(
        shipmentId,
        intent.data.uploadId,
        intent.data.uploadToken,
        buffer,
        mimeType,
      )
      if (upload.outcome !== 'SUCCESS') {
        result = upload
      } else {
        const checksum = await sha256Hex(buffer)
        result = await api.completePODUpload(shipmentId, intent.data.uploadId, { checksumSha256: checksum })
      }
    }
  } finally {
    submitting.value = false
  }

  if (result.outcome === 'SUCCESS') {
    await router.replace({
      name: 'submission-result',
      query: { kind: 'pod', status: 'success', shipmentId },
    })
    return
  }

  if (result.outcome === 'REQUEST_SENT_RESPONSE_UNKNOWN') {
    await router.replace({
      name: 'submission-result',
      query: { kind: 'pod', status: 'unknown', shipmentId },
    })
    return
  }

  if (result.outcome === 'REQUEST_NOT_SENT') {
    errorMessage.value = t('pod.offline')
    return
  }

  errorMessage.value = result.error?.message || t('pod.failed')
}
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-buttons slot="start">
          <ion-back-button :default-href="`/shipments/${shipmentId}`" />
        </ion-buttons>
        <ion-title>{{ t('pod.title') }}</ion-title>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding">
      <p class="hint">{{ t('pod.hint') }}</p>

      <ion-item>
        <input type="file" accept="image/*,application/pdf" @change="onFileSelected" />
      </ion-item>
      <p v-if="selectedFile" class="file-name">{{ selectedFile.name }}</p>

      <ion-text v-if="errorMessage" color="danger">
        <p class="error">{{ errorMessage }}</p>
      </ion-text>

      <ion-button
        expand="block"
        size="large"
        color="success"
        class="submit-btn"
        :disabled="submitting || !selectedFile"
        @click="submit"
      >
        {{ submitting ? t('common.loading') : t('pod.submit') }}
      </ion-button>
    </ion-content>
  </ion-page>
</template>

<style scoped>
.hint {
  margin-bottom: 16px;
  color: var(--ion-color-medium);
}
.file-name {
  margin: 8px 4px;
  font-size: 0.95rem;
}
.error {
  margin: 12px 4px;
}
.submit-btn {
  margin-top: 20px;
  min-height: 56px;
}
</style>
