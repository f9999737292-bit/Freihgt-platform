<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { t } = useI18n()
const { pushToast } = useToast()

const eventId = computed(() => String(route.params.id))
const api = useRfxQuestionnaireApi(eventId)

const loading = ref(true)
const sandboxActive = ref(false)

async function load() {
  loading.value = true
  try {
    await api.loadStudio()
  } catch (e) {
    pushToast('error', e instanceof Error ? e.message : t('rfx.studio.loadFailed'))
  } finally {
    loading.value = false
  }
}

function enterSandbox() {
  sandboxActive.value = true
}

function closeSandbox() {
  sandboxActive.value = false
}

onMounted(() => {
  if (route.query.mode === 'sandbox') sandboxActive.value = true
  void load()
})

watch(eventId, load)
watch(
  () => route.query.mode,
  (mode) => {
    if (mode === 'sandbox') sandboxActive.value = true
  },
)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" :aria-label="t('common.actions')">
      <NuxtLink :to="`/rfx/${eventId}/studio?step=questionnaire`">{{ t('rfx.studio.previewBack') }}</NuxtLink>
    </nav>

    <div v-if="loading" class="loading-block">{{ t('common.loading') }}</div>

    <template v-else-if="api.studio.value">
      <RfxStudioRfxCarrierPreviewSandbox
        v-if="sandboxActive"
        :event-title="api.studio.value.event.title"
        :rfx-number="api.studio.value.event.rfx_number"
        :sections="api.studio.value.sections"
        :rules="api.studio.value.rules"
        @close="closeSandbox"
      />

      <template v-else>
        <RfxStudioRfxCarrierPreview
          :event-title="api.studio.value.event.title"
          :rfx-number="api.studio.value.event.rfx_number"
          :sections="api.studio.value.sections"
        />
        <div class="preview-actions">
          <UiButton variant="primary" data-testid="enter-carrier-preview-sandbox" @click="enterSandbox">
            {{ t('rfx.studio.previewSandbox.enter') }}
          </UiButton>
        </div>
      </template>
    </template>

    <UiEmptyState v-else :title="t('rfx.studio.loadFailed')" />
  </div>
</template>

<style scoped>
.breadcrumbs { margin-bottom: 1rem; font-size: 0.875rem; }
.breadcrumbs a { color: var(--color-primary); }
.loading-block { padding: 2rem; text-align: center; color: var(--color-text-muted); }
.preview-actions { margin-top: 1.5rem; }
</style>
