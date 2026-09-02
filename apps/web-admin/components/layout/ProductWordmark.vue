<script setup lang="ts">
import { PRODUCT_WORDMARK } from '~/utils/productBranding'

defineProps<{
  compact?: boolean
}>()
</script>

<template>
  <span
    class="product-wordmark"
    :class="{ 'product-wordmark--compact': compact }"
    :aria-label="PRODUCT_WORDMARK"
  >
    {{ PRODUCT_WORDMARK }}
  </span>
</template>

<style scoped>
.product-wordmark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  line-height: 1;
  white-space: nowrap;
}

.product-wordmark--compact {
  font-size: 0.45rem;
  letter-spacing: 0.02em;
}
</style>
