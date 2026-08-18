<script setup lang="ts">
definePageMeta({ middleware: 'auth' })

await navigateTo('/tenders', { replace: true })
</script>
