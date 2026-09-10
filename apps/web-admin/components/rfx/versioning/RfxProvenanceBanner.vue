<script setup lang="ts">
import type { RfxCloneEventFromTemplateResponse } from '~/types/rfx-version-lifecycle'
import type { RfxEventProvenance } from '~/types/rfx'

const props = defineProps<{
  provenance: RfxEventProvenance | RfxCloneEventFromTemplateResponse | null
  templateName?: string | null
  canLinkTemplate?: boolean
  templateId?: string | null
}>()

const { t } = useI18n()

const showSupersededWarning = computed(() => {
  if (!props.provenance) return false
  if ('source_version_warning' in props.provenance) return props.provenance.source_version_warning
  return props.provenance.source_version_status === 'SUPERSEDED'
})
</script>

<template>
  <aside v-if="provenance" class="provenance" aria-labelledby="provenance-title">
    <h2 id="provenance-title">{{ $t('rfx.provenance.title') }}</h2>
    <dl class="provenance__list">
      <div v-if="templateName">
        <dt>{{ $t('rfx.provenance.templateName') }}</dt>
        <dd>
          <NuxtLink v-if="canLinkTemplate && templateId" :to="`/rfx/templates/${templateId}`">
            {{ templateName }}
          </NuxtLink>
          <span v-else>{{ templateName }}</span>
        </dd>
      </div>
      <div v-if="templateId">
        <dt>{{ $t('rfx.provenance.templateId') }}</dt>
        <dd><code>{{ templateId }}</code></dd>
      </div>
      <div>
        <dt>{{ $t('rfx.provenance.sourceVersion') }}</dt>
        <dd>v{{ provenance.source_version_number }} ({{ $t(`rfx.templates.versionStatus.${provenance.source_version_status}`) }})</dd>
      </div>
      <div>
        <dt>{{ $t('rfx.provenance.sourceVersionId') }}</dt>
        <dd><code>{{ provenance.source_template_version_id }}</code></dd>
      </div>
    </dl>
    <p v-if="showSupersededWarning" class="provenance__warning">
      {{ $t('rfx.templates.clone.supersededWarning') }}
    </p>
  </aside>
</template>

<style scoped>
.provenance {
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg);
}
.provenance h2 { margin: 0 0 0.75rem; font-size: 1rem; }
.provenance__list { margin: 0; display: grid; gap: 0.5rem; }
.provenance__list div { display: grid; grid-template-columns: 10rem 1fr; gap: 0.5rem; font-size: 0.875rem; }
.provenance__list dt { color: var(--color-text-muted); margin: 0; }
.provenance__list dd { margin: 0; }
.provenance__warning { color: #b45309; margin: 0.75rem 0 0; font-size: 0.875rem; }
</style>
