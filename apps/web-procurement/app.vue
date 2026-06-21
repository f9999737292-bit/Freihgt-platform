<script setup lang="ts">
import { AppShell, LocaleSwitcher } from '@freight-platform/ui'
</script>

<template>
  <AppShell title="Freight Platform Procurement">
    <template #actions>
      <LocaleSwitcher />
    </template>
    <NuxtPage />
  </AppShell>
</template>
