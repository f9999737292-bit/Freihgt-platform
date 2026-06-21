<script setup lang="ts">
import { formatRfxDate, type RfxEvent } from '~/types/rfx'

defineProps<{ event: RfxEvent; companyName?: string }>()
const emit = defineEmits<{ edit: [] }>()
</script>

<template>
  <UiCard>
    <template #header>
      <div class="details-card__header">
        <div>
          <h2>{{ event.title }}</h2>
          <p class="text-muted">{{ event.rfx_number }}</p>
        </div>
        <RfxRfxStatusBadge :status="event.status" />
      </div>
    </template>

    <div class="details-grid">
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.rfxType') }}</span>
        <RfxRfxTypeBadge :type="event.rfx_type" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.category') }}</span>
        <span>{{ event.category }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.owner') }}</span>
        <span>{{ companyName || event.owner_company_id }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.currency') }}</span>
        <span>{{ event.currency_code || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.validFrom') }}</span>
        <span>{{ formatRfxDate(event.valid_from) }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.validTo') }}</span>
        <span>{{ formatRfxDate(event.valid_to) }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('rfx.responseDeadline') }}</span>
        <span>{{ formatRfxDate(event.response_deadline) }}</span>
      </div>
      <div class="details-item details-item--full">
        <span class="details-item__label">{{ $t('rfx.description') }}</span>
        <span>{{ event.description || '—' }}</span>
      </div>
    </div>

    <template v-if="event.status === 'DRAFT'" #footer>
      <UiButton variant="secondary" @click="emit('edit')">{{ $t('rfx.edit') }}</UiButton>
    </template>
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

.details-item--full {
  grid-column: 1 / -1;
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
