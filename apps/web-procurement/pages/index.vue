<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
</script>

<template>
  <div class="home">
    <h2>{{ $t('home.title') }}</h2>
    <p>{{ $t('home.description') }}</p>
    <p class="home__hint">{{ $t('home.bidRouteHint') }}</p>
  </div>
</template>

<style scoped>
.home h2 {
  margin: 0 0 0.5rem;
}

.home p {
  margin: 0 0 0.75rem;
}

.home__hint {
  color: #6b7280;
  font-size: 0.875rem;
}
</style>
