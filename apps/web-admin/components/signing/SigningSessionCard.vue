<script setup lang="ts">
import { formatDocumentDate, type SigningSession } from '~/types/document'

defineProps<{ session: SigningSession }>()
</script>

<template>
  <UiCard>
    <template #header>
      <div class="session-header">
        <h3 class="card-title">{{ $t('documents.signingSession') }}</h3>
        <SigningSignatureStatusBadge :status="session.status" />
      </div>
    </template>

    <div class="details-grid">
      <div>
        <span class="text-muted">{{ $t('documents.requiredSignersCount') }}</span>
        <div>{{ session.required_signers_count }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('documents.completedSignersCount') }}</span>
        <div>{{ session.completed_signers_count }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('freightRequests.createdAt') }}</span>
        <div>{{ formatDocumentDate(session.created_at) }}</div>
      </div>
      <div>
        <span class="text-muted">{{ $t('freightRequests.validUntil') }}</span>
        <div>{{ formatDocumentDate(session.expires_at) }}</div>
      </div>
    </div>
  </UiCard>
</template>

<style scoped>
.session-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.details-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}
</style>
