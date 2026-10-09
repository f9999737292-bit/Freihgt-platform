<script setup lang="ts">
import { EmptyState, ErrorState, LoadingState } from '@freight-platform/ui'
import type { PortalViewKind } from '../domain/viewState'

defineProps<{
  kind: PortalViewKind
  title: string
  description?: string
}>()
</script>

<template>
  <LoadingState v-if="kind === 'loading'" :label="title" />
  <EmptyState v-else-if="kind === 'empty'" :title="title" :description="description" />
  <ErrorState v-else :title="title" :description="description" />
</template>
