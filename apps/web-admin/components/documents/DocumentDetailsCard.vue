<script setup lang="ts">
import { formatDocumentDate, type DocumentDetail } from '~/types/document'

defineProps<{
  document: DocumentDetail
  ownerCompanyName?: string
  relatedEntityLabel?: string
}>()
</script>

<template>
  <UiCard>
    <template #header>
      <div class="details-card__header">
        <div>
          <h2>{{ document.document_number }}</h2>
          <p class="text-muted">{{ $t('documents.details') }}</p>
        </div>
        <DocumentsDocumentStatusBadge :status="document.document_status" />
      </div>
    </template>

    <div class="details-grid">
      <div class="details-item">
        <span class="details-item__label">{{ $t('documents.documentType') }}</span>
        <DocumentsDocumentTypeBadge :type="document.document_type" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('documents.documentStatus') }}</span>
        <DocumentsDocumentStatusBadge :status="document.document_status" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('documents.ownerCompany') }}</span>
        <span>{{ ownerCompanyName || document.owner_company_id }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('documents.relatedEntity') }}</span>
        <span>{{ document.related_entity_type || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('documents.relatedEntityId') }}</span>
        <span>{{ relatedEntityLabel || document.related_entity_id || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('documents.legalLanguage') }}</span>
        <span>{{ document.legal_language }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('freightRequests.createdAt') }}</span>
        <span>{{ formatDocumentDate(document.created_at) }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.updatedAt') }}</span>
        <span>{{ formatDocumentDate(document.updated_at) }}</span>
      </div>
    </div>
  </UiCard>
</template>

<style scoped>
.details-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.details-card__header h2 {
  margin: 0;
  font-size: 1.125rem;
}

.details-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem 1.5rem;
}

.details-item {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.details-item__label {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

@media (max-width: 768px) {
  .details-grid {
    grid-template-columns: 1fr;
  }
}
</style>
