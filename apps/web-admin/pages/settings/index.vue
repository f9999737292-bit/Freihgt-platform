<script setup lang="ts">
definePageMeta({ middleware: 'auth', layout: 'default' })

const config = useRuntimeConfig()
const authStore = useAuthStore()
const { tenantId, currentCompanyId, clearTenant, formatTenantId } = useTenantContext()
const { locale } = useI18n()
const { pushToast } = useToast()
const { clearSession } = useAuth()
const router = useRouter()

const showTenantModal = ref(false)

function handleClearSession() {
  clearSession()
  pushToast('info', 'Session cleared')
  router.push('/login')
}

function handleClearTenant() {
  clearTenant()
  router.go(0)
}
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="$t('settings.title')" />

    <UiCard>
      <div class="form-grid">
        <div>
          <span class="text-muted">{{ $t('settings.apiBaseUrl') }}</span>
          <div>{{ config.public.apiBaseUrl }}</div>
        </div>
        <div>
          <span class="text-muted">{{ $t('tenant.current') }}</span>
          <div><code :title="tenantId">{{ tenantId || '—' }}</code></div>
        </div>
        <div>
          <span class="text-muted">{{ $t('settings.currentUser') }}</span>
          <div>{{ authStore.user?.full_name }} ({{ authStore.user?.email }})</div>
        </div>
        <div>
          <span class="text-muted">{{ $t('settings.currentCompany') }}</span>
          <div><code>{{ currentCompanyId || '—' }}</code></div>
        </div>
        <div>
          <span class="text-muted">{{ $t('common.language') }}</span>
          <div>{{ locale }}</div>
        </div>
        <div>
          <span class="text-muted">{{ $t('settings.mockMode') }}</span>
          <div>{{ config.public.mockAuth ? $t('settings.enabled') : $t('settings.disabled') }}</div>
        </div>
        <div>
          <span class="text-muted">{{ $t('tenant.tenantId') }}</span>
          <div>{{ formatTenantId(tenantId) }}</div>
        </div>
      </div>

      <template #footer>
        <div class="settings-actions">
          <UiButton variant="secondary" @click="showTenantModal = true">
            {{ $t('tenant.change') }}
          </UiButton>
          <UiButton variant="secondary" @click="handleClearTenant">
            {{ $t('tenant.clear') }}
          </UiButton>
          <UiButton variant="danger" @click="handleClearSession">
            {{ $t('tenant.clearSession') }}
          </UiButton>
        </div>
      </template>
    </UiCard>

    <LayoutTenantChangeModal
      :open="showTenantModal"
      :initial-tenant-id="tenantId"
      @close="showTenantModal = false"
    />
  </div>
</template>

<style scoped>
.settings-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
</style>
