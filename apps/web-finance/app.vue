<script setup lang="ts">
import { AppShell, LocaleSwitcher } from '@freight-platform/ui'
</script>

<template>
  <AppShell title="Freight Platform Finance">
    <template #actions>
      <LocaleSwitcher />
    </template>
    <NuxtPage />
  </AppShell>
</template>
