# @freight-platform/ui

Shared Vue 3 components for freight-platform frontend apps.

## Components

| Component | Purpose |
|-----------|---------|
| `AppShell` | Base page layout with header and main area |
| `LocaleSwitcher` | Language selector (requires `@nuxtjs/i18n`) |

## Usage

```vue
<script setup lang="ts">
import { AppShell, LocaleSwitcher } from '@freight-platform/ui'
</script>

<template>
  <AppShell title="Freight Platform">
    <template #actions>
      <LocaleSwitcher />
    </template>
    <p>{{ $t('common.welcome') }}</p>
  </AppShell>
</template>
```
