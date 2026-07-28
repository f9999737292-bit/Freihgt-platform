<script setup lang="ts">
const authStore = useAuthStore()
const { getLandingRoute } = usePermissions()

if (!authStore.restored) {
  authStore.restoreSession()
}

await navigateTo(authStore.isAuthenticated ? getLandingRoute() : '/login')
</script>
