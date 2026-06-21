<script setup lang="ts">
import {
  DOCUMENT_STATUSES,
  DOCUMENT_TYPES,
  RELATED_ENTITY_TYPES,
  formatDocumentDate,
  type DocumentRecord,
} from '~/types/document'
import type { Company } from '~/types/company'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { listDocuments, isApiUnavailableError } = useDocumentsApi()
const { listCompanies } = useCompanies()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<DocumentRecord[]>([])
const total = ref(0)
const companies = ref<Company[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showCreateModal = ref(false)
const initialShipmentId = ref('')
const initialDocumentType = ref('')

const filters = reactive({
  search: '',
  document_type: '',
  document_status: '',
  related_entity_type: '',
  related_entity_id: '',
})

const pagination = reactive({ limit: 20, offset: 0 })

const typeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...DOCUMENT_TYPES.map((v) => ({ label: v, value: v })),
])
const statusOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...DOCUMENT_STATUSES.map((v) => ({ label: v, value: v })),
])
const entityTypeOptions = computed(() => [
  { label: t('common.all'), value: '' },
  ...RELATED_ENTITY_TYPES.map((v) => ({ label: v, value: v })),
])

const companyName = (id?: string) =>
  id ? companies.value.find((c) => c.id === id)?.legal_name || id.slice(0, 8) + '...' : '—'

const hasItems = computed(() => items.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)

let searchTimer: ReturnType<typeof setTimeout> | undefined

async function load() {
  if (!hasTenant.value) {
    loading.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  try {
    const data = await listDocuments({
      search: filters.search,
      document_type: filters.document_type,
      document_status: filters.document_status,
      related_entity_type: filters.related_entity_type,
      related_entity_id: filters.related_entity_id,
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items ?? []
    total.value = data.total ?? items.value.length
  } catch (error) {
    items.value = []
    total.value = 0
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('documents.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function onFiltersChange() {
  pagination.offset = 0
  load()
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(onFiltersChange, 350)
}

function openCreateModal(shipmentId = '', documentType = '') {
  initialShipmentId.value = shipmentId
  initialDocumentType.value = documentType
  showCreateModal.value = true
}

onMounted(async () => {
  try {
    companies.value = (await listCompanies({ limit: 100 })).items
  } catch {
    companies.value = []
  }
  await load()

  const shipmentId = String(route.query.shipment_id || '')
  const documentType = String(route.query.document_type || '')
  if (shipmentId || documentType) {
    openCreateModal(shipmentId, documentType || 'POD')
  }
})
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('documents.title')">
      <template #actions>
        <UiButton @click="openCreateModal()">{{ $t('documents.createDocument') }}</UiButton>
      </template>
    </UiPageHeader>

    <UiCard>
      <div class="filters-row">
        <UiInput v-model="filters.search" :label="$t('common.search')" @update:model-value="onSearchInput" />
        <UiSelect
          v-model="filters.document_type"
          :label="$t('documents.documentType')"
          :options="typeOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.document_status"
          :label="$t('documents.documentStatus')"
          :options="statusOptions"
          @update:model-value="onFiltersChange"
        />
        <UiSelect
          v-model="filters.related_entity_type"
          :label="$t('documents.relatedEntity')"
          :options="entityTypeOptions"
          @update:model-value="onFiltersChange"
        />
        <UiInput
          v-model="filters.related_entity_id"
          :label="$t('documents.relatedEntityId')"
          @update:model-value="onSearchInput"
        />
      </div>
    </UiCard>

    <UiEmptyState v-if="loadFailed && !loading" :title="$t('documents.loadFailed')" />
    <UiEmptyState v-else-if="!loading && !hasItems" :title="$t('documents.noDocumentsFound')" />

    <UiCard v-else>
      <UiTable
        :columns="[
          $t('documents.documentNumber'),
          $t('documents.documentType'),
          $t('documents.documentStatus'),
          $t('documents.ownerCompany'),
          $t('documents.relatedEntity'),
          $t('documents.legalLanguage'),
          $t('freightRequests.createdAt'),
          $t('common.actions'),
        ]"
        :loading="loading"
      >
        <tr v-for="item in items" :key="item.id">
          <td>
            <NuxtLink :to="`/documents/${item.id}`" class="link">
              {{ item.document_number }}
            </NuxtLink>
          </td>
          <td><DocumentsDocumentTypeBadge :type="item.document_type" /></td>
          <td><DocumentsDocumentStatusBadge :status="item.document_status" /></td>
          <td>{{ companyName(item.owner_company_id) }}</td>
          <td>
            <span v-if="item.related_entity_type">{{ item.related_entity_type }}</span>
            <span v-if="item.related_entity_id" class="text-muted"> / {{ item.related_entity_id.slice(0, 8) }}…</span>
            <span v-if="!item.related_entity_type && !item.related_entity_id">—</span>
          </td>
          <td>{{ item.legal_language || '—' }}</td>
          <td>{{ formatDocumentDate(item.created_at) }}</td>
          <td><NuxtLink :to="`/documents/${item.id}`">{{ $t('common.details') }}</NuxtLink></td>
        </tr>
      </UiTable>

      <div class="pagination">
        <span class="text-sm text-muted">{{ total }}</span>
        <div class="pagination__actions">
          <UiButton
            size="sm"
            variant="secondary"
            :disabled="!canGoPrev"
            @click="pagination.offset -= pagination.limit; load()"
          >
            ←
          </UiButton>
          <UiButton
            size="sm"
            variant="secondary"
            :disabled="!canGoNext"
            @click="pagination.offset += pagination.limit; load()"
          >
            →
          </UiButton>
        </div>
      </div>
    </UiCard>

    <DocumentsDocumentCreateModal
      :open="showCreateModal"
      :initial-shipment-id="initialShipmentId"
      :initial-document-type="initialDocumentType"
      @close="showCreateModal = false"
      @created="load"
    />
  </div>
</template>

<style scoped>
.link {
  font-weight: 500;
  text-decoration: none;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--color-border);
}

.pagination__actions {
  display: flex;
  gap: 0.5rem;
}
</style>
