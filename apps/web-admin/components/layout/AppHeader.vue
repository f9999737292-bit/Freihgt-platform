<script setup lang="ts">
const { locale, locales, setLocale } = useI18n()
const { logout } = useAuth()
const { tenantId, formatTenantId } = useTenantContext()
const uiStore = useUiStore()
const config = useRuntimeConfig()

const showTenantModal = ref(false)

const gatewayLabel = computed(() => {
  if (uiStore.apiGatewayStatus === 'online') return 'online'
  if (uiStore.apiGatewayStatus === 'offline') return 'offline'
  return '…'
})

const gatewayClass = computed(() =>
  uiStore.apiGatewayStatus === 'online' ? 'header__status--online' : 'header__status--offline',
)

async function switchLocale(code: string) {
  await setLocale(code as 'ru-RU' | 'en-US' | 'zh-CN')
}
</script>

<template>
  <header class="header">
    <div class="header__left">
      <button type="button" class="header__toggle" @click="uiStore.toggleSidebar()">☰</button>
      <div>
        <div class="header__title">{{ config.public.appName }}</div>
        <div class="header__subtitle">{{ $t('app.subtitle') }}</div>
      </div>
    </div>
    <div class="header__right">
      <div class="header__meta">
        <span class="text-sm text-muted">{{ $t('tenant.current') }}:</span>
        <code :title="tenantId">{{ formatTenantId(tenantId) }}</code>
        <UiButton variant="ghost" size="sm" @click="showTenantModal = true">
          {{ $t('tenant.change') }}
        </UiButton>
      </div>
      <div class="header__locales">
        <button
          v-for="item in locales"
          :key="item.code"
          type="button"
          class="header__locale"
          :class="{ 'header__locale--active': locale === item.code }"
          @click="switchLocale(item.code)"
        >
          {{ item.name }}
        </button>
      </div>
      <div class="header__status" :class="gatewayClass">API Gateway: {{ gatewayLabel }}</div>
      <UiButton variant="ghost" size="sm" @click="logout">{{ $t('common.logout') }}</UiButton>
    </div>

    <LayoutTenantChangeModal
      :open="showTenantModal"
      :initial-tenant-id="tenantId"
      @close="showTenantModal = false"
    />
  </header>
</template>

<style scoped>
.header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0 1.5rem;
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  position: relative;
}

.header__left,
.header__right {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.header__toggle {
  border: none;
  background: transparent;
  font-size: 1.25rem;
  cursor: pointer;
  color: var(--color-text-muted);
}

.header__title {
  font-weight: 600;
}

.header__subtitle {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.header__meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.header__meta code {
  font-size: 0.75rem;
  background: var(--color-bg);
  padding: 0.125rem 0.375rem;
  border-radius: var(--radius-sm);
}

.header__locales {
  display: flex;
  gap: 0.25rem;
}

.header__locale {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  border-radius: var(--radius-sm);
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
  cursor: pointer;
}

.header__locale--active {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.header__status {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border-radius: 999px;
}

.header__status--online {
  background: #dcfce7;
  color: #166534;
}

.header__status--offline {
  background: #fee2e2;
  color: #991b1b;
}

@media (max-width: 1100px) {
  .header__meta,
  .header__status {
    display: none;
  }
}
</style>
