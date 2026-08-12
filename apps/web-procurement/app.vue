<script setup lang="ts">
const { isAuthenticated, logout, restoreSession } = useAuth()

onMounted(() => {
  restoreSession()
})
</script>

<template>
  <AppShell title="Freight Platform Procurement">
    <template #actions>
      <LocaleSwitcher />
      <button v-if="isAuthenticated" type="button" class="logout-btn" @click="logout">
        {{ $t('common.logout') }}
      </button>
      <NuxtLink v-else to="/login" class="login-link">{{ $t('common.login') }}</NuxtLink>
    </template>
    <NuxtPage />
  </AppShell>
</template>

<style scoped>
.logout-btn,
.login-link {
  font: inherit;
  font-size: 0.875rem;
}

.logout-btn {
  padding: 0.375rem 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #fff;
  cursor: pointer;
}

.login-link {
  color: #2563eb;
  text-decoration: none;
}
</style>
