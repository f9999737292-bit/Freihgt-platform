<script setup lang="ts">
defineProps<{
  columns: string[]
  loading?: boolean
}>()
</script>

<template>
  <div class="ui-table-wrap">
    <table class="ui-table">
      <thead>
        <tr>
          <th v-for="column in columns" :key="column">{{ column }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td :colspan="columns.length" class="ui-table__empty">{{ $t('common.loading') }}</td>
        </tr>
        <slot v-else />
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.ui-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.ui-table th,
.ui-table td {
  padding: 0.75rem 1rem;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

.ui-table th {
  background: #f8fafc;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-muted);
}

.ui-table tr:last-child td {
  border-bottom: none;
}

.ui-table__empty {
  text-align: center;
  color: var(--color-text-muted);
}
</style>
