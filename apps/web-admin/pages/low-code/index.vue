<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const { backendOnline, backendStatus, checkBackendStatus } = useBackendStatus()
const { listFormTemplates, isApiUnavailableError, isLowCodeServiceError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { t } = useI18n()
const config = useRuntimeConfig()

const loading = ref(true)
const templatesCount = ref<number | null>(null)
const lowCodeStatus = ref<'online' | 'offline' | 'unknown'>('unknown')
const lowCodeMessage = ref('')

const { canAccessLowCodeAdmin } = useLowCodePermissions()

const navLinks = computed(() => {
  const links = [
    { to: '/low-code/form-templates', labelKey: 'lowCode.formTemplates', descKey: 'lowCode.formTemplatesDesc' },
    { to: '/low-code/custom-field-values', labelKey: 'lowCode.customFieldValues', descKey: 'lowCode.customFieldValuesDesc' },
    { to: '/low-code/audit', labelKey: 'lowCode.auditLog', descKey: 'lowCode.auditLogDesc' },
  ]
  if (canAccessLowCodeAdmin()) {
    links.splice(1, 0, {
      to: '/low-code/admin/form-templates',
      labelKey: 'lowCode.formTemplateAdmin',
      descKey: 'lowCode.formTemplateAdminDesc',
    })
  }
  return links
})

async function probeLowCode() {
  loading.value = true
  lowCodeMessage.value = ''
  templatesCount.value = null
  lowCodeStatus.value = 'unknown'

  if (!hasTenant.value) {
    loading.value = false
    return
  }

  try {
    const data = await listFormTemplates()
    templatesCount.value = data.items.length
    lowCodeStatus.value = 'online'
  } catch (error) {
    templatesCount.value = null
    if (isApiUnavailableError(error) || isLowCodeServiceError(error)) {
      lowCodeStatus.value = 'offline'
      lowCodeMessage.value = t('lowCode.serviceUnavailable')
    } else {
      lowCodeStatus.value = 'unknown'
      lowCodeMessage.value = error instanceof Error ? error.message : t('common.error')
    }
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await checkBackendStatus()
  await probeLowCode()
}

onMounted(refreshAll)
</script>

<template>
  <div class="page-stack low-code-hub">
    <header class="low-code-hub__hero">
      <div>
        <h1 class="low-code-hub__title">{{ $t('lowCode.title') }}</h1>
        <p class="low-code-hub__subtitle">{{ $t('lowCode.subtitle') }}</p>
      </div>
      <UiButton size="sm" variant="secondary" :disabled="loading" @click="refreshAll">
        {{ loading ? $t('common.loading') : $t('common.refresh') }}
      </UiButton>
    </header>

    <div v-if="canAccessLowCodeAdmin()" class="low-code-hub__notice low-code-hub__notice--info">
      <strong>{{ $t('lowCode.formTemplateAdmin') }}</strong>
      <p>{{ $t('lowCode.formTemplateAdminHint') }}</p>
    </div>

    <div class="low-code-hub__notice low-code-hub__notice--warn">
      <strong>{{ $t('lowCode.readOnlyPreview') }}</strong>
      <p>{{ $t('lowCode.readOnlyHint') }}</p>
    </div>

    <div class="health-grid">
      <UiCard>
        <template #header>{{ $t('backendStatus.title') }}</template>
        <div class="status-row">
          <UiBadge
            :status="backendOnline ? 'online' : backendStatus === 'checking' ? 'checking' : 'offline'"
            :tone="backendOnline ? 'success' : 'danger'"
          />
          <span>{{ backendOnline ? $t('backendStatus.online') : $t('backendStatus.offline') }}</span>
        </div>
        <p class="text-muted">{{ config.public.apiBaseUrl }}</p>
      </UiCard>

      <UiCard>
        <template #header>{{ $t('lowCode.serviceStatus') }}</template>
        <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
        <template v-else>
          <div class="status-row">
            <UiBadge
              :status="lowCodeStatus"
              :tone="lowCodeStatus === 'online' ? 'success' : lowCodeStatus === 'offline' ? 'danger' : 'neutral'"
            />
            <span>
              {{
                lowCodeStatus === 'online'
                  ? $t('lowCode.serviceOnline')
                  : lowCodeStatus === 'offline'
                    ? $t('lowCode.serviceOffline')
                    : $t('common.error')
              }}
            </span>
          </div>
          <p v-if="templatesCount !== null" class="text-muted">
            {{ $t('lowCode.publishedTemplatesCount', { count: templatesCount }) }}
          </p>
          <p v-if="lowCodeMessage" class="low-code-hub__error">{{ lowCodeMessage }}</p>
        </template>
      </UiCard>
    </div>

    <UiCard>
      <template #header>{{ $t('lowCode.pages') }}</template>
      <div class="link-grid">
        <NuxtLink v-for="link in navLinks" :key="link.to" :to="link.to" class="link-card">
          <strong>{{ $t(link.labelKey) }}</strong>
          <span>{{ $t(link.descKey) }}</span>
        </NuxtLink>
      </div>
    </UiCard>
  </div>
</template>

<style scoped>
.low-code-hub__hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.low-code-hub__title {
  margin: 0;
  font-size: 1.5rem;
}

.low-code-hub__subtitle {
  margin: 0.375rem 0 0;
  color: var(--color-text-muted);
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

.low-code-hub__notice--info {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #1e3a8a;
}

.low-code-hub__notice--warn {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.health-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 1rem;
}

.status-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}

.low-code-hub__error {
  margin: 0.5rem 0 0;
  font-size: 0.875rem;
  color: #b45309;
}

.link-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 0.75rem;
}

.link-card {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  text-decoration: none;
  color: inherit;
  transition: border-color 0.15s, background 0.15s;
}

.link-card:hover {
  border-color: var(--color-primary);
  background: var(--color-surface-muted, #f8fafc);
  text-decoration: none;
}

.link-card span {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}
</style>
