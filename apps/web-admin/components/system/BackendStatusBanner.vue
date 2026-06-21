<script setup lang="ts">
const config = useRuntimeConfig()
const { backendOnline, backendStatus, lastCheckedAt, checkBackendStatus } = useBackendStatus()

const checking = ref(false)

const mockAuthActive = computed(() => config.public.mockAuth === true)
const showBanner = computed(
  () =>
    import.meta.dev ||
    mockAuthActive.value ||
    backendStatus.value === 'offline' ||
    backendStatus.value === 'checking',
)

const apiBaseUrl = computed(() => config.public.apiBaseUrl.replace(/\/$/, ''))

async function onRefresh() {
  checking.value = true
  try {
    await checkBackendStatus()
  } finally {
    checking.value = false
  }
}

onMounted(() => {
  checkBackendStatus()
})
</script>

<template>
  <div v-if="showBanner" class="backend-status-banner" :class="backendOnline ? 'backend-status-banner--online' : 'backend-status-banner--offline'">
    <div class="backend-status-banner__content">
      <div class="backend-status-banner__badges">
        <span
          class="backend-status-banner__badge"
          :class="backendOnline ? 'backend-status-banner__badge--online' : 'backend-status-banner__badge--offline'"
        >
          {{
            backendStatus === 'checking'
              ? $t('common.loading')
              : backendOnline
                ? $t('backendStatus.online')
                : $t('backendStatus.offline')
          }}
        </span>
        <span v-if="mockAuthActive" class="backend-status-banner__badge backend-status-banner__badge--mock">
          {{ $t('backendStatus.mockModeActive') }}
        </span>
      </div>

      <div class="backend-status-banner__text">
        <p v-if="backendOnline" class="backend-status-banner__message">
          {{ $t('backendStatus.gatewayAvailable') }}
        </p>
        <template v-else>
          <p class="backend-status-banner__message">
            {{ $t('backendStatus.offlineWarning', { url: apiBaseUrl }) }}
          </p>
          <p class="backend-status-banner__hint">{{ $t('backendStatus.apiUnavailableUntilStarted') }}</p>
          <pre class="backend-status-banner__commands">{{ $t('backendStatus.startBackendCommands') }}</pre>
        </template>
        <p v-if="mockAuthActive" class="backend-status-banner__hint">
          {{ $t('backendStatus.mockModeHint') }}
        </p>
        <p v-if="lastCheckedAt" class="backend-status-banner__meta">
          {{ $t('health.lastChecked') }}: {{ lastCheckedAt.toLocaleString() }}
        </p>
      </div>
    </div>

    <div class="backend-status-banner__actions">
      <UiButton size="sm" variant="secondary" :loading="checking || backendStatus === 'checking'" @click="onRefresh">
        {{ $t('backendStatus.refresh') }}
      </UiButton>
      <NuxtLink to="/health" class="backend-status-banner__link">
        <UiButton size="sm" variant="ghost">{{ $t('backendStatus.openHealthPage') }}</UiButton>
      </NuxtLink>
      <span class="backend-status-banner__docs-hint" :title="$t('backendStatus.troubleshootingDocsPath')">
        {{ $t('backendStatus.troubleshootingDocs') }}
      </span>
    </div>
  </div>
</template>

<style scoped>
.backend-status-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}

.backend-status-banner--online {
  background: #ecfdf5;
  border-color: #a7f3d0;
  color: #065f46;
}

.backend-status-banner--offline {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.backend-status-banner__content {
  flex: 1;
  min-width: 0;
}

.backend-status-banner__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.backend-status-banner__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.125rem 0.625rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.backend-status-banner__badge--online {
  background: #d1fae5;
  color: #065f46;
}

.backend-status-banner__badge--offline {
  background: #fee2e2;
  color: #991b1b;
}

.backend-status-banner__badge--mock {
  background: #dbeafe;
  color: #1e40af;
}

.backend-status-banner__message {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
}

.backend-status-banner__hint {
  margin: 0.35rem 0 0;
  font-size: 0.8125rem;
}

.backend-status-banner__meta {
  margin: 0.35rem 0 0;
  font-size: 0.75rem;
  opacity: 0.85;
}

.backend-status-banner__commands {
  margin: 0.5rem 0 0;
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-sm);
  background: rgba(0, 0, 0, 0.06);
  font-size: 0.75rem;
  line-height: 1.5;
  white-space: pre-wrap;
}

.backend-status-banner__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}

.backend-status-banner__link {
  text-decoration: none;
}

.backend-status-banner__docs-hint {
  font-size: 0.75rem;
  opacity: 0.85;
  padding: 0.25rem 0.5rem;
}

@media (max-width: 768px) {
  .backend-status-banner {
    flex-direction: column;
  }

  .backend-status-banner__actions {
    width: 100%;
  }
}
</style>
