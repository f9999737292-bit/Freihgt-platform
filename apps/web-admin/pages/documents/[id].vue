<script setup lang="ts">
import type { Company } from '~/types/company'
import type { Shipment } from '~/types/shipment'
import {
  mergeDocumentVersion,
  signingSessionStorageKey,
  type DocumentDetail,
  type DocumentFile,
  type DocumentVersion,
  type SigningSession,
} from '~/types/document'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getDocument, isApiUnavailableError } = useDocumentsApi()
const { getSigningSession } = useSigningApi()
const { getCompany } = useCompanies()
const { getShipment } = useShipmentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const document = ref<DocumentDetail | null>(null)
const versions = ref<DocumentVersion[]>([])
const files = ref<DocumentFile[]>([])
const signingSession = ref<SigningSession | null>(null)
const ownerCompany = ref<Company | null>(null)
const relatedShipment = ref<Shipment | null>(null)
const loading = ref(true)
const loadingRelated = ref(false)
const apiUnavailable = ref(false)
const showVersionModal = ref(false)
const showFileModal = ref(false)
const showCancelModal = ref(false)
const showSigningSessionModal = ref(false)
const showSignatureModal = ref(false)

const documentId = computed(() => String(route.params.id))

const ownerCompanyName = computed(
  () => ownerCompany.value?.legal_name || document.value?.owner_company_id,
)

const relatedEntityLabel = computed(() => {
  if (document.value?.related_entity_type === 'SHIPMENT' && relatedShipment.value) {
    return relatedShipment.value.shipment_number
  }
  return document.value?.related_entity_id || '—'
})

function persistSigningSessionId(sessionId: string) {
  if (import.meta.client) {
    sessionStorage.setItem(signingSessionStorageKey(documentId.value), sessionId)
  }
}

function restoreSigningSessionId() {
  if (!import.meta.client) return ''
  return sessionStorage.getItem(signingSessionStorageKey(documentId.value)) || ''
}

async function loadSigningSession(sessionId: string) {
  try {
    signingSession.value = await getSigningSession(sessionId)
  } catch {
    signingSession.value = null
  }
}

async function loadRelated() {
  if (!document.value) return

  loadingRelated.value = true
  ownerCompany.value = null
  relatedShipment.value = null

  const tasks: Promise<void>[] = []

  if (document.value.owner_company_id) {
    tasks.push(
      getCompany(document.value.owner_company_id)
        .then((data) => {
          ownerCompany.value = data
        })
        .catch(() => {
          ownerCompany.value = null
        }),
    )
  }

  if (document.value.related_entity_type === 'SHIPMENT' && document.value.related_entity_id) {
    tasks.push(
      getShipment(document.value.related_entity_id)
        .then((data) => {
          relatedShipment.value = data
        })
        .catch(() => {
          relatedShipment.value = null
        }),
    )
  }

  await Promise.all(tasks)
  loadingRelated.value = false
}

function applyDocumentDetail(detail: DocumentDetail) {
  document.value = detail
  files.value = detail.files ?? []
  if (detail.latest_version) {
    versions.value = mergeDocumentVersion(versions.value, detail.latest_version)
  }
}

async function loadDocument() {
  loading.value = true
  apiUnavailable.value = false
  try {
    const detail = await getDocument(documentId.value)
    applyDocumentDetail(detail)
    await loadRelated()

    const storedSessionId = restoreSigningSessionId()
    if (storedSessionId) {
      await loadSigningSession(storedSessionId)
    } else {
      signingSession.value = null
    }
  } catch (error) {
    document.value = null
    versions.value = []
    files.value = []
    signingSession.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('documents.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function onVersionCreated() {
  await loadDocument()
}

async function onFileCreated() {
  await loadDocument()
}

async function onSigningSessionCreated(sessionId: string) {
  persistSigningSessionId(sessionId)
  await loadSigningSession(sessionId)
  await loadDocument()
}

async function onSignatureCreated() {
  await loadDocument()
  const sessionId = restoreSigningSessionId()
  if (sessionId) {
    await loadSigningSession(sessionId)
  }
}

watch(documentId, loadDocument, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/documents">{{ $t('documents.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('documents.details') }}</span>
    </nav>

    <UiPageHeader :title="document?.document_number || $t('documents.details')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/documents')">
          {{ $t('common.back') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <UiEmptyState v-else-if="apiUnavailable" :title="$t('documents.loadFailed')" />
    <UiEmptyState v-else-if="!document" :title="$t('documents.noDocumentsFound')" />

    <template v-else>
      <DocumentsDocumentActions
        :document="document"
        :signing-session="signingSession"
        @updated="loadDocument"
        @create-version="showVersionModal = true"
        @add-file="showFileModal = true"
        @cancel="showCancelModal = true"
        @create-signing-session="showSigningSessionModal = true"
        @add-signature="showSignatureModal = true"
        @archive="loadDocument"
      />

      <DocumentsDocumentDetailsCard
        :document="document"
        :owner-company-name="ownerCompanyName"
        :related-entity-label="relatedEntityLabel"
      />

      <SigningSigningSessionCard v-if="signingSession" :session="signingSession" />

      <DocumentsDocumentVersionsTable :versions="versions" :loading="loadingRelated" />
      <DocumentsDocumentFilesTable :files="files" :loading="loadingRelated" />
    </template>

    <DocumentsDocumentVersionCreateModal
      :open="showVersionModal"
      :document-id="documentId"
      @close="showVersionModal = false"
      @created="onVersionCreated"
    />

    <DocumentsDocumentFileCreateModal
      :open="showFileModal"
      :document-id="documentId"
      :versions="versions"
      @close="showFileModal = false"
      @created="onFileCreated"
    />

    <DocumentsDocumentCancelModal
      :open="showCancelModal"
      :document-id="documentId"
      @close="showCancelModal = false"
      @cancelled="loadDocument"
    />

    <SigningSigningSessionCreateModal
      :open="showSigningSessionModal"
      :document-id="documentId"
      @close="showSigningSessionModal = false"
      @created="onSigningSessionCreated"
    />

    <SigningSignatureCreateModal
      v-if="signingSession"
      :open="showSignatureModal"
      :signing-session-id="signingSession.id"
      @close="showSignatureModal = false"
      @created="onSignatureCreated"
    />
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}
</style>
