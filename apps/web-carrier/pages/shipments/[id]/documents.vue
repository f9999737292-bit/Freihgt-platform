<script setup lang="ts">
import { Select } from '@freight-platform/ui'
import { CustomerPageHeader } from '@freight-platform/ui'
import DocumentWorkspace from '../../../components/documents/DocumentWorkspace.vue'
import {
  DOCUMENT_PAGE_SIZE,
  documentMatchesShipment,
  downloadName,
  type CarrierAttachment,
  type CarrierDocument,
} from '../../../domain/document'
import { viewKindFromError, type PortalViewKind } from '../../../domain/viewState'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const localePath = useLocalePath()
const api = useCarrierApi()
const office = useCarrierOffice()

const shipmentId = computed(() => String(route.params.id ?? ''))
const documentId = computed(() => {
  const value = route.query.document_id
  return typeof value === 'string' && value.trim() !== '' ? value : null
})

const documents = ref<CarrierDocument[]>([])
const documentsKind = ref<PortalViewKind>('loading')
const documentsDetail = ref('')
const documentRecord = ref<CarrierDocument | null>(null)
const documentKind = ref<PortalViewKind>('ready')
const documentDetail = ref('')
const attachments = ref<CarrierAttachment[]>([])
const attachmentsKind = ref<PortalViewKind>('ready')
const downloadDetail = ref('')
const documentType = ref('')
const documentStatus = ref('')
const ticket = ref(0)

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

function titleFor(state: PortalViewKind, emptyKey: string) {
  if (state === 'empty') return t(emptyKey)
  if (state === 'forbidden') return t('carrier.forbidden')
  if (state === 'not_found') return t('carrier.documentNotFound')
  if (state === 'unauthorized') return t('carrier.forbidden')
  if (state === 'rejected') return t('carrier.rejected')
  if (state === 'loading') return t('carrier.loading')
  return t('carrier.unavailable')
}

function clearDocumentState() {
  documents.value = []
  documentRecord.value = null
  attachments.value = []
  documentsDetail.value = ''
  documentDetail.value = ''
  downloadDetail.value = ''
}

async function loadDocuments(shipment: string, company: string, current: number) {
  documentsKind.value = 'loading'
  documents.value = []
  documentsDetail.value = ''
  try {
    const page = await api.listDocuments(shipment, {
      documentType: documentType.value,
      documentStatus: documentStatus.value,
      limit: DOCUMENT_PAGE_SIZE,
      offset: 0,
    })
    if (current !== ticket.value || office.selectedCompanyId.value !== company || shipmentId.value !== shipment) return
    documents.value = page.items ?? []
    documentsKind.value = documents.value.length === 0 ? 'empty' : 'ready'
  } catch (error) {
    if (current !== ticket.value || office.selectedCompanyId.value !== company) return
    documents.value = []
    documentsKind.value = viewKindFromError(error)
    documentsDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadDocument(id: string, shipment: string, company: string, current: number) {
  documentRecord.value = null
  attachments.value = []
  documentKind.value = 'loading'
  attachmentsKind.value = 'loading'
  documentDetail.value = ''
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
    if (current !== ticket.value || office.selectedCompanyId.value !== company) return
    documentRecord.value = null
    attachments.value = []
    const kind = viewKindFromError(error)
    documentKind.value = kind
    attachmentsKind.value = kind
    documentDetail.value = error instanceof Error ? error.message : ''
  }
}

function applyFilters(value: { documentType: string; documentStatus: string }) {
  documentType.value = value.documentType
  documentStatus.value = value.documentStatus
  const company = office.selectedCompanyId.value
  const shipment = shipmentId.value
  if (!company || !shipment) return
  const current = ++ticket.value
  void loadDocuments(shipment, company, current)
}

async function refresh(company: string, shipment: string) {
  const current = ticket.value
  await loadDocuments(shipment, company, current)
  if (current !== ticket.value || office.selectedCompanyId.value !== company) return
  if (documentId.value) await loadDocument(documentId.value, shipment, company, current)
  else {
    documentRecord.value = null
    attachments.value = []
    documentKind.value = 'ready'
    attachmentsKind.value = 'ready'
  }
}

async function onDownload(attachmentId: string) {
  const currentDocument = documentId.value
  const shipment = shipmentId.value
  const attachment = attachments.value.find((item) => item.id === attachmentId)
  if (!currentDocument || !shipment || !attachment || !documentRecord.value) return
  if (!documentMatchesShipment(documentRecord.value, shipment)) return
  const company = office.selectedCompanyId.value
  downloadDetail.value = ''
  try {
    const file = await api.downloadDocumentAttachment(currentDocument, attachmentId)
    if (office.selectedCompanyId.value !== company || documentId.value !== currentDocument) return
    const url = URL.createObjectURL(file.blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = downloadName(file.contentDisposition, attachment.file_name)
    anchor.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (error) {
    downloadDetail.value = error instanceof Error ? error.message : t('carrier.unavailable')
  }
}

watch(
  () => [office.selectedCompanyId.value, shipmentId.value, documentId.value] as const,
  async ([company, shipment], previous) => {
    const current = ++ticket.value
    if (previous && previous[0] && company && previous[0] !== company) {
      clearDocumentState()
      documentsKind.value = 'empty'
      documentKind.value = 'ready'
      await router.replace(localePath('/transport-orders'))
      return
    }
    if (!company || !shipment) return
    await refresh(company, shipment)
    if (current !== ticket.value) return
  },
  { immediate: true },
)
</script>

<template>
  <section data-testid="shipment-documents">
    <CustomerPageHeader :title="t('carrier.documentsTitle')" />
    <Select
      v-if="companyOptions.length > 1"
      v-model="companyModel"
      name="company"
      data-testid="company-switch"
      :label="t('carrier.companyLabel')"
      :options="companyOptions"
    />
    <DocumentWorkspace
      :shipment-id="shipmentId"
      :documents-kind="documentsKind"
      :documents-title="titleFor(documentsKind, 'carrier.emptyDocuments')"
      :documents-detail="documentsDetail"
      :documents="documents"
      :document-kind="documentKind"
      :document-title="titleFor(documentKind, 'carrier.documentDetail')"
      :document-detail="documentDetail"
      :document="documentRecord"
      :attachments-kind="attachmentsKind"
      :attachments-title="titleFor(attachmentsKind, 'carrier.emptyAttachments')"
      :attachments="attachments"
      :download-detail="downloadDetail"
      @open="router.replace(localePath({ path: `/shipments/${shipmentId}/documents`, query: { document_id: $event } }))"
      @close="router.replace(localePath(`/shipments/${shipmentId}/documents`))"
      @download="onDownload"
      @filter="applyFilters"
    />
  </section>
</template>
