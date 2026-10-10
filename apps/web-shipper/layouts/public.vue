<template>
  <div class="public-layout">
    <slot />
  </div>
</template>

<style scoped>
.public-layout {
  min-height: 100vh;
  display: grid;
  place-items: center;
  font-family: system-ui, sans-serif;
  background: #f7f8fa;
  padding: 1.5rem;
}
</style>
