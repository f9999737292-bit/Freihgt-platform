<script setup lang="ts">
import { canArchiveDocument, canCancelDocument, type DocumentDetail, type SigningSession } from '~/types/document'

const props = defineProps<{
  document: DocumentDetail
  signingSession?: SigningSession | null
}>()
const emit = defineEmits<{
  updated: []
  createVersion: []
  addFile: []
  cancel: []
  createSigningSession: []
  addSignature: []
  archive: []
}>()

const { markReadyForSigning, archiveDocument } = useDocumentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const markingReady = ref(false)
const archiving = ref(false)

const isDraft = computed(() => props.document.document_status === 'DRAFT')
const isReadyForSigning = computed(() => props.document.document_status === 'READY_FOR_SIGNING')
const isSigned = computed(() => props.document.document_status === 'SIGNED')
const canAddSignature = computed(
  () =>
    !!props.signingSession &&
    props.signingSession.status !== 'COMPLETED' &&
    props.document.document_status !== 'SIGNED' &&
    props.document.document_status !== 'ARCHIVED',
)
const showCancel = computed(() => canCancelDocument(props.document.document_status))
const showArchive = computed(() => canArchiveDocument(props.document.document_status))

async function onReadyForSigning() {
  markingReady.value = true
  try {
    await markReadyForSigning(props.document.id)
    pushToast('success', t('documents.readyForSigningSuccess'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    markingReady.value = false
  }
}

async function onArchive() {
  if (!confirm(t('documents.archiveConfirm'))) return
  archiving.value = true
  try {
    await archiveDocument(props.document.id)
    pushToast('success', t('documents.documentArchived'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    archiving.value = false
  }
}
</script>

<template>
  <div class="actions-stack">
    <div v-if="isDraft" class="actions-row">
      <UiButton variant="secondary" @click="emit('createVersion')">
        {{ $t('documents.createVersion') }}
      </UiButton>
      <UiButton variant="secondary" @click="emit('addFile')">
        {{ $t('documents.addFile') }}
      </UiButton>
      <UiButton :loading="markingReady" :disabled="markingReady" @click="onReadyForSigning">
        {{ $t('documents.readyForSigning') }}
      </UiButton>
    </div>

    <div v-if="isReadyForSigning && !signingSession" class="actions-row">
      <UiButton @click="emit('createSigningSession')">
        {{ $t('documents.createSigningSession') }}
      </UiButton>
    </div>

    <div v-if="canAddSignature" class="actions-row">
      <UiButton @click="emit('addSignature')">{{ $t('documents.addSignature') }}</UiButton>
    </div>

    <UiCard v-if="isSigned" class="hint-card">
      <p>{{ $t('documents.documentSigned') }}</p>
      <p v-if="document.related_entity_type === 'SHIPMENT'" class="text-muted">
        {{ $t('documents.shipmentDocumentsHint') }}
      </p>
      <UiButton
        v-if="document.related_entity_type === 'SHIPMENT' && document.related_entity_id"
        variant="secondary"
        @click="$router.push(`/shipments/${document.related_entity_id}`)"
      >
        {{ $t('documents.openShipment') }}
      </UiButton>
    </UiCard>

    <div class="actions-row">
      <UiButton v-if="showArchive" :loading="archiving" variant="secondary" @click="onArchive">
        {{ $t('documents.archiveDocument') }}
      </UiButton>
      <UiButton v-if="showCancel" variant="secondary" @click="emit('cancel')">
        {{ $t('documents.cancelDocument') }}
      </UiButton>
    </div>
  </div>
</template>

<style scoped>
.actions-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.hint-card p {
  margin: 0 0 0.75rem;
}

.hint-card p + p {
  margin-top: -0.25rem;
}
</style>
