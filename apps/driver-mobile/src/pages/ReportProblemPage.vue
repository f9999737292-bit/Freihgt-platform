<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { clearDraft, draftKey, loadDraft, saveDraft, useSubmissionLock } from '@/composables/useSubmission'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import { DRIVER_EXCEPTION_CATEGORIES, type DriverExceptionCategory } from '@/types/driver'
import { createOperationId } from '@/utils/idempotency'
import { buildExceptionRequestPayload } from '@/utils/contractSchemas'

interface ProblemDraft {
  category: DriverExceptionCategory
  comment: string
  operationId: string
}

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const network = useNetworkStore()
const { t } = useI18n()
const { submitting, runOnce } = useSubmissionLock()

const shipmentId = String(route.params.shipmentId)
const draftStorageKey = draftKey('problem', shipmentId)

const category = ref<DriverExceptionCategory>('VEHICLE_BREAKDOWN')
const comment = ref('')
const operationId = ref(createOperationId('problem', shipmentId))
const errorMessage = ref('')

const categoryOptions = DRIVER_EXCEPTION_CATEGORIES.map((code) => ({
  value: code,
  labelKey: `problem.categories.${code}`,
}))

onMounted(() => {
  const saved = loadDraft<ProblemDraft>(draftStorageKey)
  if (saved) {
    category.value = saved.category
    comment.value = saved.comment
    operationId.value = saved.operationId
  }
})

function persistDraft() {
  saveDraft<ProblemDraft>(draftStorageKey, {
    category: category.value,
    comment: comment.value,
    operationId: operationId.value,
  })
}

async function submit() {
  errorMessage.value = ''
  persistDraft()

  const payload = buildExceptionRequestPayload({
    category: category.value,
    comment: comment.value,
    idempotencyKey: operationId.value,
  })

  const api = auth.createApi(() => network.online)
  const result = await runOnce(() => api.reportProblem(shipmentId, payload))

  if (result.outcome === 'SUCCESS') {
    clearDraft(draftStorageKey)
    await router.replace({
      name: 'submission-result',
      query: {
        kind: 'problem',
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
      query: { kind: 'problem', status: 'unknown', shipmentId },
    })
    return
  }

  if (result.outcome === 'REQUEST_NOT_SENT') {
    errorMessage.value = t('problem.offline')
    return
  }

  errorMessage.value = result.error?.message || t('problem.failed')
}
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-buttons slot="start">
          <ion-back-button :default-href="`/shipments/${shipmentId}`" />
        </ion-buttons>
        <ion-title>{{ t('problem.title') }}</ion-title>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding form-content">
      <ion-list>
        <ion-item>
          <ion-label position="stacked">{{ t('problem.category') }}</ion-label>
          <ion-select v-model="category" interface="action-sheet" @ion-change="persistDraft">
            <ion-select-option v-for="option in categoryOptions" :key="option.value" :value="option.value">
              {{ t(option.labelKey) }}
            </ion-select-option>
          </ion-select>
        </ion-item>
        <ion-item>
          <ion-label position="stacked">{{ t('problem.comment') }}</ion-label>
          <ion-textarea v-model="comment" rows="4" @ion-input="persistDraft" />
        </ion-item>
      </ion-list>

      <ion-text v-if="errorMessage" color="danger">
        <p class="error">{{ errorMessage }}</p>
      </ion-text>

      <ion-button
        expand="block"
        size="large"
        color="danger"
        class="submit-btn"
        :disabled="submitting"
        @click="submit"
      >
        {{ submitting ? t('common.loading') : t('problem.submit') }}
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
