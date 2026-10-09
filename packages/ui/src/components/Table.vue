<script setup lang="ts">
defineProps<{
  columns: string[]
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
        <slot />
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.ui-table-wrap {
  overflow-x: auto;
}
.ui-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.ui-table th,
.ui-table td {
  text-align: left;
  padding: 0.6rem 0.5rem;
  border-bottom: 1px solid #e5e7eb;
}
</style>
