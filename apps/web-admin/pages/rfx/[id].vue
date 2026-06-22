<script setup lang="ts">
import { toDatetimeLocal, toRFC3339, type RfxEvent } from '~/types/rfx'
import type { Company } from '~/types/company'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getRfxEvent, updateRfxEvent, isApiUnavailableError } = useRfxApi()
const { listCompanies } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()

const event = ref<RfxEvent | null>(null)
const companies = ref<Company[]>([])
const loading = ref(true)
const apiUnavailable = ref(false)
const showEditModal = ref(false)
const saving = ref(false)

const editForm = reactive({ title: '', description: '', response_deadline: '' })

const eventId = computed(() => String(route.params.id))
const companyName = computed(() => {
  if (!event.value) return ''
  return companies.value.find((c) => c.id === event.value!.owner_company_id)?.legal_name
})

function companyNameById(id: string) {
  return companies.value.find((c) => c.id === id)?.legal_name || id
}

async function loadEvent() {
  loading.value = true
  apiUnavailable.value = false
  try {
    event.value = await getRfxEvent(eventId.value)
  } catch (error) {
    event.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('rfx.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function openEdit() {
  if (!event.value) return
  editForm.title = event.value.title
  editForm.description = event.value.description || ''
  editForm.response_deadline = toDatetimeLocal(event.value.response_deadline)
  showEditModal.value = true
}

async function saveEdit() {
  if (!event.value) return
  saving.value = true
  try {
    event.value = await updateRfxEvent(event.value.id, {
      title: editForm.title,
      description: editForm.description,
      response_deadline: toRFC3339(editForm.response_deadline),
    })
    pushToast('success', t('rfx.updatedSuccess'))
    showEditModal.value = false
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    saving.value = false
  }
}

watch(eventId, loadEvent, { immediate: true })
onMounted(async () => {
  try {
    const data = await listCompanies({ limit: 100 })
    companies.value = data.items
  } catch {
    companies.value = []
  }
})
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/rfx">{{ $t('rfx.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('rfx.details') }}</span>
    </nav>

    <UiPageHeader :title="event?.title || $t('rfx.details')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/rfx')">{{ $t('common.back') }}</UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <UiEmptyState v-else-if="apiUnavailable" :title="$t('rfx.loadFailed')" />
    <UiEmptyState v-else-if="!event" :title="$t('rfx.noRfxFound')" />

    <template v-else>
      <RfxRfxActions :event="event" @updated="loadEvent" @edit="openEdit" />
      <RfxRfxDetailsCard :event="event" :company-name="companyName" @edit="openEdit" />
      <RfxRfxParticipantsTable :rfx-event-id="event.id" :company-name="companyNameById" />
      <LowCodeCustomFieldsPanel
        entity-type="RFX"
        :entity-id="event.id"
        :entity-status="event.status"
      />
    </template>

    <UiModal :open="showEditModal" :title="$t('rfx.edit')" @close="showEditModal = false">
      <div class="form-grid">
        <UiInput v-model="editForm.title" :label="$t('rfx.titleLabel')" required />
        <UiInput v-model="editForm.description" :label="$t('rfx.description')" />
        <UiInput
          v-model="editForm.response_deadline"
          type="datetime-local"
          :label="$t('rfx.responseDeadline')"
        />
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="showEditModal = false">{{ $t('common.cancel') }}</UiButton>
        <UiButton :loading="saving" @click="saveEdit">{{ $t('common.save') }}</UiButton>
      </template>
    </UiModal>
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}
</style>
