<script setup lang="ts">

definePageMeta({ middleware: 'auth', layout: 'default' })



const route = useRoute()

const { t } = useI18n()

const { pushToast } = useToast()



const eventId = computed(() => String(route.params.id))

const api = useRfxQuestionnaireApi(eventId)



const loading = ref(true)



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



onMounted(load)

watch(eventId, load)

</script>



<template>

  <div class="page-stack">

    <nav class="breadcrumbs" :aria-label="t('common.actions')">

      <NuxtLink :to="`/rfx/${eventId}/studio?step=questionnaire`">{{ t('rfx.studio.previewBack') }}</NuxtLink>

    </nav>



    <div v-if="loading" class="loading-block">{{ t('common.loading') }}</div>



    <RfxStudioRfxCarrierPreview

      v-else-if="api.studio.value"

      :event-title="api.studio.value.event.title"

      :rfx-number="api.studio.value.event.rfx_number"

      :sections="api.studio.value.sections"

    />



    <UiEmptyState v-else :title="t('rfx.studio.loadFailed')" />

  </div>

</template>



<style scoped>

.breadcrumbs {

  margin-bottom: 1rem;

  font-size: 0.875rem;

}



.breadcrumbs a {

  color: var(--color-primary);

}



.loading-block {

  padding: 2rem;

  text-align: center;

  color: var(--color-text-muted);

}

</style>

