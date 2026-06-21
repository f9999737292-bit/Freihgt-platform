<script setup lang="ts">
import { formatCompanyDate, type Company } from '~/types/company'

defineProps<{ company: Company }>()
</script>

<template>
  <UiCard>
    <template #header>
      <div class="details-card__header">
        <div>
          <h2>{{ company.legal_name }}</h2>
          <p v-if="company.short_name" class="text-muted">{{ company.short_name }}</p>
        </div>
        <CompaniesCompanyTypeBadge :type="company.company_type" />
      </div>
    </template>

    <div class="details-grid">
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.companyType') }}</span>
        <CompaniesCompanyTypeBadge :type="company.company_type" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('common.status') }}</span>
        <CompaniesCompanyStatusBadge :status="company.status" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.taxId') }}</span>
        <span>{{ company.tax_id || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.registrationNumber') }}</span>
        <span>{{ company.registration_number || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.country') }}</span>
        <span>{{ company.country_code }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.preferredLanguage') }}</span>
        <span>{{ company.preferred_locale }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.createdAt') }}</span>
        <span>{{ formatCompanyDate(company.created_at) }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('companies.updatedAt') }}</span>
        <span>{{ formatCompanyDate(company.updated_at) }}</span>
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
