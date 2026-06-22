<script setup lang="ts">
import { DEMO_ENTITY_REFS, LOW_CODE_ENTITY_TYPES, type LowCodeEntityType } from '~/types/lowCode'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { resolveDemoEntityId } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const form = reactive({
  entity_type: 'TRANSPORT_ORDER' as LowCodeEntityType,
  entity_id: '',
})

const loadedEntity = reactive({
  entity_type: '' as LowCodeEntityType | '',
  entity_id: '',
})

const loaded = ref(false)
const resolvingDemo = ref(false)
const panelKey = ref(0)

const entityTypeOptions = computed(() =>
  LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),
)

const demoRefHint = computed(() => DEMO_ENTITY_REFS[form.entity_type])

function markLoaded() {
  loadedEntity.entity_type = form.entity_type
  loadedEntity.entity_id = form.entity_id.trim()
  loaded.value = true
  panelKey.value += 1
}

async function loadValues() {
  if (!hasTenant.value) return
  if (!form.entity_id.trim()) {
    pushToast('error', t('lowCode.entityIdRequired'))
    return
  }
  markLoaded()
}

async function useDemoEntity() {
  if (!hasTenant.value) return
  resolvingDemo.value = true
  try {
    const id = await resolveDemoEntityId(form.entity_type)
    if (!id) {
      pushToast('error', t('lowCode.demoEntityNotFound', { ref: demoRefHint.value }))
      return
    }
    form.entity_id = id
    markLoaded()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('lowCode.demoResolveFailed'))
  } finally {
    resolvingDemo.value = false
  }
}

function reloadPanel() {
  if (!loadedEntity.entity_id) return
  panelKey.value += 1
}
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.customFieldValues') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.customFieldValues')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/low-code')">{{ $t('common.back') }}</UiButton>
      </template>
    </UiPageHeader>

    <div class="low-code-hub__notice low-code-hub__notice--warn">
      <strong>{{ $t('lowCode.editCustomFields') }}</strong>
      <p>{{ $t('lowCode.coreEntityNotChanged') }}</p>
    </div>

    <UiCard>
      <div class="lookup-form">
        <UiSelect
          v-model="form.entity_type"
          :label="$t('lowCode.entityType')"
          :options="entityTypeOptions"
        />
        <UiInput
          v-model="form.entity_id"
          :label="$t('lowCode.entityId')"
          :placeholder="$t('lowCode.entityIdPlaceholder')"
        />
        <div class="lookup-form__actions">
          <UiButton @click="loadValues">{{ $t('lowCode.loadValues') }}</UiButton>
          <UiButton variant="secondary" :loading="resolvingDemo" @click="useDemoEntity">
            {{ $t('lowCode.useDemoEntity') }}
          </UiButton>
          <UiButton
            v-if="loaded && loadedEntity.entity_id"
            variant="secondary"
            @click="reloadPanel"
          >
            {{ $t('lowCode.reloadValues') }}
          </UiButton>
        </div>
      </div>
      <p class="demo-hint">
        {{ $t('lowCode.demoHint', { entityType: form.entity_type, demoRef: demoRefHint }) }}
      </p>
    </UiCard>

    <LowCodeLowCodeCustomFieldsPanel
      v-if="loaded && loadedEntity.entity_id"
      :key="panelKey"
      :entity-type="loadedEntity.entity_type"
      :entity-id="loadedEntity.entity_id"
      editable
    />
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

.low-code-hub__notice {
  padding: 1rem 1.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid transparent;
}

.low-code-hub__notice p {
  margin: 0.375rem 0 0;
  font-size: 0.875rem;
}

.low-code-hub__notice--warn {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.lookup-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
  align-items: end;
}

.lookup-form__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.demo-hint {
  margin: 1rem 0 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}
</style>
