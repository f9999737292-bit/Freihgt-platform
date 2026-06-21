<script setup lang="ts">
import { formatUserDate, type User } from '~/types/user'

defineProps<{ user: User }>()
const emit = defineEmits<{ edit: [] }>()
</script>

<template>
  <UiCard>
    <template #header>
      <div class="details-card__header">
        <div>
          <h2>{{ user.full_name }}</h2>
          <p class="text-muted">{{ user.email }}</p>
        </div>
        <UsersUserStatusBadge :status="user.status" />
      </div>
    </template>

    <div class="details-grid">
      <div class="details-item">
        <span class="details-item__label">{{ $t('users.email') }}</span>
        <span>{{ user.email }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('users.phone') }}</span>
        <span>{{ user.phone || '—' }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('users.preferredLanguage') }}</span>
        <span>{{ user.preferred_locale }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('common.status') }}</span>
        <UsersUserStatusBadge :status="user.status" />
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('users.createdAt') }}</span>
        <span>{{ formatUserDate(user.created_at) }}</span>
      </div>
      <div class="details-item">
        <span class="details-item__label">{{ $t('users.updatedAt') }}</span>
        <span>{{ formatUserDate(user.updated_at) }}</span>
      </div>
    </div>

    <template #footer>
      <UiButton variant="secondary" @click="emit('edit')">{{ $t('users.edit') }}</UiButton>
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
