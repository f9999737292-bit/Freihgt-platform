<script setup lang="ts">
import type { StudioNavStep } from '~/components/rfx/studio/studioNav'

defineProps<{
  steps: StudioNavStep[]
}>()
</script>

<template>
  <div class="studio-shell">
    <aside class="studio-shell__nav" :aria-label="$t('rfx.studio.navLabel')">
      <nav class="studio-nav">
        <template v-for="step in steps" :key="step.id">
          <NuxtLink
            v-if="!step.planned && step.to"
            :to="step.to"
            class="studio-nav__item"
            :class="{ 'studio-nav__item--active': step.active }"
          >
            <span>{{ step.label }}</span>
          </NuxtLink>
          <span
            v-else
            class="studio-nav__item studio-nav__item--planned"
            :aria-disabled="true"
          >
            <span>{{ step.label }}</span>
            <UiBadge status="PLANNED" tone="neutral">{{ $t('rfx.studio.planned') }}</UiBadge>
          </span>
        </template>
      </nav>
    </aside>
    <main class="studio-shell__content">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.studio-shell {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 1.5rem;
  align-items: start;
}

.studio-shell__nav {
  position: sticky;
  top: 1rem;
}

.studio-nav {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.75rem;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
}

.studio-nav__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.625rem 0.75rem;
  border-radius: var(--radius-md);
  color: var(--color-text);
  text-decoration: none;
  font-size: 0.9375rem;
}

.studio-nav__item:hover:not(.studio-nav__item--planned) {
  background: var(--color-bg);
}

.studio-nav__item--active {
  background: rgba(37, 99, 235, 0.08);
  color: var(--color-primary);
  font-weight: 600;
}

.studio-nav__item--planned {
  opacity: 0.55;
  cursor: not-allowed;
}

.studio-shell__content {
  min-width: 0;
}

@media (max-width: 960px) {
  .studio-shell {
    grid-template-columns: 1fr;
  }

  .studio-shell__nav {
    position: static;
  }

  .studio-nav {
    flex-direction: row;
    flex-wrap: wrap;
  }
}
</style>
