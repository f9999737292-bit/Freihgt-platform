<script setup lang="ts">
import { PARTICIPANT_TYPES, type RfxEvent } from '~/types/rfx'

const props = defineProps<{ event: RfxEvent }>()
const emit = defineEmits<{ updated: []; edit: [] }>()

const { publishRfxEvent, cancelRfxEvent } = useRfxApi()
const { pushToast } = useToast()
const { t } = useI18n()

const publishing = ref(false)
const cancelling = ref(false)

async function publish() {
  publishing.value = true
  try {
    await publishRfxEvent(props.event.id)
    pushToast('success', t('rfx.publishedSuccess'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    publishing.value = false
  }
}

async function cancel() {
  if (!confirm(t('rfx.cancelConfirm'))) return
  cancelling.value = true
  try {
    await cancelRfxEvent(props.event.id)
    pushToast('success', t('rfx.cancelledSuccess'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    cancelling.value = false
  }
}
</script>

<template>
  <div v-if="event.status === 'DRAFT'" class="actions-row">
    <UiButton :loading="publishing" @click="publish">{{ $t('rfx.publish') }}</UiButton>
    <UiButton variant="secondary" @click="emit('edit')">{{ $t('rfx.edit') }}</UiButton>
    <UiButton variant="danger" :loading="cancelling" @click="cancel">{{ $t('rfx.cancel') }}</UiButton>
  </div>
</template>

<style scoped>
.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
</style>
