<script setup lang="ts">
import { formatRfxDate, type RfxParticipant } from '~/types/rfx'

const props = defineProps<{ rfxEventId: string; companyName?: (id: string) => string }>()

const { listRfxParticipants, isApiUnavailableError } = useRfxApi()
const { pushToast } = useToast()
const { t } = useI18n()

const items = ref<RfxParticipant[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showAddModal = ref(false)

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    items.value = await listRfxParticipants(props.rfxEventId)
  } catch (error) {
    items.value = []
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

watch(() => props.rfxEventId, load, { immediate: true })
</script>

<template>
  <UiCard>
    <template #header>
      <div class="section-header">
        <h3>{{ $t('rfx.participants') }}</h3>
        <UiButton size="sm" @click="showAddModal = true">{{ $t('rfx.addParticipant') }}</UiButton>
      </div>
    </template>

    <UiTable
      v-if="items.length || loading"
      :columns="[
        $t('rfx.company'),
        $t('rfx.participantType'),
        $t('common.status'),
        $t('rfx.invitedAt'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.id">
        <td>{{ companyName?.(item.company_id) || item.company_id }}</td>
        <td>{{ item.participant_type }}</td>
        <td><RfxRfxStatusBadge :status="item.status" /></td>
        <td>{{ formatRfxDate(item.invited_at) }}</td>
        <td>—</td>
      </tr>
    </UiTable>

    <UiEmptyState v-else-if="loadFailed" :title="$t('rfx.loadFailed')" />
    <UiEmptyState v-else :title="$t('rfx.noParticipants')" />
  </UiCard>

  <RfxRfxParticipantCreateModal
    :open="showAddModal"
    :rfx-event-id="rfxEventId"
    @close="showAddModal = false"
    @added="load"
  />
</template>

<style scoped>
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.section-header h3 {
  margin: 0;
  font-size: 1rem;
}
</style>
