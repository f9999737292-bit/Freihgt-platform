<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
</script>

<template>
  <div class="home">
    <UiPageHeader
      :title="$t('home.title')"
      :description="$t('home.description')"
    />

    <UiCard class="home__card">
      <h3>{{ $t('home.freightRequestsCardTitle') }}</h3>
      <p>{{ $t('home.freightRequestsCardDescription') }}</p>
      <UiButton to="/freight-requests">
        {{ $t('home.freightRequestsAction') }}
      </UiButton>
    </UiCard>

    <p class="home__hint">{{ $t('home.bidRouteHint') }}</p>
  </div>
</template>

<style scoped>
.home__card h3 {
  margin: 0 0 0.5rem;
  font-size: 1.125rem;
}

.home__card p {
  margin: 0 0 1rem;
  color: #4b5563;
}

.home__hint {
  margin: 1.25rem 0 0;
  color: #6b7280;
  font-size: 0.875rem;
}
</style>
