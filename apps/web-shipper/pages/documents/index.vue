<script setup lang="ts">
import { Select } from '@freight-platform/ui'
import DocumentWorkspace from '../../components/documents/DocumentWorkspace.vue'
import { DOCUMENT_PAGE_SIZE, documentMatchesShipment, downloadName, retainShipmentContext, type ShipperAttachment, type ShipperDocument } from '../../domain/document'
import type { ShipperShipment } from '../../domain/shipment'
import { viewKindFromError, type PortalViewKind } from '../../domain/viewState'

const route = useRoute()
const router = useRouter()
const localePath = useLocalePath()
const { t } = useI18n()
const api = useShipperApi()
const office = useShipperOffice()

const seenCompany = ref<string | null>(null)
const ticket = ref(0)
const shipments = ref<ShipperShipment[]>([])
const shipmentsKind = ref<PortalViewKind>('loading')
const shipmentsDetail = ref('')
const documents = ref<ShipperDocument[]>([])
const documentsKind = ref<PortalViewKind>('loading')
const documentsDetail = ref('')
const documentsTotal = ref(0)
const documentsOffset = ref(0)
const documentType = ref('')
const documentStatus = ref('')
const documentRecord = ref<ShipperDocument | null>(null)
const documentKind = ref<PortalViewKind>('loading')
const documentDetail = ref('')
const attachments = ref<ShipperAttachment[]>([])
const attachmentsKind = ref<PortalViewKind>('loading')
const attachmentsDetail = ref('')
const downloadDetail = ref('')

const shipmentId = computed(() => queryValue('shipment_id'))
const documentId = computed(() => queryValue('document_id'))
const companyOptions = computed(() => office.companies.value.map((item) => ({
  value: item.companyId,
  label: item.legalName || item.companyId,
})))
const companyModel = computed({
  get: () => office.selectedCompanyId.value ?? '',
  set: (value: string) => {
    if (value && value !== office.selectedCompanyId.value) office.chooseCompany(value)
  },
})

function queryValue(name: string): string | null {
  const value = route.query[name]
  return typeof value === 'string' && value.trim() !== '' ? value : null
}

function titleFor(state: PortalViewKind, emptyKey: string) {
  if (state === 'empty') return t(emptyKey)
  if (state === 'forbidden') return t('shipper.forbidden')
  if (state === 'not_found') return t('shipper.notFound')
  if (state === 'unauthorized') return t('shipper.forbidden')
  if (state === 'loading') return t('shipper.loading')
  if (state === 'rejected') return t('shipper.rejected')
  return t('shipper.unavailable')
}

function clearDocuments() {
  documents.value = []
  documentsTotal.value = 0
  documentsOffset.value = 0
  documentsKind.value = 'loading'
  documentRecord.value = null
  documentKind.value = 'loading'
  attachments.value = []
  attachmentsKind.value = 'loading'
  downloadDetail.value = ''
  documentsDetail.value = ''
  documentDetail.value = ''
  attachmentsDetail.value = ''
}

async function loadShipments() {
  const current = ++ticket.value
  const company = office.requireCompany()
  shipments.value = []
  shipmentsKind.value = 'loading'
  shipmentsDetail.value = ''
  clearDocuments()
  try {
    const page = await api.listShipments()
    if (current !== ticket.value || office.selectedCompanyId.value !== company) return
    shipments.value = page.items ?? []
    shipmentsKind.value = shipments.value.length === 0 ? 'empty' : 'ready'
  } catch (error) {
    if (current !== ticket.value) return
    shipments.value = []
    shipmentsKind.value = viewKindFromError(error)
    shipmentsDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadDocuments(shipment: string, offset = documentsOffset.value) {
  const current = ++ticket.value
  const company = office.requireCompany()
  documents.value = []
  documentsKind.value = 'loading'
  documentsDetail.value = ''
  documentsOffset.value = offset
  try {
    const page = await api.listDocuments(shipment, {
      documentType: documentType.value,
      documentStatus: documentStatus.value,
      limit: DOCUMENT_PAGE_SIZE,
      offset,
    })
    if (current !== ticket.value || office.selectedCompanyId.value !== company || shipmentId.value !== shipment) return
    documents.value = page.items ?? []
    documentsTotal.value = page.total ?? documents.value.length
    documentsKind.value = documents.value.length === 0 ? 'empty' : 'ready'
  } catch (error) {
    if (current !== ticket.value) return
    documents.value = []
    documentsKind.value = viewKindFromError(error)
    documentsDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadDocument(id: string, shipment: string) {
  const current = ticket.value
  const company = office.requireCompany()
  documentRecord.value = null
  attachments.value = []
  documentKind.value = 'loading'
  attachmentsKind.value = 'loading'
  documentDetail.value = ''
  attachmentsDetail.value = ''
  downloadDetail.value = ''
  try {
    const item = await api.getDocument(id)
    if (current !== ticket.value || office.selectedCompanyId.value !== company || documentId.value !== id || shipmentId.value !== shipment) return
    if (!documentMatchesShipment(item, shipment)) {
      documentRecord.value = null
      attachments.value = []
      documentKind.value = 'not_found'
      attachmentsKind.value = 'not_found'
      return
    }
    const files = await api.listDocumentAttachments(id)
    if (current !== ticket.value || office.selectedCompanyId.value !== company || documentId.value !== id || shipmentId.value !== shipment) return
    documentRecord.value = item
    attachments.value = files.items ?? []
    documentKind.value = 'ready'
    attachmentsKind.value = 'ready'
  } catch (error) {
    if (current !== ticket.value) return
    documentRecord.value = null
    attachments.value = []
    const kind = viewKindFromError(error)
    documentKind.value = kind
    attachmentsKind.value = kind
    documentDetail.value = error instanceof Error ? error.message : ''
  }
}

async function refresh() {
  const company = office.selectedCompanyId.value
  if (!company) return
  const retained = retainShipmentContext(seenCompany.value, company, shipmentId.value)
  const companyChanged = seenCompany.value !== null && seenCompany.value !== company
  seenCompany.value = company
  if (companyChanged) {
    shipments.value = []
    clearDocuments()
    if (shipmentId.value || documentId.value) {
      await router.replace(localePath('/documents'))
      return
    }
  }
  if (!retained) {
    await loadShipments()
    return
  }
  await loadDocuments(retained, documentsOffset.value)
  if (documentId.value && shipmentId.value && office.selectedCompanyId.value === company) {
    await loadDocument(documentId.value, shipmentId.value)
  } else {
    documentRecord.value = null
    attachments.value = []
  }
}

async function onDownload(attachmentId: string) {
  const currentDocument = documentId.value
  const currentShipment = shipmentId.value
  const attachment = attachments.value.find((item) => item.id === attachmentId)
  if (!currentDocument || !currentShipment || !attachment || !documentRecord.value) return
  if (!documentMatchesShipment(documentRecord.value, currentShipment)) return
  const company = office.requireCompany()
  downloadDetail.value = ''
  try {
    const result = await api.downloadDocumentAttachment(currentDocument, attachmentId)
    if (office.selectedCompanyId.value !== company || documentId.value !== currentDocument) return
    const name = downloadName(result.contentDisposition, attachment.file_name)
    const url = URL.createObjectURL(result.blob)
    const link = window.document.createElement('a')
    link.href = url
    link.download = name
    link.rel = 'noopener'
    window.document.body.appendChild(link)
    link.click()
    link.remove()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (error) {
    if (office.selectedCompanyId.value !== company) return
    downloadDetail.value = titleFor(viewKindFromError(error), 'shipper.unavailable')
  }
}

function onSelectShipment(id: string) {
  documentsOffset.value = 0
  void router.push(localePath({ path: '/documents', query: { shipment_id: id } }))
}

function onOpenDocument(id: string) {
  if (!shipmentId.value) return
  void router.push(localePath({ path: '/documents', query: { shipment_id: shipmentId.value, document_id: id } }))
}

function onCloseDocument() {
  if (!shipmentId.value) return
  void router.push(localePath({ path: '/documents', query: { shipment_id: shipmentId.value } }))
}

function onPage(offset: number) {
  if (!shipmentId.value) return
  void loadDocuments(shipmentId.value, offset)
}

function onFilter(filters: { documentType: string; documentStatus: string }) {
  documentType.value = filters.documentType
  documentStatus.value = filters.documentStatus
  documentsOffset.value = 0
  if (shipmentId.value) void loadDocuments(shipmentId.value, 0)
}

watch(() => [office.selectedCompanyId.value, shipmentId.value, documentId.value], () => {
  void refresh()
}, { immediate: true })
</script>

<template>
  <Select
    v-if="companyOptions.length > 1"
    v-model="companyModel"
    name="company"
    data-testid="company-switch"
    :label="t('shipper.companyLabel')"
    :options="companyOptions"
  />
  <DocumentWorkspace
    :shipment-id="shipmentId"
    :shipments="shipments"
    :shipments-kind="shipmentsKind"
    :shipments-title="titleFor(shipmentsKind, 'shipper.emptyShipments')"
    :shipments-detail="shipmentsDetail"
    :documents-kind="documentsKind"
    :documents-title="titleFor(documentsKind, 'shipper.emptyDocuments')"
    :documents-detail="documentsDetail"
    :documents="documents"
    :documents-total="documentsTotal"
    :documents-offset="documentsOffset"
    :document-id="documentId"
    :document-kind="documentKind"
    :document-title="titleFor(documentKind, 'shipper.emptyDocuments')"
    :document-detail="documentDetail"
    :document="documentRecord"
    :attachments-kind="attachmentsKind"
    :attachments-title="titleFor(attachmentsKind, 'shipper.emptyAttachments')"
    :attachments-detail="attachmentsDetail"
    :attachments="attachments"
    :download-detail="downloadDetail"
    @select-shipment="onSelectShipment"
    @open-document="onOpenDocument"
    @close-document="onCloseDocument"
    @download="onDownload"
    @documents-filter="onFilter"
    @documents-page="onPage"
  />
</template>
