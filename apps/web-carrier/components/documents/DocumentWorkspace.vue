<script setup lang="ts">
import { ref } from 'vue'
import { Button, Card, Input, Table } from '@freight-platform/ui'
import PortalState from '../PortalState.vue'
import type { CarrierAttachment, CarrierDocument } from '../../domain/document'
import type { PortalViewKind } from '../../domain/viewState'

defineProps<{
  shipmentId: string
  documentsKind: PortalViewKind
  documentsTitle: string
  documentsDetail?: string
  documents: CarrierDocument[]
  documentKind: PortalViewKind
  documentTitle: string
  documentDetail?: string
  document: CarrierDocument | null
  attachmentsKind: PortalViewKind
  attachmentsTitle: string
  attachments: CarrierAttachment[]
  downloadDetail?: string
}>()

const emit = defineEmits<{
  open: [documentId: string]
  close: []
  download: [attachmentId: string]
  filter: [value: { documentType: string; documentStatus: string }]
}>()

const { t } = useI18n()
const documentType = ref('')
const documentStatus = ref('')
</script>

<template>
  <div data-testid="documents-workspace">
    <form class="filters" @submit.prevent="emit('filter', { documentType, documentStatus })">
      <Input v-model="documentType" name="document_type" data-testid="document-type-filter" :label="t('carrier.documentType')" />
      <Input v-model="documentStatus" name="document_status" data-testid="document-status-filter" :label="t('carrier.documentStatus')" />
      <Button type="submit" variant="ghost">{{ t('carrier.apply') }}</Button>
    </form>
    <PortalState v-if="documentsKind !== 'ready'" :kind="documentsKind" :title="documentsTitle" :description="documentsDetail" />
    <Card v-else>
      <p v-if="documents.length === 0" data-testid="documents-empty">{{ t('carrier.emptyDocuments') }}</p>
      <Table
        v-else
        :columns="[t('carrier.documentNumber'), t('carrier.documentType'), t('carrier.documentStatus'), t('carrier.createdAt'), '']"
      >
        <tr v-for="item in documents" :key="item.id" data-testid="document-row">
          <td data-testid="document-number">{{ item.document_number }}</td>
          <td>{{ item.document_type }}</td>
          <td>{{ item.document_status }}</td>
          <td>{{ item.created_at }}</td>
          <td>
            <Button type="button" variant="ghost" data-testid="open-document" @click="emit('open', item.id)">
              {{ t('carrier.open') }}
            </Button>
          </td>
        </tr>
      </Table>
    </Card>

    <section v-if="shipmentId" data-testid="document-detail">
      <PortalState v-if="documentKind === 'loading' || documentKind === 'forbidden' || documentKind === 'not_found' || documentKind === 'unavailable' || documentKind === 'rejected'" :kind="documentKind" :title="documentTitle" :description="documentDetail" />
      <Card v-else-if="document">
        <h2>{{ document.document_number || t('carrier.documentDetail') }}</h2>
        <p data-testid="document-type">{{ document.document_type }}</p>
        <p data-testid="document-status">{{ document.document_status }}</p>
        <p data-testid="related-shipment">{{ document.related_entity_id }}</p>
        <p>{{ document.updated_at }}</p>
        <Button type="button" variant="ghost" data-testid="close-document" @click="emit('close')">
          {{ t('carrier.backToDocuments') }}
        </Button>
        <PortalState v-if="attachmentsKind !== 'ready'" :kind="attachmentsKind" :title="attachmentsTitle" />
        <p v-else-if="attachments.length === 0" data-testid="attachments-empty">{{ t('carrier.emptyAttachments') }}</p>
        <Table
          v-else
          :columns="[t('carrier.fileName'), t('carrier.mediaType'), t('carrier.fileSize'), t('carrier.finalizedAt'), '']"
        >
          <tr v-for="file in attachments" :key="file.id" data-testid="attachment-row">
            <td>{{ file.file_name }}</td>
            <td>{{ file.media_type }}</td>
            <td>{{ file.size_bytes }}</td>
            <td>{{ file.finalized_at }}</td>
            <td>
              <Button
                type="button"
                data-testid="download-attachment"
                :aria-label="`${t('carrier.download')} ${file.file_name ?? ''}`"
                @click="emit('download', file.id)"
              >
                {{ t('carrier.download') }}
              </Button>
            </td>
          </tr>
        </Table>
        <p v-if="downloadDetail" data-testid="download-error">{{ downloadDetail }}</p>
      </Card>
    </section>
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}
</style>
