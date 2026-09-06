<script setup lang="ts">

import { RFX_QUESTIONNAIRE_API_KEY, useRfxQuestionnaireApi } from '~/composables/useRfxQuestionnaireApi'

import { buildStudioNavSteps, resolveStudioStep } from '~/components/rfx/studio/studioNav'



definePageMeta({ middleware: 'auth', layout: 'default' })



const route = useRoute()

const { t } = useI18n()

const { pushToast } = useToast()



const eventId = computed(() => String(route.params.id))

const api = useRfxQuestionnaireApi(eventId)



provide(RFX_QUESTIONNAIRE_API_KEY, api)



const selectedQuestionId = ref<string | null>(null)

const readiness = ref<Awaited<ReturnType<typeof api.validatePublish>> | null>(null)



const activeStep = computed(() => resolveStudioStep(route.query.step))

const navSteps = computed(() => buildStudioNavSteps(eventId.value, activeStep.value, t))



async function loadStudio() {

  try {

    await api.loadStudio()

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('rfx.studio.loadFailed'))

  }

}



async function handleSaveDraft() {

  try {

    await api.flushPendingPatches()

    await api.saveDraft()

    pushToast('success', t('rfx.studio.saved'))

  } catch (e) {

    if (api.autosaveStatus.value !== 'conflict') {

      pushToast('error', e instanceof Error ? e.message : t('rfx.studio.autosaveError'))

    }

  }

}



async function handleValidate() {

  try {

    await api.flushPendingPatches()

    readiness.value = await api.validatePublish()

    await navigateTo(`/rfx/${eventId.value}/studio?step=validation`)

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('rfx.studio.validateFailed'))

  }

}



async function handleReload() {

  await api.reloadAfterConflict()

  pushToast('success', t('rfx.studio.reloaded'))

}



onMounted(loadStudio)

watch(eventId, loadStudio)

</script>



<template>

  <div class="page-stack">

    <RfxStudioHeader

      :rfx-number="api.studio.value?.event.rfx_number"

      :title="api.studio.value?.event.title"

      :status="api.studio.value?.event.status"

      :autosave-status="api.autosaveStatus.value"

      :last-saved-at="api.lastSavedAt.value"

      :field-error="api.fieldError.value"

      @preview="navigateTo(`/rfx/${eventId}/studio/preview`)"

      @save="handleSaveDraft"

      @validate="handleValidate"

      @reload="handleReload"

    />



    <div v-if="api.loading.value && !api.studio.value" class="loading-block">

      {{ t('common.loading') }}

    </div>



    <RfxStudioShell v-else :steps="navSteps">

      <template v-if="activeStep === 'basics'">

        <UiCard class="basics-card">

          <h2>{{ t('rfx.studio.steps.basics') }}</h2>

          <p>{{ t('rfx.studio.basicsHint') }}</p>

          <NuxtLink :to="`/rfx/${eventId}`" class="link">{{ t('rfx.studio.editBasicsLink') }}</NuxtLink>

        </UiCard>

      </template>



      <template v-else-if="activeStep === 'questionnaire'">

        <RfxStudioRfxQuestionnaireBuilder

          v-model:selected-question-id="selectedQuestionId"

          :sections="api.studio.value?.sections ?? []"

        />

      </template>



      <template v-else-if="activeStep === 'validation'">

        <RfxStudioRfxPublishReadinessPanel :result="readiness ?? api.publishReadiness.value" />

      </template>

    </RfxStudioShell>

  </div>

</template>



<style scoped>

.loading-block {

  padding: 2rem;

  text-align: center;

  color: var(--color-text-muted);

}



.basics-card {

  padding: 1rem;

}



.basics-card h2 {

  margin: 0 0 0.5rem;

}



.link {

  color: var(--color-primary);

}

</style>

