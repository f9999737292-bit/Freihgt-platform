<script setup lang="ts">
import { ref } from 'vue'
import { Button, Card, CustomerPageHeader, Input, Table } from '@freight-platform/ui'
import PortalState from '../PortalState.vue'
import type { ShipperAttachment, ShipperDocument } from '../../domain/document'
import { displayFact } from '../../domain/shipment'
import type { PortalViewKind } from '../../domain/viewState'

defineProps<{
  shipmentId: string | null
  shipments: Array<{ id: string; shipment_number?: string; status?: string }>
  shipmentsKind: PortalViewKind
  shipmentsTitle: string
  shipmentsDetail?: string
  documentsKind: PortalViewKind
  documentsTitle: string
  documentsDetail?: string
  documents: ShipperDocument[]
  documentsTotal: number
  documentsOffset: number
  documentId: string | null
  documentKind: PortalViewKind
  documentTitle: string
  documentDetail?: string
  document: ShipperDocument | null
  attachmentsKind: PortalViewKind
  attachmentsTitle: string
  attachmentsDetail?: string
  attachments: ShipperAttachment[]
  downloadDetail?: string
}>()

const emit = defineEmits<{
  'select-shipment': [shipmentId: string]
  'open-document': [documentId: string]
  'close-document': []
  download: [attachmentId: string]
  'documents-filter': [filters: { documentType: string; documentStatus: string }]
  'documents-page': [offset: number]
}>()

const { t } = useI18n()
const documentType = ref('')
const documentStatus = ref('')

function applyFilters() {
  emit('documents-filter', { documentType: documentType.value, documentStatus: documentStatus.value })
}
</script>

<template>
  <section class="workspace" data-testid="documents-workspace">
    <CustomerPageHeader :title="t('shipper.documentsTitle')" />

    <section v-if="!shipmentId" data-testid="shipment-picker" aria-labelledby="shipment-picker-heading">
      <h2 id="shipment-picker-heading">{{ t('shipper.chooseShipment') }}</h2>
      <p>{{ t('shipper.documentsNeedShipment') }}</p>
      <PortalState v-if="shipmentsKind !== 'ready'" :kind="shipmentsKind" :title="shipmentsTitle" :description="shipmentsDetail" />
      <Card v-else>
        <ul class="choices">
          <li v-for="item in shipments" :key="item.id">
            <Button type="button" data-testid="open-shipment-documents" @click="emit('select-shipment', item.id)">
              {{ displayFact(item.shipment_number) }}
            </Button>
            <span>{{ displayFact(item.status) }}</span>
          </li>
        </ul>
      </Card>
    </section>

    <template v-else>
      <section data-testid="document-list" aria-labelledby="document-list-heading">
        <h2 id="document-list-heading">{{ t('shipper.documentList') }}</h2>
        <form class="filters" @submit.prevent="applyFilters">
          <Input v-model="documentType" name="document_type" data-testid="document-type-filter" :label="t('shipper.documentType')" />
          <Input v-model="documentStatus" name="document_status" data-testid="document-status-filter" :label="t('shipper.documentStatus')" />
          <Button type="submit" variant="ghost">{{ t('shipper.apply') }}</Button>
        </form>
        <PortalState v-if="documentsKind !== 'ready'" :kind="documentsKind" :title="documentsTitle" :description="documentsDetail" />
        <Card v-else>
          <Table
            :columns="[
              t('shipper.documentNumber'),
              t('shipper.documentType'),
              t('shipper.documentStatus'),
              t('shipper.createdAt'),
              t('shipper.updatedAt'),
              t('shipper.version'),
              '',
            ]"
          >
            <tr v-for="item in documents" :key="item.id" data-testid="document-row">
              <td>{{ displayFact(item.document_number) }}</td>
              <td>{{ displayFact(item.document_type) }}</td>
              <td>{{ displayFact(item.document_status) }}</td>
              <td>{{ displayFact(item.created_at) }}</td>
              <td>{{ displayFact(item.updated_at) }}</td>
              <td>{{ item.version ?? '—' }}</td>
              <td>
                <Button type="button" data-testid="open-document" @click="emit('open-document', item.id)">
                  {{ t('shipper.open') }}
                </Button>
              </td>
            </tr>
          </Table>
          <Button
            v-if="documentsOffset + documents.length < documentsTotal"
            type="button"
            variant="ghost"
            data-testid="documents-next"
            @click="emit('documents-page', documentsOffset + documents.length)"
          >
            {{ t('shipper.nextPage') }}
          </Button>
        </Card>
      </section>

      <section v-if="documentId" data-testid="document-detail" aria-labelledby="document-detail-heading">
        <h2 id="document-detail-heading">{{ t('shipper.documentDetail') }}</h2>
        <Button type="button" variant="ghost" data-testid="close-document" @click="emit('close-document')">
          {{ t('shipper.backToDocuments') }}
        </Button>
        <PortalState v-if="documentKind !== 'ready'" :kind="documentKind" :title="documentTitle" :description="documentDetail" />
        <Card v-else-if="document">
          <dl class="facts">
            <div>
              <dt>{{ t('shipper.documentNumber') }}</dt>
              <dd data-testid="document-number">{{ displayFact(document.document_number) }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.documentType') }}</dt>
              <dd>{{ displayFact(document.document_type) }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.documentStatus') }}</dt>
              <dd>{{ displayFact(document.document_status) }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.relatedShipment') }}</dt>
              <dd>{{ displayFact(document.related_entity_id) }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.legalLanguage') }}</dt>
              <dd>{{ displayFact(document.legal_language) }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.version') }}</dt>
              <dd>{{ document.version ?? '—' }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.createdAt') }}</dt>
              <dd>{{ displayFact(document.created_at) }}</dd>
            </div>
            <div>
              <dt>{{ t('shipper.updatedAt') }}</dt>
              <dd>{{ displayFact(document.updated_at) }}</dd>
            </div>
          </dl>
          <h3>{{ t('shipper.attachments') }}</h3>
          <p v-if="downloadDetail" role="alert" data-testid="download-error">{{ downloadDetail }}</p>
          <PortalState v-if="attachmentsKind !== 'ready'" :kind="attachmentsKind" :title="attachmentsTitle" :description="attachmentsDetail" />
          <Table
            v-else
            :columns="[
              t('shipper.fileName'),
              t('shipper.mediaType'),
              t('shipper.fileSize'),
              t('shipper.documentStatus'),
              t('shipper.createdAt'),
              t('shipper.finalizedAt'),
              '',
            ]"
          >
            <tr v-for="item in attachments" :key="item.id" data-testid="attachment-row">
              <td>{{ displayFact(item.file_name) }}</td>
              <td>{{ displayFact(item.media_type) }}</td>
              <td>{{ item.size_bytes ?? '—' }}</td>
              <td>{{ displayFact(item.status) }}</td>
              <td>{{ displayFact(item.created_at) }}</td>
              <td>{{ displayFact(item.finalized_at) }}</td>
              <td>
                <Button
                  type="button"
                  data-testid="download-attachment"
                  :aria-label="`${t('shipper.download')} ${displayFact(item.file_name)}`"
                  @click="emit('download', item.id)"
                >
                  {{ t('shipper.download') }}
                </Button>
              </td>
            </tr>
          </Table>
        </Card>
      </section>
    </template>
  </section>
</template>

<style scoped>
.workspace {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  min-width: 0;
  max-width: 100%;
}
.choices {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.choices li,
.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}
.facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  margin: 0 0 1rem;
}
.facts dt {
  color: #6b7280;
  font-size: 0.75rem;
  font-weight: 600;
}
.facts dd {
  margin: 0.15rem 0 0;
}
.workspace :deep(a:focus-visible),
.workspace :deep(button:focus-visible),
.workspace :deep(input:focus-visible),
.workspace :deep(select:focus-visible) {
  outline: 2px solid #0f3d4c;
  outline-offset: 2px;
}
</style>
