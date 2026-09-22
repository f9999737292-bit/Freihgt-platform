<script setup lang="ts">
import { computed, ref } from 'vue'
import { canShowBuyerXlsxCreateEntry } from '~/utils/buyerXlsxAccess'
import {
  performBuyerXlsxCreateTemplateDownload,
  triggerBuyerXlsxCreateTemplateDownload,
  type BuyerXlsxCreateTemplateErrorKind,
  type BuyerXlsxCreateTemplateFlowSnapshot,
  type BuyerXlsxCreateTemplateLabels,
  type BuyerXlsxCreateTemplatePayload,
} from '~/utils/buyerXlsxCreateTemplate'

const props = defineProps<{
  excelExchangeEnabled: boolean
  roles: readonly string[]
  download: () => Promise<BuyerXlsxCreateTemplatePayload>
  labels: BuyerXlsxCreateTemplateLabels
  flow: BuyerXlsxCreateTemplateFlowSnapshot
}>()

type DownloadPhase = 'idle' | 'downloading' | 'success' | 'error'

const phase = ref<DownloadPhase>('idle')
const errorKind = ref<BuyerXlsxCreateTemplateErrorKind | ''>('')

const visible = computed(() => canShowBuyerXlsxCreateEntry({
  excelExchangeEnabled: props.excelExchangeEnabled,
  roles: props.roles,
}))

const statusText = computed(() => {
  if (phase.value === 'downloading') return props.labels.downloading
  if (phase.value === 'success') return props.labels.success
  return ''
})

const errorText = computed(() => {
  switch (errorKind.value) {
    case 'unauthorized':
      return props.labels.unauthorized
    case 'forbidden':
      return props.labels.forbidden
    case 'notFound':
      return props.labels.notFound
    case 'rateLimited':
      return props.labels.rateLimited
    case 'invalid_binary':
      return props.labels.invalidBinary
    case 'unavailable':
      return props.labels.unavailable
    default:
      return ''
  }
})

function onDownload() {
  if (phase.value === 'downloading') return
  phase.value = 'downloading'
  errorKind.value = ''
  void runDownload()
}

async function runDownload() {
  const outcome = await performBuyerXlsxCreateTemplateDownload({
    fetchTemplate: () => props.download(),
    saveFile: (file) => {
      triggerBuyerXlsxCreateTemplateDownload(file.blob, file.filename)
    },
    flow: props.flow,
  })
  phase.value = outcome.phase
  errorKind.value = outcome.kind ?? ''
}
</script>

<template>
  <div
    v-if="visible"
    class="buyer-xlsx-create-template"
    :data-phase="phase"
  >
    <button
      type="button"
      class="buyer-xlsx-create-template-download"
      data-testid="buyer-xlsx-create-template-download"
      :disabled="phase === 'downloading'"
      :aria-busy="phase === 'downloading'"
      aria-describedby="buyer-xlsx-create-template-download-hint buyer-xlsx-create-template-download-status"
      @click="onDownload"
    >
      {{ phase === 'downloading' ? labels.downloading : labels.download }}
    </button>
    <p id="buyer-xlsx-create-template-download-hint" class="buyer-xlsx-create-template__hint">
      {{ labels.hint }}
    </p>
    <p
      id="buyer-xlsx-create-template-download-status"
      class="buyer-xlsx-create-template__status"
      data-testid="buyer-xlsx-create-template-download-status"
      role="status"
      aria-live="polite"
    >
      {{ statusText }}
    </p>
    <p
      v-if="phase === 'error'"
      class="buyer-xlsx-create-template__error"
      data-testid="buyer-xlsx-create-template-download-error"
      :data-error-kind="errorKind"
      role="alert"
      aria-live="assertive"
    >
      {{ errorText }}
    </p>
  </div>
</template>

<style scoped>
.buyer-xlsx-create-template {
  flex: 1 1 18rem;
  max-width: 36rem;
}

.buyer-xlsx-create-template-download {
  display: inline-flex;
  align-items: center;
  min-height: 38px;
  padding: 0.5rem 1rem;
  border: 1px solid var(--color-border, #d0d5dd);
  border-radius: var(--radius-md, 8px);
  background: var(--color-surface, #fff);
  color: var(--color-text, #101828);
  font: inherit;
  font-weight: 500;
  cursor: pointer;
}

.buyer-xlsx-create-template-download:focus,
.buyer-xlsx-create-template-download:focus-visible {
  outline: 2px solid var(--color-primary, #175cd3);
  outline-offset: 2px;
}

.buyer-xlsx-create-template-download:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.buyer-xlsx-create-template__hint,
.buyer-xlsx-create-template__status {
  color: var(--color-text-muted, #667085);
  margin: 0.5rem 0 0;
}

.buyer-xlsx-create-template__error {
  border: 1px solid var(--color-danger, #b42318);
  border-radius: var(--radius-md, 8px);
  margin: 0.5rem 0 0;
  padding: 0.75rem 1rem;
}
</style>
