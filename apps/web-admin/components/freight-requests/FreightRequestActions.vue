<script setup lang="ts">
import type { FreightRequest } from '~/types/rfx'

const props = defineProps<{ request: FreightRequest }>()
const emit = defineEmits<{ updated: [] }>()

const { publishFreightRequest } = useFreightRequestsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const publishing = ref(false)

async function publish() {
  publishing.value = true
  try {
    await publishFreightRequest(props.request.id)
    pushToast('success', t('freightRequests.publishedSuccess'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    publishing.value = false
  }
}
</script>

<template>
  <div v-if="request.status === 'DRAFT'" class="actions-row">
    <UiButton :loading="publishing" @click="publish">
      {{ $t('freightRequests.publish') }}
    </UiButton>
  </div>
</template>

<style scoped>
.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
</style>
