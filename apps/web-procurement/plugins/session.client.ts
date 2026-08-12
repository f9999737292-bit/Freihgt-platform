export default defineNuxtPlugin(() => {
  useSession().restoreSession()
})
