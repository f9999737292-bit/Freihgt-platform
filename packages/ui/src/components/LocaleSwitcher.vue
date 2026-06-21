<script setup lang="ts">
const { locale, locales, setLocale } = useI18n()
</script>

<template>
  <label class="locale-switcher">
    <span>{{ $t('common.language') }}</span>
    <select :value="locale" @change="setLocale(($event.target as HTMLSelectElement).value)">
      <option v-for="item in locales" :key="item.code" :value="item.code">
        {{ item.name }}
      </option>
    </select>
  </label>
</template>

<style scoped>
.locale-switcher {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
}

select {
  padding: 0.25rem 0.5rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  background: #fff;
}
</style>
