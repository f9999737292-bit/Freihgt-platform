export function useToast() {
  const uiStore = useUiStore()

  function pushToast(type: 'success' | 'error' | 'info' | 'warning', message: string) {
    uiStore.pushToast(type, message)
  }

  return {
    toasts: computed(() => uiStore.toasts),
    pushToast,
    removeToast: uiStore.removeToast,
  }
}
