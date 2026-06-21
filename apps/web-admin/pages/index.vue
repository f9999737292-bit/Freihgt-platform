<script setup lang="ts">
const authStore = useAuthStore()

if (!authStore.restored) {
  authStore.restoreSession()
}

await navigateTo(authStore.isAuthenticated ? '/dashboard' : '/login')
</script>
