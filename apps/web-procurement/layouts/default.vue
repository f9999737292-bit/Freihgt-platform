<script setup lang="ts">
import { AppShell, LocaleSwitcher } from '@freight-platform/ui'

const { isAuthenticated, logout } = useAuth()
const { toasts } = useToast()
const { t } = useI18n()
const { enabled: contractRateEnabled } = useContractRateFeature()
const { canReadContracts } = usePermissions()

const showContractsNav = computed(() => contractRateEnabled.value && canReadContracts())
</script>

<template>
  <AppShell :title="t('nav.appTitle')">
    <template #actions>
      <LocaleSwitcher />
      <button v-if="isAuthenticated" type="button" class="shell-action" @click="logout">
        {{ t('common.logout') }}
      </button>
      <NuxtLink v-else to="/login" class="shell-action shell-action--link">
        {{ t('common.login') }}
      </NuxtLink>
    </template>

    <nav class="procurement-nav" aria-label="Main navigation">
      <NuxtLink to="/tenders" class="procurement-nav__link">{{ t('nav.tenders') }}</NuxtLink>
      <NuxtLink to="/transport-orders" class="procurement-nav__link">{{ t('nav.buyerOrders') }}</NuxtLink>
      <NuxtLink to="/carrier/tenders" class="procurement-nav__link">{{ t('nav.carrierTenders') }}</NuxtLink>
      <NuxtLink to="/carrier/transport-orders" class="procurement-nav__link">{{ t('nav.carrierOrders') }}</NuxtLink>
      <NuxtLink v-if="showContractsNav" to="/contracts" class="procurement-nav__link">{{ t('nav.contracts') }}</NuxtLink>
      <NuxtLink to="/settlements" class="procurement-nav__link">{{ t('nav.settlements') }}</NuxtLink>
      <NuxtLink to="/billing-registers" class="procurement-nav__link">{{ t('nav.billingRegisters') }}</NuxtLink>
      <NuxtLink to="/payments" class="procurement-nav__link">{{ t('nav.payments') }}</NuxtLink>
    </nav>

    <div class="procurement-content">
      <slot />
    </div>

    <div class="toast-stack">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="toast"
        :class="`toast--${toast.type}`"
      >
        {{ toast.message }}
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.shell-action {
  font: inherit;
  font-size: 0.875rem;
  padding: 0.375rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  cursor: pointer;
}

.shell-action--link {
  display: inline-flex;
  align-items: center;
  color: var(--color-primary);
  text-decoration: none;
}

.procurement-nav {
  display: flex;
  gap: 0.25rem;
  margin: -0.5rem 0 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--color-border);
}

.procurement-nav__link {
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-md);
  color: var(--color-text-muted);
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
}

.procurement-nav__link.router-link-active {
  background: #dbeafe;
  color: #1d4ed8;
}

.procurement-content {
  min-height: 12rem;
}

.toast-stack {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  z-index: 1100;
}

.toast {
  min-width: 240px;
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-md);
  color: #fff;
}

.toast--success {
  background: var(--color-success);
}

.toast--error {
  background: var(--color-danger);
}

.toast--info {
  background: var(--color-info);
}
</style>
